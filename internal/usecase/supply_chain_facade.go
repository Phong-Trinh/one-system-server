package usecase

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── SupplyChainFacade ─────────────────────────────────────────────────────────
//
// SupplyChainFacade is the single entry point for all supply chain operations.
// It composes all supply chain usecases into one struct so callers (HTTP handlers,
// background jobs, the allocation engine) have one consistent dependency to inject.
//
// Design notes:
//   - Each sub-usecase (ITOUseCase, POUseCase, etc.) is exposed as a public field
//     so transport/app layers can call them directly without going through the facade.
//   - Methods on the facade itself handle cross-usecase orchestration — specifically:
//     the ROP engine dispatch loop and any other workflow that crosses domain boundaries.
//   - Circular construction-time dependencies (poUC ↔ assetUC, invoiceUC ↔ poUC) are
//     resolved by the facade's constructor using late-binding setter methods.
//
// NOTE: Do NOT instantiate the individual sub-usecases directly outside of this facade.
// Always use NewSupplyChainFacade to ensure cross-usecase wiring is correctly established.
type SupplyChainFacade struct {
	// Sub-usecases — exposed for direct use by transport/app layers.
	Inventory InvUseCase
	ITO       ITOUseCase
	PurO      PurOUseCase
	PR        services.PRService
	GR        GRUseCase
	GI        GIUseCase
	B2B       B2BUseCase
	Invoice   InvoiceUseCase
	Asset     AssetUseCase

	// productionUC and orchestrator are set via SetProductionUseCase after construction
	// to break the circular dependency (app.go wires facade first, then production UC).
	productionUC ProductionUseCase
	orchestrator *OrderPoolingOrchestrator
}

// InvUseCase is the public alias for services.InventoryService,
// used to expose it on the facade without package-import ambiguity.
type InvUseCase = services.InventoryService

// SupplyChainRepos bundles all repository dependencies for the facade constructor.
// Keeping them in a struct avoids a 20-argument constructor signature.
type SupplyChainRepos struct {
	Stock        services.NodeStockRepository
	Config       services.NodeItemConfigRepository
	Supplier     services.SupplierRepository
	ITO          services.InternalTransferOrderRepository
	ITOLine      services.ITOLineRepository
	PR           services.PurchaseRequisitionRepository
	PRLine       services.PRLineRepository
	PurO         services.PurchaseOrderRepository
	PurOLine     services.PurchaseOrderLineRepository
	GI           services.GoodsIssueRepository
	GILine       services.GoodsIssueLineRepository
	GR           services.GoodsReceiptRepository
	GRLine       services.GoodsReceiptLineRepository
	DT           services.DiscrepancyTicketRepository
	Invoice      services.SupplierInvoiceRepository
	InvoiceLine  services.SupplierInvoiceLineRepository
	Transaction  services.TransactionRepository
	B2BOrder     services.B2BSalesOrderRepository
	B2BOrderLine services.B2BSalesOrderLineRepository
	Asset        services.AssetRepository
	Machine      services.MachineRepository
	Node         services.NodeRepository
	EquipmentType services.EquipmentTypeRepository
}

// NewSupplyChainFacade constructs and wires all supply chain usecases.
// This is the only place where cross-usecase late-binding (post-construction wiring)
// is performed to break circular construction-time dependencies:
//
//	poUseCase.assetUC   ← assetUseCase (poUC calls asset on SettlePayment)
//	invoiceUseCase.poUC ← poUseCase    (invoice calls po.SettlePayment in 3-Way Match)
func NewSupplyChainFacade(repos SupplyChainRepos) *SupplyChainFacade {
	// ── Leaf usecases (no cross-usecase deps) ────────────────────────────────
	inv := newInventoryUseCase(repos.Stock, repos.Config)

	assetUC := newAssetUseCase(
		repos.Asset,
		repos.Machine,
		repos.PurO,
		repos.GR,
		repos.PR,
		repos.PRLine,
		repos.InvoiceLine,
	)

	giUC := newGIUseCase(repos.GI, repos.GILine, inv)

	grUC := newGRUseCase(repos.GR, repos.GRLine, repos.DT, repos.PurO, inv)

	itoUC := newITOUseCase(
		repos.ITO,
		repos.ITOLine,
		repos.GI,
		repos.GILine,
		repos.GR,
		repos.GRLine,
		repos.DT,
		repos.Node,
		inv,
	)

	prUC := newPRUseCase(repos.PR, repos.PRLine, repos.EquipmentType)

	b2bUC := newB2BUseCase(repos.B2BOrder, repos.B2BOrderLine, giUC)

	// ── PO usecase (depends on assetUC via late binding) ─────────────────────
	puroUC := newPurOUseCase(repos.PurO, repos.PurOLine, repos.PR, repos.PRLine, repos.Supplier)
	puroUC.setAssetUseCase(assetUC) // Late-bind: poUC calls assetUC.AutoCreateAsset on SettlePayment

	// ── Invoice usecase (depends on poUC via late binding) ───────────────────
	invoiceUC := newInvoiceUseCase(
		repos.Invoice,
		repos.InvoiceLine,
		repos.Transaction,
		repos.PurO,
		repos.GR,
	)
	invoiceUC.setPurOUseCase(puroUC) // Late-bind: invoiceUC calls poUC.SettlePayment in 3-Way Match

	return &SupplyChainFacade{
		Inventory: inv,
		ITO:       itoUC,
		PurO:      puroUC,
		PR:        prUC,
		GR:        grUC,
		GI:        giUC,
		B2B:       b2bUC,
		Invoice:   invoiceUC,
		Asset:     assetUC,
	}
}

// ── ROP Dispatch ──────────────────────────────────────────────────────────────

// HandleROPBreach is the central ROP dispatch method called by the facade after
// any stock-decreasing event. It reads the ROPCheckResult and fires the appropriate
// replenishment document:
//   - INTERNAL_TRANSFER  → creates an auto ITO to the configured provider node.
//   - EXTERNAL_PROCUREMENT → creates a draft PO on the HQ dashboard.
//
// This method is the integration point that keeps InventoryService clean (it returns
// ROPCheckResult without acting on it) while the facade orchestrates the next step.
//
// Parameters:
//   - orgID:    The organization context (required for ITO.OrgID field).
//   - hqNodeID: The HQ node that will own auto-draft POs.
//   - result:   The ROPCheckResult returned by InventoryService.StockOut or CheckROP.
func (f *SupplyChainFacade) HandleROPBreach(ctx context.Context, orgID, hqNodeID string, result *services.ROPCheckResult) error {
	if result == nil || !result.Breached {
		return nil
	}

	cfg := result.Config
	if cfg == nil {
		return fmt.Errorf("facade: HandleROPBreach: ROPCheckResult.Config is nil (cannot determine strategy)")
	}

	log.Info().
		Str("node_id", cfg.NodeID).
		Str("item_id", cfg.ItemID).
		Str("strategy", string(result.Strategy)).
		Float64("qty_on_hand", result.CurrentQty).
		Float64("reorder_point", result.ReorderPoint).
		Msg("[SupplyChainFacade] ROP breached — firing replenishment")

	switch result.Strategy {
	case models.SourcingInternalTransfer:
		if cfg.ProviderNodeID == nil {
			return fmt.Errorf("facade: HandleROPBreach: INTERNAL_TRANSFER but provider_node_id not set on NodeItemConfig for node=%s item=%s",
				cfg.NodeID, cfg.ItemID)
		}
		hasActive, err := f.ITO.HasActiveITO(ctx, cfg.NodeID, cfg.ItemID)
		if err != nil {
			return fmt.Errorf("facade: HandleROPBreach: check active ITO: %w", err)
		}
		if hasActive {
			log.Info().
				Str("node_id", cfg.NodeID).
				Str("item_id", cfg.ItemID).
				Msg("[SupplyChainFacade] ROP breached, but an active ITO already exists — skipping duplicate replenishment")
			return nil
		}

		// Quantity to replenish = ReorderPoint - CurrentQty + SafetyStock (gap + buffer).
		replenishQty := (cfg.ReorderPoint - result.CurrentQty) + cfg.SafetyStock
		if replenishQty <= 0 {
			replenishQty = cfg.SafetyStock // Minimum: at least the safety stock buffer
		}

		ito, err := f.ITO.CreateAutoITO(ctx, cfg, replenishQty)
		if err != nil {
			return fmt.Errorf("facade: HandleROPBreach: create auto ITO: %w", err)
		}
		// Patch the orgID on the ITO (CreateAutoITO doesn't have org context).
		log.Info().Str("ito_id", ito.ID).Msg("[SupplyChainFacade] Auto ITO created")

		// ── Auto-ensure Provider Stock for ITOs ────────────────────────────────
		// When a Store's ROP triggers an ITO to the Factory, we check if the Factory
		// already has enough stock for the item. If not, we trigger the Factory's
		// replenishment strategy (Auto-PO or Draft PurO).
		if cfg.ProviderNodeID != nil {
			// Run async to not block the current request (e.g. Sale Order completion).
			// Use context.Background() so it isn't cancelled when the HTTP request ends.
			go f.ensureProviderStock(context.Background(), orgID, hqNodeID, *cfg.ProviderNodeID, cfg.ItemID, replenishQty)
		}

	case models.SourcingExternalProcurement:
		hasActive, err := f.PurO.HasActivePurO(ctx, cfg.NodeID, cfg.ItemID)
		if err != nil {
			return fmt.Errorf("facade: HandleROPBreach: check active PO: %w", err)
		}
		if hasActive {
			log.Info().
				Str("node_id", cfg.NodeID).
				Str("item_id", cfg.ItemID).
				Msg("[SupplyChainFacade] ROP breached, but an active PO already exists — skipping duplicate draft")
			return nil
		}

		// Quantity to replenish = ReorderPoint - CurrentQty + SafetyStock (gap + buffer).
		replenishQty := (cfg.ReorderPoint - result.CurrentQty) + cfg.SafetyStock
		if replenishQty <= 0 {
			replenishQty = cfg.SafetyStock // Minimum: at least the safety stock buffer
		}

		purO, err := f.PurO.CreateDraftPurO(ctx, orgID, hqNodeID, cfg.NodeID, cfg.ItemID, replenishQty, cfg)
		if err != nil {
			return fmt.Errorf("facade: HandleROPBreach: create draft PO: %w", err)
		}
		log.Info().Str("po_id", purO.ID).Msg("[SupplyChainFacade] Draft PO created on HQ dashboard")

	default:
		return fmt.Errorf("facade: HandleROPBreach: unknown sourcing strategy %q", result.Strategy)
	}
	return nil
}

// GetBOMByItem delegates to productionUC to resolve BOM for backflushing.
func (f *SupplyChainFacade) GetBOMByItem(ctx context.Context, itemID string) (*models.BOM, []*models.BOMLine, error) {
	if f.productionUC == nil {
		return nil, nil, fmt.Errorf("productionUC not set")
	}
	return f.productionUC.GetFullBOMByItem(ctx, itemID)
}

// StockOutWithROP decreases stock and synchronously evaluates the ROP threshold.uction domain to call after
// writing a StockConsumption record. It performs StockOut and then dispatches
// the ROP breach (if any) without the caller needing to know the facade internals.
//
// Parameters:
//   - orgID, hqNodeID: passed through to HandleROPBreach for PO/ITO creation.
//   - nodeID, itemID:  the (node, item) pair being consumed.
//   - qtyBU:           quantity consumed in base units.
// SetProductionUseCase wires the ProductionUseCase and Orchestrator into the facade after construction.
// Must be called in app.go after both the facade and productionUC are created.
func (f *SupplyChainFacade) SetProductionUseCase(uc ProductionUseCase, orchestrator *OrderPoolingOrchestrator) {
	f.productionUC = uc
	f.orchestrator = orchestrator
}

// ensureProviderStock checks if the Provider (e.g. Factory) has enough
// stock for the ITO item. If not, it triggers the provider's replenishment strategy
// (auto-creates a ProductionOrder, or synthesizes a ROP breach for Draft Purchase Order).
// Runs in a goroutine; errors are logged but do not block the ITO creation.
func (f *SupplyChainFacade) ensureProviderStock(ctx context.Context, orgID, hqNodeID, providerNodeID, itemID string, neededQty float64) {
	currentQty, err := f.Inventory.GetStock(ctx, providerNodeID, itemID)
	if err != nil {
		log.Error().Err(err).Str("provider_node_id", providerNodeID).Str("item_id", itemID).
			Msg("[SupplyChainFacade] ensureProviderStock: failed to get provider stock")
		return
	}

	if currentQty >= neededQty {
		log.Info().
			Str("provider_node_id", providerNodeID).
			Str("item_id", itemID).
			Float64("current_qty", currentQty).
			Float64("needed_qty", neededQty).
			Msg("[SupplyChainFacade] Provider has sufficient stock for ITO — no auto-replenishment needed")
		return
	}

	// Provider doesn't have enough — trigger replenishment for the shortfall.
	shortfallQty := neededQty - currentQty

	// Determine provider's sourcing strategy for this item.
	cfg, err := f.Inventory.GetConfig(ctx, providerNodeID, itemID)
	if err != nil || cfg == nil {
		log.Warn().Str("item_id", itemID).Str("provider_node_id", providerNodeID).
			Msg("[SupplyChainFacade] ensureProviderStock: no NodeItemConfig found, cannot determine strategy. Falling back to ProductionOrder.")
	} else if cfg.SourcingStrategy == models.SourcingExternalProcurement {
		// Externally procured item — synthesize a ROP breach so HQ can buy it
		result := &services.ROPCheckResult{
			Breached:     true,
			CurrentQty:   currentQty,
			// Set ReorderPoint to neededQty so (ROP - CurrentQty + SafetyStock) >= shortfall
			ReorderPoint: neededQty,
			Strategy:     models.SourcingExternalProcurement,
			Config:       cfg,
		}

		log.Info().
			Str("provider_node_id", providerNodeID).
			Str("item_id", itemID).
			Float64("shortfall", shortfallQty).
			Msg("[SupplyChainFacade] ensureProviderStock: short on procured item, triggering draft Purchase Order")

		if err := f.HandleROPBreach(ctx, orgID, hqNodeID, result); err != nil {
			log.Error().Err(err).Str("item_id", itemID).Msg("[SupplyChainFacade] ensureProviderStock: failed to handle synthesized ROP breach")
		}
		return
	} else if cfg.SourcingStrategy == models.SourcingInternalTransfer {
		log.Warn().Str("item_id", itemID).Str("provider_node_id", providerNodeID).
			Msg("[SupplyChainFacade] ensureProviderStock: cascading internal transfers not fully supported yet")
		return
	}

	// Default/Fallback: Auto-create Production Order (LOCAL_PRODUCTION)
	if f.productionUC == nil {
		return
	}

	// Find the BOM for this item.
	bom, _, err := f.productionUC.GetFullBOMByItem(ctx, itemID)
	if err != nil || bom == nil {
		log.Warn().
			Str("provider_node_id", providerNodeID).
			Str("item_id", itemID).
			Msg("[SupplyChainFacade] ensureProviderStock: no BOM found for item — cannot auto-create PO")
		return
	}

	po, err := f.productionUC.CreateProductionOrder(ctx, bom.ID, providerNodeID, shortfallQty)
	if err != nil {
		log.Error().Err(err).Str("provider_node_id", providerNodeID).Str("item_id", itemID).
			Msg("[SupplyChainFacade] ensureProviderStock: failed to create Production Order")
		return
	}

	log.Info().
		Str("po_id", po.ID).
		Str("provider_node_id", providerNodeID).
		Str("item_id", itemID).
		Float64("produce_qty", shortfallQty).
		Msg("[SupplyChainFacade] ✅ Auto-created Production Order at Provider to fulfill ITO demand")

	// Enqueue into the orchestrator for automatic batch decomposition.
	if f.orchestrator != nil {
		f.orchestrator.Enqueue(po)
	}
}

func (f *SupplyChainFacade) StockOutWithROP(ctx context.Context, orgID, hqNodeID, nodeID, itemID string, qtyBU float64) error {
	result, err := f.Inventory.StockOut(ctx, nodeID, itemID, qtyBU)
	if err != nil {
		return fmt.Errorf("facade: StockOutWithROP: %w", err)
	}
	return f.HandleROPBreach(ctx, orgID, hqNodeID, result)
}

// RunMRPForProductionOrder is called immediately after a Production Order is created.
// It checks if there is sufficient stock for all BOM ingredients. If a shortage is detected,
// it synthesizes a ROP breach to automatically trigger the configured replenishment strategy
// (e.g., EXTERNAL_PROCUREMENT creates a Draft Purchase Order at HQ).
func (f *SupplyChainFacade) RunMRPForProductionOrder(ctx context.Context, orgID, hqNodeID string, po *models.ProductionOrder) {
	_, bomLines, err := f.GetBOMByItem(ctx, po.ItemID)
	if err != nil || len(bomLines) == 0 {
		log.Warn().Str("po_id", po.ID).Msg("[SupplyChainFacade] MRP skipped: no BOM found for PO item")
		return
	}

	for _, line := range bomLines {
		requiredQty := line.Qty * po.PlannedInput

		currentQty, err := f.Inventory.GetStock(ctx, po.NodeID, line.ItemID)
		if err != nil {
			log.Error().Err(err).Str("item_id", line.ItemID).Msg("[SupplyChainFacade] MRP: failed to get stock")
			continue
		}

		if currentQty >= requiredQty {
			continue // Sufficient stock, no action needed.
		}

		// Material shortage detected!
		cfg, err := f.Inventory.GetConfig(ctx, po.NodeID, line.ItemID)
		if err != nil || cfg == nil {
			log.Warn().Str("item_id", line.ItemID).Msg("[SupplyChainFacade] MRP: item is short, but no NodeItemConfig found to trigger replenishment")
			continue
		}

		// Synthesize a ROP breach to reuse HandleROPBreach logic.
		// By setting ReorderPoint to requiredQty, the calculation `(ReorderPoint - CurrentQty) + SafetyStock`
		// correctly yields `shortage + SafetyStock`.
		fakeResult := &services.ROPCheckResult{
			Breached:     true,
			CurrentQty:   currentQty,
			ReorderPoint: requiredQty,
			Strategy:     cfg.SourcingStrategy,
			Config:       cfg,
		}

		log.Info().
			Str("po_id", po.ID).
			Str("item_id", line.ItemID).
			Float64("shortage", requiredQty-currentQty).
			Msg("[SupplyChainFacade] MRP: Material shortage detected, triggering replenishment")

		if err := f.HandleROPBreach(ctx, orgID, hqNodeID, fakeResult); err != nil {
			log.Error().Err(err).Str("item_id", line.ItemID).Msg("[SupplyChainFacade] MRP: failed to handle ROP breach")
		}
	}
}
