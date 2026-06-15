package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// prUseCase implements services.PRService.
// It is embedded inside SupplyChainFacade and should not be instantiated directly.
type prUseCase struct {
	prRepo     services.PurchaseRequisitionRepository
	lineRepo   services.PRLineRepository
	eqTypeRepo services.EquipmentTypeRepository
}

func newPRUseCase(
	prRepo services.PurchaseRequisitionRepository,
	lineRepo services.PRLineRepository,
	eqTypeRepo services.EquipmentTypeRepository,
) services.PRService {
	return &prUseCase{
		prRepo:     prRepo,
		lineRepo:   lineRepo,
		eqTypeRepo: eqTypeRepo,
	}
}

// SubmitPR creates a PurchaseRequisition in PENDING_HQ_APPROVAL status.
// Validates: at least one line, and each line has either ItemID or EquipmentTypeID (not both empty).
func (uc *prUseCase) SubmitPR(ctx context.Context, req services.SubmitPRRequest) (*models.PurchaseRequisition, error) {
	if len(req.Lines) == 0 {
		return nil, fmt.Errorf("pr: SubmitPR: at least one line is required")
	}
	for i, l := range req.Lines {
		if l.ItemID == nil && l.EquipmentTypeID == nil {
			return nil, fmt.Errorf("pr: SubmitPR: line %d must have either item_id or equipment_type_id", i)
		}
		if l.Qty <= 0 {
			return nil, fmt.Errorf("pr: SubmitPR: line %d qty must be > 0", i)
		}
	}

	now := time.Now()
	pr := &models.PurchaseRequisition{
		ID:              uuid.NewString(),
		OrgID:           req.OrgID,
		RequesterNodeID: req.RequesterNodeID,
		RequesterStaff:  req.StaffID,
		Status:          models.PRPendingHQApproval,
		Justification:   req.Justification,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.prRepo.Create(ctx, pr); err != nil {
		return nil, fmt.Errorf("pr: SubmitPR: persist: %w", err)
	}

	for i, l := range req.Lines {
		// If it's a proposed new equipment type, create a DRAFT EquipmentType if it doesn't exist
		if l.EquipmentTypeID != nil && *l.EquipmentTypeID != "" && l.ProposedEquipmentName != nil && *l.ProposedEquipmentName != "" {
			exists, _ := uc.eqTypeRepo.FindByID(ctx, *l.EquipmentTypeID)
			if exists == nil {
				capacityUnit := "unit"
				if l.ProposedCapacityUnit != nil && *l.ProposedCapacityUnit != "" {
					capacityUnit = *l.ProposedCapacityUnit
				}
				draftEqType := &models.EquipmentType{
					ID:           *l.EquipmentTypeID,
					Name:         *l.ProposedEquipmentName,
					CapacityUnit: capacityUnit,
					Status:       models.EquipmentTypeDraft,
				}
				if err := uc.eqTypeRepo.Create(ctx, draftEqType); err != nil {
					return nil, fmt.Errorf("pr: SubmitPR: create draft equipment type %s: %w", *l.EquipmentTypeID, err)
				}
			}
		}

		line := &models.PRLine{
			ID:                    uuid.NewString(),
			PRID:                  pr.ID,
			ItemID:                l.ItemID,
			EquipmentTypeID:       l.EquipmentTypeID,
			ProposedEquipmentName: l.ProposedEquipmentName,
			ProposedCapacityUnit:  l.ProposedCapacityUnit,
			ExpectedCapacity:      l.ExpectedCapacity,
			Qty:                   l.Qty,
			UnitOfMeasure:         l.UnitOfMeasure,
			EstimatedUnitPrice:    l.EstimatedUnitPrice,
			Justification:         l.Justification,
			Description:           l.Description,
		}
		if err := uc.lineRepo.AddLine(ctx, line); err != nil {
			return nil, fmt.Errorf("pr: SubmitPR: add line %d: %w", i, err)
		}
	}

	return pr, nil
}

// ApprovePR applies HQ corrections to all lines, activates any draft EquipmentTypes,
// then transitions the PR to APPROVED — all atomically.
// HQ corrections are written back to the PR lines so the PR is the authoritative record.
func (uc *prUseCase) ApprovePR(ctx context.Context, prID, reviewerStaffID string, note *string, corrections []services.PRLineCorrection) error {
	pr, err := uc.loadPR(ctx, prID)
	if err != nil {
		return err
	}
	if pr.Status != models.PRPendingHQApproval {
		return fmt.Errorf("pr: ApprovePR: PR %s is not in PENDING_HQ_APPROVAL (current: %s)", prID, pr.Status)
	}

	// Load current lines to validate corrections
	lines, err := uc.lineRepo.ListByPR(ctx, prID)
	if err != nil {
		return fmt.Errorf("pr: ApprovePR: list lines: %w", err)
	}

	// Build index for fast lookup
	lineByID := make(map[string]*models.PRLine, len(lines))
	for _, l := range lines {
		lineByID[l.ID] = l
	}

	// Step 1: Validate and apply HQ corrections to each line
	correctionByLineID := make(map[string]services.PRLineCorrection, len(corrections))
	for _, c := range corrections {
		correctionByLineID[c.LineID] = c
	}

	for _, line := range lines {
		c, ok := correctionByLineID[line.ID]
		if !ok {
			return fmt.Errorf("pr: ApprovePR: missing HQ correction for line %s", line.ID)
		}
		if c.EquipmentTypeID == "" && line.ItemID == nil {
			return fmt.Errorf("pr: ApprovePR: line %s must have a verified equipment_type_id", line.ID)
		}
		if c.Qty <= 0 {
			return fmt.Errorf("pr: ApprovePR: line %s qty must be > 0", line.ID)
		}

		// Apply corrections to the line model
		eqTypeID := c.EquipmentTypeID
		line.EquipmentTypeID = &eqTypeID
		line.ExpectedCapacity = c.ExpectedCapacity
		line.ProposedEquipmentName = nil // HQ has verified; clear the free-text field
		line.Qty = c.Qty
		line.UnitOfMeasure = c.UnitOfMeasure
		line.EstimatedUnitPrice = c.EstimatedPrice

		// Persist corrected line back to the database
		if err := uc.lineRepo.UpdateLine(ctx, line); err != nil {
			return fmt.Errorf("pr: ApprovePR: update line %s: %w", line.ID, err)
		}
	}

	// Step 2: Auto-activate any DRAFT EquipmentTypes referenced by the (now-corrected) lines
	for _, line := range lines {
		if line.EquipmentTypeID != nil && *line.EquipmentTypeID != "" {
			eqType, err := uc.eqTypeRepo.FindByID(ctx, *line.EquipmentTypeID)
			if err == nil && eqType != nil && eqType.Status == models.EquipmentTypeDraft {
				eqType.Status = models.EquipmentTypeActive
				if err := uc.eqTypeRepo.Update(ctx, eqType); err != nil {
					return fmt.Errorf("pr: ApprovePR: activate equipment type %s: %w", *line.EquipmentTypeID, err)
				}
			}
		}
	}

	// Step 3: Transition PR to APPROVED
	now := time.Now()
	pr.Status = models.PRApproved
	pr.ReviewedBy = &reviewerStaffID
	pr.ReviewNote = note
	pr.ReviewedAt = &now
	pr.UpdatedAt = now

	return uc.prRepo.Update(ctx, pr)
}


// RejectPR transitions a PENDING_HQ_APPROVAL PR to REJECTED with a mandatory reason.
func (uc *prUseCase) RejectPR(ctx context.Context, prID, reviewerStaffID, reason string) error {
	if reason == "" {
		return fmt.Errorf("pr: RejectPR: rejection reason is required")
	}
	pr, err := uc.loadPR(ctx, prID)
	if err != nil {
		return err
	}
	if pr.Status != models.PRPendingHQApproval {
		return fmt.Errorf("pr: RejectPR: PR %s is not in PENDING_HQ_APPROVAL (current: %s)", prID, pr.Status)
	}

	now := time.Now()
	pr.Status = models.PRRejected
	pr.ReviewedBy = &reviewerStaffID
	pr.ReviewNote = &reason
	pr.ReviewedAt = &now
	pr.UpdatedAt = now

	return uc.prRepo.Update(ctx, pr)
}

func (uc *prUseCase) GetPR(ctx context.Context, prID string) (*models.PurchaseRequisition, []*models.PRLine, error) {
	pr, err := uc.loadPR(ctx, prID)
	if err != nil {
		return nil, nil, err
	}
	lines, err := uc.lineRepo.ListByPR(ctx, prID)
	return pr, lines, err
}

func (uc *prUseCase) ListPRsByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error) {
	return uc.prRepo.FindByNode(ctx, nodeID)
}

func (uc *prUseCase) ListPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error) {
	return uc.prRepo.FindPendingByOrg(ctx, orgID)
}

func (uc *prUseCase) loadPR(ctx context.Context, id string) (*models.PurchaseRequisition, error) {
	pr, err := uc.prRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("pr: load PR %s: %w", id, err)
	}
	if pr == nil {
		return nil, fmt.Errorf("pr: PR %s not found", id)
	}
	return pr, nil
}
