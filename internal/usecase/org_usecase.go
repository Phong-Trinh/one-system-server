package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type OrgUseCase interface {
	Create(ctx context.Context, name string) (*models.Organization, error)
	GetByID(ctx context.Context, id string) (*models.Organization, error)
	List(ctx context.Context) ([]*models.Organization, error)
	Update(ctx context.Context, id, name string) (*models.Organization, error)
	Delete(ctx context.Context, id string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type orgUseCase struct {
	repo services.OrgRepository
}

func NewOrgUseCase(repo services.OrgRepository) OrgUseCase {
	return &orgUseCase{repo: repo}
}

func (uc *orgUseCase) Create(ctx context.Context, name string) (*models.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("org name is required")
	}
	now := time.Now().UTC()
	org := &models.Organization{
		ID:        uuid.NewString(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := uc.repo.Create(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (uc *orgUseCase) GetByID(ctx context.Context, id string) (*models.Organization, error) {
	org, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("organization %q not found", id)
	}
	return org, nil
}

func (uc *orgUseCase) List(ctx context.Context) ([]*models.Organization, error) {
	return uc.repo.FindAll(ctx)
}

func (uc *orgUseCase) Update(ctx context.Context, id, name string) (*models.Organization, error) {
	org, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("organization %q not found", id)
	}
	org.Name = name
	org.UpdatedAt = time.Now().UTC()
	if err := uc.repo.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (uc *orgUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}
