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

type NodeUseCase interface {
	Create(ctx context.Context, orgID string, nodeType models.NodeType, name, address string) (*models.Node, error)
	GetByID(ctx context.Context, id string) (*models.Node, error)
	ListByOrg(ctx context.Context, orgID string) ([]*models.Node, error)
	ListAll(ctx context.Context) ([]*models.Node, error)
	Update(ctx context.Context, id string, name, address string) (*models.Node, error)
	Delete(ctx context.Context, id string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type nodeUseCase struct {
	nodeRepo services.NodeRepository
	orgRepo  services.OrgRepository
}

func NewNodeUseCase(nodeRepo services.NodeRepository, orgRepo services.OrgRepository) NodeUseCase {
	return &nodeUseCase{nodeRepo: nodeRepo, orgRepo: orgRepo}
}

func (uc *nodeUseCase) Create(ctx context.Context, orgID string, nodeType models.NodeType, name, address string) (*models.Node, error) {
	// Validate org exists
	org, err := uc.orgRepo.FindByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, fmt.Errorf("organization %q not found", orgID)
	}

	// v1 rule: one HQ and one FACTORY per org
	if nodeType == models.NodeHQ || nodeType == models.NodeFactory {
		existing, err := uc.nodeRepo.FindByOrgID(ctx, orgID)
		if err != nil {
			return nil, err
		}
		for _, n := range existing {
			if n.Type == nodeType {
				return nil, fmt.Errorf("org %q already has a %s node", orgID, nodeType)
			}
		}
	}

	now := time.Now().UTC()
	node := &models.Node{
		ID:        uuid.NewString(),
		OrgID:     orgID,
		Type:      nodeType,
		Name:      name,
		Address:   address,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := uc.nodeRepo.Create(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

func (uc *nodeUseCase) GetByID(ctx context.Context, id string) (*models.Node, error) {
	node, err := uc.nodeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found", id)
	}
	return node, nil
}

func (uc *nodeUseCase) ListByOrg(ctx context.Context, orgID string) ([]*models.Node, error) {
	return uc.nodeRepo.FindByOrgID(ctx, orgID)
}

func (uc *nodeUseCase) ListAll(ctx context.Context) ([]*models.Node, error) {
	return uc.nodeRepo.FindAll(ctx)
}

func (uc *nodeUseCase) Update(ctx context.Context, id, name, address string) (*models.Node, error) {
	node, err := uc.nodeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found", id)
	}
	node.Name = name
	node.Address = address
	node.UpdatedAt = time.Now().UTC()
	if err := uc.nodeRepo.Update(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

func (uc *nodeUseCase) Delete(ctx context.Context, id string) error {
	return uc.nodeRepo.Delete(ctx, id)
}
