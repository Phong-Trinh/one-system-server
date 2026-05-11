package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type MachineUseCase interface {
	Create(ctx context.Context, nodeID, stationTypeID string, maxSlots int) (*models.Machine, error)
	GetByID(ctx context.Context, id string) (*models.Machine, error)
	ListByNode(ctx context.Context, nodeID string) ([]*models.Machine, error)
	ListAll(ctx context.Context) ([]*models.Machine, error)
	Update(ctx context.Context, id string, maxSlots int) (*models.Machine, error)
	Delete(ctx context.Context, id string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type machineUseCase struct {
	machineRepo     services.MachineRepository
	nodeRepo        services.NodeRepository
	stationTypeRepo services.StationTypeRepository
}

func NewMachineUseCase(
	machineRepo services.MachineRepository,
	nodeRepo services.NodeRepository,
	stationTypeRepo services.StationTypeRepository,
) MachineUseCase {
	return &machineUseCase{
		machineRepo:     machineRepo,
		nodeRepo:        nodeRepo,
		stationTypeRepo: stationTypeRepo,
	}
}

func (uc *machineUseCase) Create(ctx context.Context, nodeID, stationTypeID string, maxSlots int) (*models.Machine, error) {
	if maxSlots <= 0 {
		return nil, fmt.Errorf("max_slots must be positive")
	}
	// Validate node exists
	node, err := uc.nodeRepo.FindByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}
	// Validate station type exists
	st, err := uc.stationTypeRepo.FindByID(ctx, stationTypeID)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("station type %q not found", stationTypeID)
	}

	m := &models.Machine{
		ID:            uuid.NewString(),
		StationTypeID: stationTypeID,
		NodeID:        nodeID,
		MaxSlots:      maxSlots,
		Status:        models.MachineIdle,
	}
	if err := uc.machineRepo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *machineUseCase) GetByID(ctx context.Context, id string) (*models.Machine, error) {
	m, err := uc.machineRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("machine %q not found", id)
	}
	return m, nil
}

func (uc *machineUseCase) ListByNode(ctx context.Context, nodeID string) ([]*models.Machine, error) {
	return uc.machineRepo.FindByNodeID(ctx, nodeID)
}

func (uc *machineUseCase) ListAll(ctx context.Context) ([]*models.Machine, error) {
	return uc.machineRepo.FindAll(ctx)
}

func (uc *machineUseCase) Update(ctx context.Context, id string, maxSlots int) (*models.Machine, error) {
	m, err := uc.machineRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("machine %q not found", id)
	}
	if maxSlots <= 0 {
		return nil, fmt.Errorf("max_slots must be positive")
	}
	m.MaxSlots = maxSlots
	if err := uc.machineRepo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (uc *machineUseCase) Delete(ctx context.Context, id string) error {
	return uc.machineRepo.Delete(ctx, id)
}
