package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// ItemRepository defines persistence operations for Item.
type ItemRepository interface {
	Create(ctx context.Context, item *models.Item) error
	FindByID(ctx context.Context, id string) (*models.Item, error)
	FindAll(ctx context.Context) ([]*models.Item, error)
	FindByOrgID(ctx context.Context, orgID string) ([]*models.Item, error)
	Update(ctx context.Context, item *models.Item) error
	Delete(ctx context.Context, id string) error
}

// ItemCapacityConfigRepository defines persistence for the bin-packing input table.
// For each (Item × EquipmentType) pair, it records slot consumption and mix rules.
type ItemCapacityConfigRepository interface {
	Save(ctx context.Context, cfg *models.ItemCapacityConfig) error
	// Get returns the capacity config for an (item, equipmentType) pair.
	// Returns nil, nil when no config exists.
	Get(ctx context.Context, itemID, equipmentTypeID string) (*models.ItemCapacityConfig, error)
	// ListByEquipmentType returns all items configured for a given equipment type.
	// Used by the allocation engine to check slot consumption for queued batches.
	ListByEquipmentType(ctx context.Context, equipmentTypeID string) ([]*models.ItemCapacityConfig, error)
	Delete(ctx context.Context, itemID, equipmentTypeID string) error
}
