package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── OrderUseCase ──────────────────────────────────────────────────────────────

// OrderUseCase manages customer sale orders at Store nodes.
// When an order is COMPLETED, it deducts stock and fires ROP checks via the SupplyChainFacade.
type OrderUseCase interface {
	// CreateOrder creates a new sale order (status = PENDING).
	CreateOrder(ctx context.Context, nodeID, source string, platform *string, items []models.OrderItem, deadlineAt *time.Time) (*models.Order, error)

	// GetOrder retrieves a single order by ID.
	GetOrder(ctx context.Context, id string) (*models.Order, error)

	// ListByNode returns all orders for a given Store node.
	ListByNode(ctx context.Context, nodeID string) ([]*models.Order, error)

	// CompleteOrder transitions order to COMPLETED and fires StockOut for each item.
	// The facade callback fires ROP replenishment if any item hits its reorder point.
	CompleteOrder(ctx context.Context, id, orgID, hqNodeID string) error

	// CancelOrder cancels an order.
	CancelOrder(ctx context.Context, id string) error
}

// ── Repository interface (minimal — Order repo) ───────────────────────────────

// OrderRepository defines persistence for customer sale orders.
type OrderRepository interface {
	Create(ctx context.Context, order *models.Order) error
	FindByID(ctx context.Context, id string) (*models.Order, error)
	FindByNode(ctx context.Context, nodeID string) ([]*models.Order, error)
	UpdateStatus(ctx context.Context, id string, status models.OrderStatus) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type orderUseCase struct {
	repo    OrderRepository
	invSvc  services.InventoryService
	facade  *SupplyChainFacade
}

// NewOrderUseCase constructs the order use case.
// facade is used to call StockOutWithROP on order completion.
func NewOrderUseCase(repo OrderRepository, facade *SupplyChainFacade) OrderUseCase {
	return &orderUseCase{
		repo:   repo,
		invSvc: facade.Inventory,
		facade: facade,
	}
}

func (uc *orderUseCase) CreateOrder(ctx context.Context, nodeID, source string, platform *string, items []models.OrderItem, deadlineAt *time.Time) (*models.Order, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("order: at least one item required")
	}

	var total float64
	for _, it := range items {
		total += it.Price * float64(it.Quantity)
	}

	// [NEW] Check Stock Availability Before Order Creation
	for _, it := range items {
		qtyBU := float64(it.Quantity)
		_, bomLines, err := uc.facade.GetBOMByItem(ctx, it.ItemID)
		if err == nil && len(bomLines) > 0 {
			for _, line := range bomLines {
				ingQty := qtyBU * line.Qty
				avail, err := uc.invSvc.GetStock(ctx, nodeID, line.ItemID)
				if err != nil || avail < ingQty {
					return nil, fmt.Errorf("insufficient stock for ingredient %s: have %.2f, need %.2f", line.ItemID, avail, ingQty)
				}
			}
		} else {
			avail, err := uc.invSvc.GetStock(ctx, nodeID, it.ItemID)
			if err != nil || avail < qtyBU {
				return nil, fmt.Errorf("insufficient stock for item %s: have %.2f, need %.2f", it.ItemID, avail, qtyBU)
			}
		}
	}

	src := models.OrderSourceDirectPOS
	if source == "PLATFORM" {
		src = models.OrderSourcePlatform
	}

	order := &models.Order{
		ID:          uuid.NewString(),
		NodeID:      nodeID,
		Source:      src,
		Platform:    platform,
		Status:      models.OrderStatusPending,
		TotalAmount: total,
		Items:       items,
		DeadlineAt:  deadlineAt,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := uc.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("order: create: %w", err)
	}
	return order, nil
}

func (uc *orderUseCase) GetOrder(ctx context.Context, id string) (*models.Order, error) {
	o, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("order %q not found", id)
	}
	return o, nil
}

func (uc *orderUseCase) ListByNode(ctx context.Context, nodeID string) ([]*models.Order, error) {
	return uc.repo.FindByNode(ctx, nodeID)
}

// CompleteOrder marks the order COMPLETED, deducts stock for each item,
// and fires ROP checks via the SupplyChainFacade.
func (uc *orderUseCase) CompleteOrder(ctx context.Context, id, orgID, hqNodeID string) error {
	order, err := uc.GetOrder(ctx, id)
	if err != nil {
		return err
	}
	if order.Status == models.OrderStatusCompleted {
		return fmt.Errorf("order %s is already completed", id)
	}
	if order.Status == models.OrderStatusCancelled {
		return fmt.Errorf("order %s is cancelled — cannot complete", id)
	}

	// Deduct stock and fire ROP for each item line.
	for _, item := range order.Items {
		qtyBU := float64(item.Quantity) // PRODUCT items: qty is already in base units (pieces)

		// POS BOM Backflushing: if item has a BOM, deduct ingredients instead of the item itself.
		_, bomLines, err := uc.facade.GetBOMByItem(ctx, item.ItemID)
		if err == nil && len(bomLines) > 0 {
			// Backflush ingredients
			for _, line := range bomLines {
				ingQty := qtyBU * line.Qty
				if err := uc.facade.StockOutWithROP(ctx, orgID, hqNodeID, order.NodeID, line.ItemID, ingQty); err != nil {
					return fmt.Errorf("order: CompleteOrder: StockOutWithROP ingredient %s: %w", line.ItemID, err)
				}
			}
		} else {
			// No BOM found, deduct item directly
			if err := uc.facade.StockOutWithROP(ctx, orgID, hqNodeID, order.NodeID, item.ItemID, qtyBU); err != nil {
				return fmt.Errorf("order: CompleteOrder: StockOutWithROP item %s: %w", item.ItemID, err)
			}
		}
	}

	return uc.repo.UpdateStatus(ctx, id, models.OrderStatusCompleted)
}

func (uc *orderUseCase) CancelOrder(ctx context.Context, id string) error {
	_, err := uc.GetOrder(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.UpdateStatus(ctx, id, models.OrderStatusCancelled)
}
