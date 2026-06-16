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

