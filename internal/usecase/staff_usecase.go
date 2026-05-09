package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type StaffUseCase interface {
	Create(ctx context.Context, nodeID, name string, wageRate float64) (*models.Staff, error)
	GetByID(ctx context.Context, id string) (*models.Staff, error)
	ListByNode(ctx context.Context, nodeID string) ([]*models.Staff, error)
	Update(ctx context.Context, id, name string, wageRate float64) (*models.Staff, error)
	Delete(ctx context.Context, id string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type staffUseCase struct {
	staffRepo services.StaffRepository
	nodeRepo  services.NodeRepository
}

func NewStaffUseCase(staffRepo services.StaffRepository, nodeRepo services.NodeRepository) StaffUseCase {
	return &staffUseCase{staffRepo: staffRepo, nodeRepo: nodeRepo}
}

func (uc *staffUseCase) Create(ctx context.Context, nodeID, name string, wageRate float64) (*models.Staff, error) {
	if name == "" {
		return nil, fmt.Errorf("staff name is required")
	}
	if wageRate < 0 {
		return nil, fmt.Errorf("wage_rate cannot be negative")
	}
	// Validate node exists
	node, err := uc.nodeRepo.FindByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}

	s := &models.Staff{
		ID:       uuid.NewString(),
		NodeID:   nodeID,
		Name:     name,
		WageRate: wageRate,
	}
	if err := uc.staffRepo.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *staffUseCase) GetByID(ctx context.Context, id string) (*models.Staff, error) {
	s, err := uc.staffRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("staff %q not found", id)
	}
	return s, nil
}

func (uc *staffUseCase) ListByNode(ctx context.Context, nodeID string) ([]*models.Staff, error) {
	return uc.staffRepo.FindByNodeID(ctx, nodeID)
}

func (uc *staffUseCase) Update(ctx context.Context, id, name string, wageRate float64) (*models.Staff, error) {
	s, err := uc.staffRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("staff %q not found", id)
	}
	if wageRate < 0 {
		return nil, fmt.Errorf("wage_rate cannot be negative")
	}
	s.Name = name
	s.WageRate = wageRate
	if err := uc.staffRepo.Update(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

func (uc *staffUseCase) Delete(ctx context.Context, id string) error {
	return uc.staffRepo.Delete(ctx, id)
}
