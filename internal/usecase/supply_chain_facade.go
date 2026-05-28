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

		replenishQty := (cfg.ReorderPoint - result.CurrentQty) + cfg.SafetyStock
		if replenishQty <= 0 {
			replenishQty = cfg.SafetyStock
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

// StockOutWithROP is a convenience method for the production domain to call after
// writing a StockConsumption record. It performs StockOut and then dispatches
// the ROP breach (if any) without the caller needing to know the facade internals.
//
// Parameters:
//   - orgID, hqNodeID: passed through to HandleROPBreach for PO/ITO creation.
//   - nodeID, itemID:  the (node, item) pair being consumed.
//   - qtyBU:           quantity consumed in base units.
func (f *SupplyChainFacade) StockOutWithROP(ctx context.Context, orgID, hqNodeID, nodeID, itemID string, qtyBU float64) error {
	result, err := f.Inventory.StockOut(ctx, nodeID, itemID, qtyBU)
	if err != nil {
		return fmt.Errorf("facade: StockOutWithROP: %w", err)
	}
	return f.HandleROPBreach(ctx, orgID, hqNodeID, result)
}
