package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

type ItemUseCase interface {
	Create(ctx context.Context, orgID, name, sku string, itemType models.ItemType, baseUnit string) (*models.Item, error)
	GetByID(ctx context.Context, id string) (*models.Item, error)
	List(ctx context.Context) ([]*models.Item, error)
	ListByOrg(ctx context.Context, orgID string) ([]*models.Item, error)
	Update(ctx context.Context, id string, name, sku string, itemType models.ItemType, baseUnit string) (*models.Item, error)
	Delete(ctx context.Context, id string) error
}

type itemUseCase struct {
	repo services.ItemRepository
}

func NewItemUseCase(repo services.ItemRepository) ItemUseCase {
	return &itemUseCase{repo: repo}
}

func (uc *itemUseCase) Create(ctx context.Context, orgID, name, sku string, itemType models.ItemType, baseUnit string) (*models.Item, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	item := &models.Item{
		ID:       uuid.NewString(),
		OrgID:    orgID,
		Name:     name,
		SKU:      sku,
		Type:     itemType,
		BaseUnit: baseUnit,
	}
	if err := uc.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *itemUseCase) GetByID(ctx context.Context, id string) (*models.Item, error) {
	item, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("item %q not found", id)
	}
	return item, nil
}

func (uc *itemUseCase) List(ctx context.Context) ([]*models.Item, error) {
	return uc.repo.FindAll(ctx)
}

func (uc *itemUseCase) ListByOrg(ctx context.Context, orgID string) ([]*models.Item, error) {
	return uc.repo.FindByOrgID(ctx, orgID)
}

func (uc *itemUseCase) Update(ctx context.Context, id string, name, sku string, itemType models.ItemType, baseUnit string) (*models.Item, error) {
	item, err := uc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	item.Name = name
	item.SKU = sku
	item.Type = itemType
	item.BaseUnit = baseUnit
	
	if err := uc.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (uc *itemUseCase) Delete(ctx context.Context, id string) error {
	return uc.repo.Delete(ctx, id)
}
