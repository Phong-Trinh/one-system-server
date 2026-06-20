package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

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
		if uc.facade.IsProducedLocally(ctx, nodeID, it.ItemID) {
			_, bomLines, _ := uc.facade.GetBOMByItem(ctx, it.ItemID)
			for _, line := range bomLines {
				ingQty := qtyBU * line.Qty
				avail, err := uc.invSvc.GetStock(ctx, nodeID, line.ItemID)
				if err != nil || avail < ingQty {
					// [NEW] Trigger ROP check manually so the system tries to replenish it!
					_ = uc.facade.TriggerROPCheck(ctx, "SNAPBITE_ORG", "HQ", nodeID, line.ItemID)
					return nil, fmt.Errorf("insufficient stock for ingredient %s: have %.2f, need %.2f", line.ItemID, avail, ingQty)
				}
			}
		} else {
			avail, err := uc.invSvc.GetStock(ctx, nodeID, it.ItemID)
			if err != nil || avail < qtyBU {
				_ = uc.facade.TriggerROPCheck(ctx, "SNAPBITE_ORG", "HQ", nodeID, it.ItemID)
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

	// [NEW] Dispatch to Kitchen for locally produced items
	for _, it := range items {
		if err := uc.facade.DispatchToKitchen(ctx, nodeID, it.ItemID, float64(it.Quantity), order.ID); err != nil {
			log.Error().Err(err).Str("order_id", order.ID).Str("item_id", it.ItemID).Msg("failed to dispatch to kitchen")
		}
	}

	return order, nil
}

func (uc *orderUseCase) GetOrder(ctx context.Context, id string) (*models.Order, error) {
	order, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("order: get: %w", err)
	}
	if order != nil {
		_ = uc.facade.GetProductionStatusForOrders(ctx, []*models.Order{order})
	}
	return order, nil
}

func (uc *orderUseCase) ListByNode(ctx context.Context, nodeID string) ([]*models.Order, error) {
	orders, err := uc.repo.FindByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	_ = uc.facade.GetProductionStatusForOrders(ctx, orders)
	return orders, nil
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

	// Deduct stock and fire ROP for each item line (only for items not produced locally).
	for _, item := range order.Items {
		qtyBU := float64(item.Quantity)

		if uc.facade.IsProducedLocally(ctx, order.NodeID, item.ItemID) {
			// Do NOT backflush ingredients. The ingredients were automatically deducted
			// by the allocation_engine when the kitchen staff started the batch.
			continue
		}

		// Direct stock deduction for items not produced locally (e.g., Coca, semi-products)
		if err := uc.facade.StockOutWithROP(ctx, orgID, hqNodeID, order.NodeID, item.ItemID, qtyBU); err != nil {
			return fmt.Errorf("order: CompleteOrder: StockOutWithROP item %s: %w", item.ItemID, err)
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
