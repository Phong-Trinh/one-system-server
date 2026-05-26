package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// GIUseCase manages GoodsIssue creation for B2B Sales Orders.
// ITO-linked GoodsIssues are handled inside ITOUseCase.ConfirmGoodsIssue.
//
// B2B GI rules:
//   - Driver info (name, phone, vehicle plate) and media proof are always required
//     (B2B shipments are always cross-site to an external customer).
//   - Confirming a GI triggers StockOut at the issuing Factory node.
//   - ROP check fires after StockOut — result returned to facade for replenishment decisions.
type GIUseCase interface {
	// ConfirmB2BGoodsIssue dispatches goods from a Factory to an external B2B customer.
	// Requires driver info and media proof (always cross-site for B2B).
	// Triggers StockOut at the issuing node. Returns the ROPCheckResult per item
	// so the SupplyChainFacade can fire replenishment documents if needed.
	ConfirmB2BGoodsIssue(ctx context.Context, b2bOrderID, issuingNodeID string, input GoodsIssueInput) (*models.GoodsIssue, []*services.ROPCheckResult, error)

	GetGI(ctx context.Context, giID string) (*models.GoodsIssue, []*models.GoodsIssueLine, error)
}

// giUseCase implements GIUseCase.
type giUseCase struct {
	giRepo  services.GoodsIssueRepository
	giLine  services.GoodsIssueLineRepository
	inv     services.InventoryService
}

func newGIUseCase(
	giRepo services.GoodsIssueRepository,
	giLine services.GoodsIssueLineRepository,
	inv services.InventoryService,
) GIUseCase {
	return &giUseCase{
		giRepo: giRepo,
		giLine: giLine,
		inv:    inv,
	}
}

// ConfirmB2BGoodsIssue creates and confirms a GoodsIssue for a B2B Sales Order dispatch.
func (uc *giUseCase) ConfirmB2BGoodsIssue(ctx context.Context, b2bOrderID, issuingNodeID string, input GoodsIssueInput) (*models.GoodsIssue, []*services.ROPCheckResult, error) {
	// B2B shipments are always cross-site — driver info and media are mandatory.
	if input.DriverName == "" || input.VehiclePlate == "" || input.MediaURL == "" {
		return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: driver_name, vehicle_plate, and media_url are required for B2B dispatch")
	}
	if len(input.Lines) == 0 {
		return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: at least one line is required")
	}

	now := time.Now()
	gi := &models.GoodsIssue{
		ID:            uuid.NewString(),
		RefType:       models.GoodsIssueRefB2B,
		RefID:         b2bOrderID,
		IssuingNodeID: issuingNodeID,
		DriverName:    input.DriverName,
		DriverPhone:   input.DriverPhone,
		VehiclePlate:  input.VehiclePlate,
		MediaURL:      input.MediaURL,
		ShippingFee:   input.ShippingFee,
		Status:        models.GoodsIssueConfirmed,
		IssuedAt:      &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := uc.giRepo.Create(ctx, gi); err != nil {
		return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: persist GI: %w", err)
	}

	var ropResults []*services.ROPCheckResult

	for _, l := range input.Lines {
		if l.QtyIssued <= 0 {
			return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: line item %s qty_issued must be > 0", l.ItemID)
		}

		giLine := &models.GoodsIssueLine{
			ID:        uuid.NewString(),
			GIID:      gi.ID,
			ItemID:    l.ItemID,
			QtyIssued: l.QtyIssued,
		}
		if err := uc.giLine.AddLine(ctx, giLine); err != nil {
			return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: add line %s: %w", l.ItemID, err)
		}

		// StockOut at issuing Factory — triggers ROP check.
		ropResult, err := uc.inv.StockOut(ctx, issuingNodeID, l.ItemID, l.QtyIssued)
		if err != nil {
			return nil, nil, fmt.Errorf("gi: ConfirmB2BGoodsIssue: StockOut item %s: %w", l.ItemID, err)
		}
		if ropResult != nil {
			ropResults = append(ropResults, ropResult)
		}
	}

	return gi, ropResults, nil
}

func (uc *giUseCase) GetGI(ctx context.Context, giID string) (*models.GoodsIssue, []*models.GoodsIssueLine, error) {
	gi, err := uc.giRepo.FindByID(ctx, giID)
	if err != nil || gi == nil {
		return nil, nil, fmt.Errorf("gi: GetGI: GI %s not found: %w", giID, err)
	}
	lines, err := uc.giLine.ListByGI(ctx, giID)
	return gi, lines, err
}
