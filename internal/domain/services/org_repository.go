package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// OrgRepository defines persistence operations for Organization.
type OrgRepository interface {
	Create(ctx context.Context, org *models.Organization) error
	FindByID(ctx context.Context, id string) (*models.Organization, error)
	FindAll(ctx context.Context) ([]*models.Organization, error)
	Update(ctx context.Context, org *models.Organization) error
	Delete(ctx context.Context, id string) error
}

// NodeRepository defines persistence operations for Node.
type NodeRepository interface {
	Create(ctx context.Context, node *models.Node) error
	FindByID(ctx context.Context, id string) (*models.Node, error)
	FindByOrgID(ctx context.Context, orgID string) ([]*models.Node, error)
	Update(ctx context.Context, node *models.Node) error
	Delete(ctx context.Context, id string) error
}
