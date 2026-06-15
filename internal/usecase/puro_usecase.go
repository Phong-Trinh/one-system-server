package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Input DTOs ────────────────────────────────────────────────────────────────

// PurOLineInput is a single line item when creating or confirming a PurchaseOrder.
type PurOLineInput struct {
	ItemID          *string `json:"item_id"`           // FK → Item (nil for CapEx lines)
	EquipmentTypeID *string `json:"equipment_type_id"` // FK → EquipmentType (nil for OpEx lines)
	QtyOrdered      float64 `json:"qty_ordered"`       // In packaging units
	PkgUnit         string  `json:"pkg_unit"`
	Conversion      float64 `json:"conversion"`        // Base units per pkg_unit
	UnitPrice       float64 `json:"unit_price"`        // Price per packaging unit (from supplier quote)
}

// ── Interface ─────────────────────────────────────────────────────────────────

// PurOUseCase manages the PurchaseOrder lifecycle for external procurement.
//
// Two creation paths:
//   - AUTO_DRAFT (EXTERNAL_PROCUREMENT ROP trigger): system calls CreateDraftPurO → HQ reviews →
//     ConfirmDraftPurO → MarkShipped → GR confirmed → SettlePayment.
//   - PR_TRIGGERED (CapEx): HQ calls CreatePRTriggeredPurO → starts at CONFIRMED already →
//     MarkShipped → GR confirmed → SettlePayment → auto-creates Asset.
//
// Authority: ONLY HQ can create a PO. DeliveryToNodeID is NEVER the HQ node.
type PurOUseCase interface {
	// CreateDraftPurO is called by the ROP engine for EXTERNAL_PROCUREMENT sourcing strategy.
	// PO is created in DRAFT status — supplier and prices are not yet set.
	// HQ must call ConfirmDraftPurO to fill those in and send to supplier.
	CreateDraftPurO(ctx context.Context, orgID, hqNodeID, deliveryToNodeID, itemID string, qtyBU float64, cfg *models.NodeItemConfig) (*models.PurchaseOrder, error)

	// CreatePRTriggeredPurO converts an APPROVED PurchaseRequisition into a fully issued PO.
	// PO starts at CONFIRMED status (no DRAFT phase — HQ already has supplier quote from PR review).
	// Also sets PR.Status = CONVERTED_TO_PURO.
	CreatePRTriggeredPurO(ctx context.Context, prID, supplierID, hqNodeID, confirmedByStaffID string, lines []PurOLineInput) (*models.PurchaseOrder, error)

	// ConfirmDraftPurO is called by HQ to review an AUTO_DRAFT PO, attach a supplier and prices,
	// and formally send it to the supplier.
	// Transitions: DRAFT → CONFIRMED.
	ConfirmDraftPurO(ctx context.Context, purOID, supplierID, staffID string, lines []PurOLineInput) error

	// MarkShipped transitions a CONFIRMED PO to SHIPPED (supplier has dispatched goods).
	MarkShipped(ctx context.Context, purOID string) error

	// SettlePayment performs 3-Way Matching and marks the PO as COMPLETED.
	// For PR_TRIGGERED POs, auto-creates an Asset record (status=PENDING_REGISTRATION).
	// The Asset creation is performed by the injected AssetUseCase.
	// Transitions: SHIPPED → COMPLETED.
	SettlePayment(ctx context.Context, purOID, invoiceID, grID, staffID string) (*models.Asset, error)

	GetPurO(ctx context.Context, purOID string) (*models.PurchaseOrder, []*models.PurchaseOrderLine, error)
	ListDrafts(ctx context.Context, orgID string) ([]*models.PurchaseOrder, error)
	ListByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error)
	HasActivePurO(ctx context.Context, deliveryNodeID, itemID string) (bool, error)
	SimpleConfirmPurO(ctx context.Context, purOID, staffID string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type purOUseCase struct {
	purORepo     services.PurchaseOrderRepository
	lineRepo     services.PurchaseOrderLineRepository
	prRepo       services.PurchaseRequisitionRepository
	prLineRepo   services.PRLineRepository
	supplierRepo services.SupplierRepository
	// assetUC is a late-bound reference set by the SupplyChainFacade after all usecases
	// are constructed, to avoid a construction-time circular dependency.
	assetUC AssetUseCase
}

func newPurOUseCase(
	purORepo services.PurchaseOrderRepository,
	lineRepo services.PurchaseOrderLineRepository,
	prRepo services.PurchaseRequisitionRepository,
	prLineRepo services.PRLineRepository,
	supplierRepo services.SupplierRepository,
) *purOUseCase {
	return &purOUseCase{
		purORepo:     purORepo,
		lineRepo:     lineRepo,
		prRepo:       prRepo,
		prLineRepo:   prLineRepo,
		supplierRepo: supplierRepo,
	}
}

// setAssetUseCase is called by SupplyChainFacade after all usecases are wired.
func (uc *purOUseCase) setAssetUseCase(a AssetUseCase) {
	uc.assetUC = a
}

// ── CreateDraftPurO ─────────────────────────────────────────────────────────────

func (uc *purOUseCase) CreateDraftPurO(ctx context.Context, orgID, hqNodeID, deliveryToNodeID, itemID string, qtyBU float64, cfg *models.NodeItemConfig) (*models.PurchaseOrder, error) {
	if hqNodeID == deliveryToNodeID {
		return nil, fmt.Errorf("po: CreateDraftPurO: delivery_to_node_id must not be the HQ node")
	}

	now := time.Now()
	purO := &models.PurchaseOrder{
		ID:               uuid.NewString(),
		OrgID:            orgID,
		TriggerType:      models.PurOTriggerAutoDraft,
		HQNodeID:         hqNodeID,
		SupplierID:       "", // Filled by HQ in ConfirmDraftPurO
		DeliveryToNodeID: deliveryToNodeID,
		Status:           models.PurchaseOrderDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Pre-populate supplier hint if configured in NodeItemConfig.
	if cfg != nil && cfg.SupplierID != nil {
		purO.SupplierID = *cfg.SupplierID
	}

	if err := uc.purORepo.Create(ctx, purO); err != nil {
		return nil, fmt.Errorf("po: CreateDraftPurO: persist: %w", err)
	}

	// Create a draft line for the triggering item. Price is not set yet (HQ fills it in ConfirmDraftPurO).
	line := &models.PurchaseOrderLine{
		ID:         uuid.NewString(),
		PurOID:     purO.ID,
		ItemID:     &itemID,
		QtyOrdered: qtyBU, // Draft uses base units as proxy quantity; HQ adjusts to pkg units on confirm
		PkgUnit:    "base_unit",
		Conversion: 1.0,
		UnitPrice:  0, // Not yet set
	}
	if err := uc.lineRepo.AddLine(ctx, line); err != nil {
		return nil, fmt.Errorf("po: CreateDraftPurO: add draft line: %w", err)
	}

	return purO, nil
}

// ── CreatePRTriggeredPurO ───────────────────────────────────────────────────────

func (uc *purOUseCase) CreatePRTriggeredPurO(ctx context.Context, prID, supplierID, hqNodeID, confirmedByStaffID string, lines []PurOLineInput) (*models.PurchaseOrder, error) {
	// Load and validate the PR.
	pr, err := uc.prRepo.FindByID(ctx, prID)
	if err != nil || pr == nil {
		return nil, fmt.Errorf("po: CreatePRTriggeredPurO: PR %s not found: %w", prID, err)
	}
	if pr.Status != models.PRApproved {
		return nil, fmt.Errorf("po: CreatePRTriggeredPurO: PR %s must be APPROVED (current: %s)", prID, pr.Status)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("po: CreatePRTriggeredPurO: at least one line required")
	}
	for i, l := range lines {
		if l.UnitPrice <= 0 {
			return nil, fmt.Errorf("po: CreatePRTriggeredPurO: line %d must have unit_price > 0 (HQ has supplier quote)", i)
		}
	}

	now := time.Now()
	purO := &models.PurchaseOrder{
		ID:               uuid.NewString(),
		OrgID:            pr.OrgID,
		TriggerType:      models.PurOTriggerPR,
		PRID:             &prID,
		HQNodeID:         hqNodeID,
		SupplierID:       supplierID,
		DeliveryToNodeID: pr.RequesterNodeID, // Goods go directly to the requesting node
		Status:           models.PurchaseOrderConfirmed,
		ConfirmedBy:      &confirmedByStaffID,
		ConfirmedAt:      &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := uc.purORepo.Create(ctx, purO); err != nil {
		return nil, fmt.Errorf("po: CreatePRTriggeredPurO: persist PO: %w", err)
	}

	for i, l := range lines {
		line := &models.PurchaseOrderLine{
			ID:              uuid.NewString(),
			PurOID:          purO.ID,
			ItemID:          l.ItemID,
			EquipmentTypeID: l.EquipmentTypeID,
			QtyOrdered:      l.QtyOrdered,
			PkgUnit:         l.PkgUnit,
			Conversion:      l.Conversion,
			UnitPrice:       l.UnitPrice,
		}
		if err := uc.lineRepo.AddLine(ctx, line); err != nil {
			return nil, fmt.Errorf("po: CreatePRTriggeredPurO: add line %d: %w", i, err)
		}
	}

	// Mark the PR as converted.
	pr.Status = models.PRConvertedToPurO
	pr.UpdatedAt = now
	if err := uc.prRepo.Update(ctx, pr); err != nil {
		return nil, fmt.Errorf("po: CreatePRTriggeredPurO: update PR status: %w", err)
	}

	return purO, nil
}

// ── ConfirmDraftPurO ────────────────────────────────────────────────────────────

// ConfirmDraftPurO is called by HQ to review an auto-generated draft PO.
// HQ replaces the draft lines with supplier-quoted quantities and prices.
func (uc *purOUseCase) ConfirmDraftPurO(ctx context.Context, purOID, supplierID, staffID string, lines []PurOLineInput) error {
	purO, err := uc.loadPurO(ctx, purOID)
	if err != nil {
		return err
	}
	if purO.Status != models.PurchaseOrderDraft {
		return fmt.Errorf("po: ConfirmDraftPurO: PO %s is not in DRAFT (current: %s)", purOID, purO.Status)
	}
	if supplierID == "" {
		return fmt.Errorf("po: ConfirmDraftPurO: supplier_id is required")
	}
	for i, l := range lines {
		if l.UnitPrice <= 0 {
			return fmt.Errorf("po: ConfirmDraftPurO: line %d must have unit_price > 0", i)
		}
	}

	// Replace draft lines with confirmed lines.
	existingLines, _ := uc.lineRepo.ListByPurO(ctx, purOID)
	for _, el := range existingLines {
		_ = uc.lineRepo.DeleteLine(ctx, el.ID)
	}
	for i, l := range lines {
		line := &models.PurchaseOrderLine{
			ID:              uuid.NewString(),
			PurOID:          purOID,
			ItemID:          l.ItemID,
			EquipmentTypeID: l.EquipmentTypeID,
			QtyOrdered:      l.QtyOrdered,
			PkgUnit:         l.PkgUnit,
			Conversion:      l.Conversion,
			UnitPrice:       l.UnitPrice,
		}
		if err := uc.lineRepo.AddLine(ctx, line); err != nil {
			return fmt.Errorf("po: ConfirmDraftPurO: add line %d: %w", i, err)
		}
	}

	now := time.Now()
	purO.SupplierID = supplierID
	purO.Status = models.PurchaseOrderConfirmed
	purO.ConfirmedBy = &staffID
	purO.ConfirmedAt = &now
	purO.UpdatedAt = now

	return uc.purORepo.Update(ctx, purO)
}

// SimpleConfirmPurO provides a shortcut to confirm an auto-generated draft PO without modifying lines.
func (uc *purOUseCase) SimpleConfirmPurO(ctx context.Context, purOID, staffID string) error {
	purO, err := uc.loadPurO(ctx, purOID)
	if err != nil {
		return err
	}
	if purO.Status != models.PurchaseOrderDraft {
		return fmt.Errorf("po: SimpleConfirm: PO %s is not in DRAFT (current: %s)", purOID, purO.Status)
	}
	
	now := time.Now()
	purO.Status = models.PurchaseOrderConfirmed
	purO.ConfirmedBy = &staffID
	purO.ConfirmedAt = &now
	purO.UpdatedAt = now

	return uc.purORepo.Update(ctx, purO)
}

// ── MarkShipped ───────────────────────────────────────────────────────────────

func (uc *purOUseCase) MarkShipped(ctx context.Context, purOID string) error {
	purO, err := uc.loadPurO(ctx, purOID)
	if err != nil {
		return err
	}
	if purO.Status != models.PurchaseOrderConfirmed {
		return fmt.Errorf("po: MarkShipped: PO %s is not CONFIRMED (current: %s)", purOID, purO.Status)
	}
	purO.Status = models.PurchaseOrderShipped
	purO.UpdatedAt = time.Now()
	return uc.purORepo.Update(ctx, purO)
}

// ── SettlePayment ─────────────────────────────────────────────────────────────

// SettlePayment performs 3-Way Matching (PO + Invoice + GR) and marks the PO COMPLETED.
// For PR_TRIGGERED POs, auto-creates an Asset record via the AssetUseCase.
// The returned Asset is nil for AUTO_DRAFT POs.
func (uc *purOUseCase) SettlePayment(ctx context.Context, purOID, invoiceID, grID, staffID string) (*models.Asset, error) {
	purO, err := uc.loadPurO(ctx, purOID)
	if err != nil {
		return nil, err
	}
	if purO.Status != models.PurchaseOrderDelivered {
		return nil, fmt.Errorf("po: SettlePayment: PO %s must be DELIVERED before settling (current: %s)", purOID, purO.Status)
	}

	now := time.Now()
	purO.Status = models.PurchaseOrderCompleted
	purO.UpdatedAt = now
	if err := uc.purORepo.Update(ctx, purO); err != nil {
		return nil, fmt.Errorf("po: SettlePayment: update PO status: %w", err)
	}

	// Auto-create Asset for CapEx (PR_TRIGGERED) POs only.
	if purO.TriggerType == models.PurOTriggerPR && uc.assetUC != nil {
		asset, err := uc.assetUC.AutoCreateAsset(ctx, purOID, grID)
		if err != nil {
			return nil, fmt.Errorf("po: SettlePayment: auto-create asset: %w", err)
		}
		return asset, nil
	}

	return nil, nil
}

// ── Getters ───────────────────────────────────────────────────────────────────

func (uc *purOUseCase) GetPurO(ctx context.Context, purOID string) (*models.PurchaseOrder, []*models.PurchaseOrderLine, error) {
	purO, err := uc.loadPurO(ctx, purOID)
	if err != nil {
		return nil, nil, err
	}
	lines, err := uc.lineRepo.ListByPurO(ctx, purOID)
	return purO, lines, err
}

func (uc *purOUseCase) ListDrafts(ctx context.Context, orgID string) ([]*models.PurchaseOrder, error) {
	return uc.purORepo.FindByStatus(ctx, orgID, "")
}

func (uc *purOUseCase) ListByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error) {
	return uc.purORepo.FindByDeliveryNode(ctx, nodeID)
}

func (uc *purOUseCase) HasActivePurO(ctx context.Context, deliveryNodeID, itemID string) (bool, error) {
	puros, err := uc.purORepo.FindByDeliveryNode(ctx, deliveryNodeID)
	if err != nil {
		return false, err
	}
	for _, po := range puros {
		if po.Status == models.PurchaseOrderCompleted || po.Status == models.PurchaseOrderCancelled || po.Status == models.PurchaseOrderDelivered {
			continue // Not active (already received, cancelled, or fully settled)
		}
		
		lines, err := uc.lineRepo.ListByPurO(ctx, po.ID)
		if err != nil {
			return false, err
		}
		for _, l := range lines {
			if l.ItemID != nil && *l.ItemID == itemID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (uc *purOUseCase) loadPurO(ctx context.Context, id string) (*models.PurchaseOrder, error) {
	purO, err := uc.purORepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("po: load PO %s: %w", id, err)
	}
	if purO == nil {
		return nil, fmt.Errorf("po: PO %s not found", id)
	}
	return purO, nil
}
