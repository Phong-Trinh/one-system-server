package usecase

import (
	"context"
	"fmt"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type StationTypeUseCase interface {
	Create(ctx context.Context, id, name, capacityUnit string) (*models.StationType, error)
	GetByID(ctx context.Context, id string) (*models.StationType, error)
	List(ctx context.Context) ([]*models.StationType, error)
	Update(ctx context.Context, id, name, capacityUnit string) (*models.StationType, error)
	Delete(ctx context.Context, id string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type stationTypeUseCase struct {
	repo services.StationTypeRepository
}

func NewStationTypeUseCase(repo services.StationTypeRepository) StationTypeUseCase {
	return &stationTypeUseCase{repo: repo}
}

// Create uses caller-supplied id (e.g. "FRYER", "OVEN") — StationType is enum-style.
func (uc *stationTypeUseCase) Create(ctx context.Context, id, name, capacityUnit string) (*models.StationType, error) {
	if id == "" || name == "" || capacityUnit == "" {
		return nil, fmt.Errorf("id, name, and capacity_unit are required")
	}
	st := &models.StationType{ID: id, Name: name, CapacityUnit: capacityUnit}
	if err := uc.repo.Create(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (uc *stationTypeUseCase) GetByID(ctx context.Context, id string) (*models.StationType, error) {
	st, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("station type %q not found", id)
	}
	return st, nil
}

func (uc *stationTypeUseCase) List(ctx context.Context) ([]*models.StationType, error) {
	return uc.repo.FindAll(ctx)
}

func (uc *stationTypeUseCase) Update(ctx context.Context, id, name, capacityUnit string) (*models.StationType, error) {
	st, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, fmt.Errorf("station type %q not found", id)
	}
	st.Name = name
	st.CapacityUnit = capacityUnit
	if err := uc.repo.Update(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

func (uc *stationTypeUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}
