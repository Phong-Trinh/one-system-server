package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// B2BLineInput is a single item line in a new B2B Sales Order.
type B2BLineInput struct {
	ItemID     string
	QtyOrdered float64 // Base units
	UnitPrice  float64 // Negotiated wholesale price per base unit
}

// B2BUseCase manages the B2B wholesale fulfillment lifecycle.
//
// Authority model:
//   - HQ creates the order and negotiates terms.
//   - Factory receives the assignment and executes fulfillment (GoodsIssue).
//   - No system GoodsReceipt — the order is COMPLETED via Proof of Delivery (external logistics).
//
// Stock effect:
//   - DispatchGoods triggers StockOut at the Factory via GIUseCase.
//   - ROP check fires after dispatch; the facade handles any replenishment needed.
type B2BUseCase interface {
	// CreateB2BOrder is called by HQ Sales to create a wholesale order and assign it to a Factory.
	CreateB2BOrder(ctx context.Context, orgID, hqNodeID, factoryNodeID, createdByStaffID, customerName, customerContact string, lines []B2BLineInput) (*models.B2BSalesOrder, error)

	// DispatchGoods is called by the Factory to confirm shipment to the external customer.
	// Creates and confirms a GoodsIssue, triggers StockOut at the Factory.
	// Returns the GoodsIssue and any ROP results for the facade to act on.
	DispatchGoods(ctx context.Context, b2bOrderID string, input GoodsIssueInput) (*models.GoodsIssue, []*services.ROPCheckResult, error)

	// ConfirmDelivery marks the B2B order COMPLETED via Proof of Delivery.
	// No system GR is created — the external customer is outside the system.
	ConfirmDelivery(ctx context.Context, b2bOrderID, proofOfDeliveryURL string) error

	GetB2BOrder(ctx context.Context, orderID string) (*models.B2BSalesOrder, []*models.B2BSalesOrderLine, error)
	ListByFactory(ctx context.Context, factoryNodeID string) ([]*models.B2BSalesOrder, error)
}

// b2bUseCase implements B2BUseCase.
type b2bUseCase struct {
	orderRepo services.B2BSalesOrderRepository
	lineRepo  services.B2BSalesOrderLineRepository
	giUC      GIUseCase
}

func newB2BUseCase(
	orderRepo services.B2BSalesOrderRepository,
	lineRepo services.B2BSalesOrderLineRepository,
	giUC GIUseCase,
) B2BUseCase {
	return &b2bUseCase{
		orderRepo: orderRepo,
		lineRepo:  lineRepo,
		giUC:      giUC,
	}
}

// CreateB2BOrder creates a new wholesale order in ASSIGNED status.
func (uc *b2bUseCase) CreateB2BOrder(ctx context.Context, orgID, hqNodeID, factoryNodeID, createdByStaffID, customerName, customerContact string, lines []B2BLineInput) (*models.B2BSalesOrder, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("b2b: CreateB2BOrder: at least one line is required")
	}
	for i, l := range lines {
		if l.QtyOrdered <= 0 {
			return nil, fmt.Errorf("b2b: CreateB2BOrder: line %d qty_ordered must be > 0", i)
		}
		if l.UnitPrice <= 0 {
			return nil, fmt.Errorf("b2b: CreateB2BOrder: line %d unit_price must be > 0", i)
		}
	}

	now := time.Now()
	order := &models.B2BSalesOrder{
		ID:              uuid.NewString(),
		OrgID:           orgID,
		HQNodeID:        hqNodeID,
		FactoryNodeID:   factoryNodeID,
		CustomerName:    customerName,
		CustomerContact: customerContact,
		Status:          models.B2BSalesAssigned,
		CreatedBy:       createdByStaffID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("b2b: CreateB2BOrder: persist: %w", err)
	}

	for i, l := range lines {
		line := &models.B2BSalesOrderLine{
			ID:         uuid.NewString(),
			OrderID:    order.ID,
			ItemID:     l.ItemID,
			QtyOrdered: l.QtyOrdered,
			UnitPrice:  l.UnitPrice,
		}
		if err := uc.lineRepo.AddLine(ctx, line); err != nil {
			return nil, fmt.Errorf("b2b: CreateB2BOrder: add line %d: %w", i, err)
		}
	}

	return order, nil
}

// DispatchGoods is called by the Factory to ship goods to the external customer.
// Delegates to GIUseCase which handles GI creation and StockOut.
func (uc *b2bUseCase) DispatchGoods(ctx context.Context, b2bOrderID string, input GoodsIssueInput) (*models.GoodsIssue, []*services.ROPCheckResult, error) {
	order, err := uc.loadOrder(ctx, b2bOrderID)
	if err != nil {
		return nil, nil, err
	}
	if order.Status != models.B2BSalesAssigned {
		return nil, nil, fmt.Errorf("b2b: DispatchGoods: order %s must be ASSIGNED (current: %s)", b2bOrderID, order.Status)
	}

	gi, ropResults, err := uc.giUC.ConfirmB2BGoodsIssue(ctx, b2bOrderID, order.FactoryNodeID, input)
	if err != nil {
		return nil, nil, fmt.Errorf("b2b: DispatchGoods: %w", err)
	}

	// Transition order to IN_TRANSIT.
	order.Status = models.B2BSalesInTransit
	order.UpdatedAt = time.Now()
	if err := uc.orderRepo.Update(ctx, order); err != nil {
		return nil, nil, fmt.Errorf("b2b: DispatchGoods: update order status: %w", err)
	}

	return gi, ropResults, nil
}

// ConfirmDelivery marks the B2B order COMPLETED with a Proof of Delivery URL.
// No system GR is created — the external customer is outside OneSystem.
func (uc *b2bUseCase) ConfirmDelivery(ctx context.Context, b2bOrderID, proofOfDeliveryURL string) error {
	if proofOfDeliveryURL == "" {
		return fmt.Errorf("b2b: ConfirmDelivery: proof_of_delivery_url is required")
	}
	order, err := uc.loadOrder(ctx, b2bOrderID)
	if err != nil {
		return err
	}
	if order.Status != models.B2BSalesInTransit {
		return fmt.Errorf("b2b: ConfirmDelivery: order %s must be IN_TRANSIT (current: %s)", b2bOrderID, order.Status)
	}

	order.Status = models.B2BSalesCompleted
	order.ProofOfDelivery = &proofOfDeliveryURL
	order.UpdatedAt = time.Now()
	return uc.orderRepo.Update(ctx, order)
}

func (uc *b2bUseCase) GetB2BOrder(ctx context.Context, orderID string) (*models.B2BSalesOrder, []*models.B2BSalesOrderLine, error) {
	order, err := uc.loadOrder(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	lines, err := uc.lineRepo.ListByOrder(ctx, orderID)
	return order, lines, err
}

func (uc *b2bUseCase) ListByFactory(ctx context.Context, factoryNodeID string) ([]*models.B2BSalesOrder, error) {
	return uc.orderRepo.FindByFactory(ctx, factoryNodeID)
}

func (uc *b2bUseCase) loadOrder(ctx context.Context, id string) (*models.B2BSalesOrder, error) {
	order, err := uc.orderRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("b2b: load order %s: %w", id, err)
	}
	if order == nil {
		return nil, fmt.Errorf("b2b: order %s not found", id)
	}
	return order, nil
}
