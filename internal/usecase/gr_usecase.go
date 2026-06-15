package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// GRUseCase manages the GoodsReceipt lifecycle for external procurement (PurchaseOrder).
//
// GRs from ITOs are handled inside ITOUseCase.ConfirmGoodsReceipt.
// This usecase handles GRs linked to PurchaseOrders (supplier delivers to node).
//
// On confirmation:
//   - StockIn is triggered for each line's QtyReceived (base units).
//   - If any line has QtyReceived < QtyExpected → DiscrepancyTicket is auto-created.
//   - GR status is set to DISCREPANCY if any shortage, else CONFIRMED.
type GRUseCase interface {
	// ConfirmPurOGoodsReceipt records receipt of goods from a supplier-delivered PurchaseOrder.
	ConfirmPurOGoodsReceipt(ctx context.Context, purOID, receivingNodeID, staffID string, lines []GRLineInput) (*models.GoodsReceipt, error)

	GetGR(ctx context.Context, grID string) (*models.GoodsReceipt, []*models.GoodsReceiptLine, error)
}

// grUseCase implements GRUseCase for PurchaseOrder-linked receipts.
type grUseCase struct {
	grRepo   services.GoodsReceiptRepository
	grLine   services.GoodsReceiptLineRepository
	dtRepo   services.DiscrepancyTicketRepository
	purORepo services.PurchaseOrderRepository
	inv      services.InventoryService
}

func newGRUseCase(
	grRepo services.GoodsReceiptRepository,
	grLine services.GoodsReceiptLineRepository,
	dtRepo services.DiscrepancyTicketRepository,
	purORepo services.PurchaseOrderRepository,
	inv services.InventoryService,
) GRUseCase {
	return &grUseCase{
		grRepo:   grRepo,
		grLine:   grLine,
		dtRepo:   dtRepo,
		purORepo: purORepo,
		inv:      inv,
	}
}

// ConfirmPurOGoodsReceipt records receipt of goods delivered by a supplier.
// Called after the node receives goods from the supplier (linked to a ON_WAY_DELIVERY PurchaseOrder).
// Transitions the PO from ON_WAY_DELIVERY → remains ON_WAY_DELIVERY here; SettlePayment moves it to COMPLETED.
func (uc *grUseCase) ConfirmPurOGoodsReceipt(ctx context.Context, purOID, receivingNodeID, staffID string, lines []GRLineInput) (*models.GoodsReceipt, error) {
	purO, err := uc.purORepo.FindByID(ctx, purOID)
	if err != nil || purO == nil {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: PO %s not found: %w", purOID, err)
	}
	if purO.Status != models.PurchaseOrderOnWayDelivery {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: PO %s must be ON_WAY_DELIVERY to receive (current: %s)", purOID, purO.Status)
	}

	// Prevent duplicate Goods Receipts for the same Purchase Order
	existingGRs, err := uc.grRepo.FindByRef(ctx, models.GoodsReceiptRefPurO, purOID)
	if err == nil && len(existingGRs) > 0 {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: PO %s already has a Goods Receipt", purOID)
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: at least one line is required")
	}
	for i, l := range lines {
		if l.QtyReceived < 0 {
			return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: line %d qty_received must be ≥ 0", i)
		}
	}

	hasDiscrepancy := false
	for _, l := range lines {
		if l.QtyReceived < l.QtyExpected {
			hasDiscrepancy = true
			break
		}
	}

	grStatus := models.GoodsReceiptConfirmed
	if hasDiscrepancy {
		grStatus = models.GoodsReceiptDiscrepancy
	}

	now := time.Now()
	gr := &models.GoodsReceipt{
		ID:              uuid.NewString(),
		RefType:         models.GoodsReceiptRefPurO,
		RefID:           purOID,
		ReceivingNodeID: receivingNodeID,
		Status:          grStatus,
		ReceivedBy:      staffID,
		ReceivedAt:      &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.grRepo.Create(ctx, gr); err != nil {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: persist GR: %w", err)
	}

	for _, l := range lines {
		grLine := &models.GoodsReceiptLine{
			ID:          uuid.NewString(),
			GRID:        gr.ID,
			ItemID:      l.ItemID,
			QtyExpected: l.QtyExpected,
			QtyReceived: l.QtyReceived,
		}
		if err := uc.grLine.AddLine(ctx, grLine); err != nil {
			return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: add GR line %s: %w", l.ItemID, err)
		}

		// StockIn at receiving node using actually received quantity (base units).
		if l.QtyReceived > 0 && l.ItemID != "" {
			if err := uc.inv.StockIn(ctx, receivingNodeID, l.ItemID, l.QtyReceived); err != nil {
				return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: StockIn item %s: %w", l.ItemID, err)
			}
		}

		// Auto-create DiscrepancyTicket for any shortfall.
		if l.QtyReceived < l.QtyExpected {
			dt := &models.DiscrepancyTicket{
				ID:         uuid.NewString(),
				GRID:       gr.ID,
				ItemID:     l.ItemID,
				QtyMissing: l.QtyExpected - l.QtyReceived,
				QtyDamaged: 0,
				Status:     models.DiscrepancyOpen,
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := uc.dtRepo.Create(ctx, dt); err != nil {
				return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: create DiscrepancyTicket item %s: %w", l.ItemID, err)
			}
		}
	}

	// Update PurchaseOrder status to DELIVERED
	purO.Status = models.PurchaseOrderDelivered
	purO.UpdatedAt = now
	if err := uc.purORepo.Update(ctx, purO); err != nil {
		return nil, fmt.Errorf("gr: ConfirmPurOGoodsReceipt: update PO status: %w", err)
	}

	return gr, nil
}

func (uc *grUseCase) GetGR(ctx context.Context, grID string) (*models.GoodsReceipt, []*models.GoodsReceiptLine, error) {
	gr, err := uc.grRepo.FindByID(ctx, grID)
	if err != nil || gr == nil {
		return nil, nil, fmt.Errorf("gr: GetGR: GR %s not found: %w", grID, err)
	}
	lines, err := uc.grLine.ListByGR(ctx, grID)
	return gr, lines, err
}
