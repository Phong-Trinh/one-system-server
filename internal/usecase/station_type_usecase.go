package usecase

import (
	"context"
	"fmt"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

// EquipmentTypeUseCase manages EquipmentType records (formerly called StationType).
// EquipmentType is the category of kitchen equipment (e.g., FRYER, OVEN, GRILL).
// It is the model equivalent of what the business spec calls "StationType".
type EquipmentTypeUseCase interface {
	Create(ctx context.Context, id, name, capacityUnit string) (*models.EquipmentType, error)
	GetByID(ctx context.Context, id string) (*models.EquipmentType, error)
	List(ctx context.Context) ([]*models.EquipmentType, error)
	Update(ctx context.Context, id, name, capacityUnit string) (*models.EquipmentType, error)
	Delete(ctx context.Context, id string) error
}

// StationTypeUseCase is an alias for EquipmentTypeUseCase kept for backward compatibility
// with existing transport-layer callers. New code should use EquipmentTypeUseCase directly.
type StationTypeUseCase = EquipmentTypeUseCase

// ── Implementation ────────────────────────────────────────────────────────────

type equipmentTypeUseCase struct {
	repo services.EquipmentTypeRepository
}

// NewEquipmentTypeUseCase constructs the EquipmentTypeUseCase.
func NewEquipmentTypeUseCase(repo services.EquipmentTypeRepository) EquipmentTypeUseCase {
	return &equipmentTypeUseCase{repo: repo}
}

// NewStationTypeUseCase is an alias constructor for backward compatibility.
func NewStationTypeUseCase(repo services.EquipmentTypeRepository) EquipmentTypeUseCase {
	return NewEquipmentTypeUseCase(repo)
}

// Create uses caller-supplied id (e.g., "FRYER", "OVEN") — EquipmentType is enum-style.
func (uc *equipmentTypeUseCase) Create(ctx context.Context, id, name, capacityUnit string) (*models.EquipmentType, error) {
	if id == "" || name == "" || capacityUnit == "" {
		return nil, fmt.Errorf("id, name, and capacity_unit are required")
	}
	et := &models.EquipmentType{
		ID:           id,
		Name:         name,
		CapacityUnit: capacityUnit,
		Status:       models.EquipmentTypeActive,
	}
	if err := uc.repo.Create(ctx, et); err != nil {
		return nil, err
	}
	return et, nil
}

func (uc *equipmentTypeUseCase) GetByID(ctx context.Context, id string) (*models.EquipmentType, error) {
	et, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if et == nil {
		return nil, fmt.Errorf("equipment type %q not found", id)
	}
	return et, nil
}

func (uc *equipmentTypeUseCase) List(ctx context.Context) ([]*models.EquipmentType, error) {
	return uc.repo.FindAll(ctx)
}

func (uc *equipmentTypeUseCase) Update(ctx context.Context, id, name, capacityUnit string) (*models.EquipmentType, error) {
	et, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if et == nil {
		return nil, fmt.Errorf("equipment type %q not found", id)
	}
	et.Name = name
	et.CapacityUnit = capacityUnit
	if err := uc.repo.Update(ctx, et); err != nil {
		return nil, err
	}
	return et, nil
}

func (uc *equipmentTypeUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}
