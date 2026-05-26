package usecase

import (
	"context"
	"fmt"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// inventoryUseCase implements services.InventoryService.
// It is embedded inside SupplyChainFacade and should not be instantiated directly.
type inventoryUseCase struct {
	stockRepo  services.NodeStockRepository
	configRepo services.NodeItemConfigRepository
}

func newInventoryUseCase(
	stockRepo services.NodeStockRepository,
	configRepo services.NodeItemConfigRepository,
) services.InventoryService {
	return &inventoryUseCase{
		stockRepo:  stockRepo,
		configRepo: configRepo,
	}
}

// GetStock returns the current qty_on_hand for (nodeID, itemID) in base units.
// Returns 0 when no record exists yet.
func (uc *inventoryUseCase) GetStock(ctx context.Context, nodeID, itemID string) (float64, error) {
	stock, err := uc.stockRepo.Get(ctx, nodeID, itemID)
	if err != nil {
		return 0, fmt.Errorf("inventory: GetStock(%s, %s): %w", nodeID, itemID, err)
	}
	if stock == nil {
		return 0, nil
	}
	return stock.QtyOnHand, nil
}

// InitStock sets the qty_on_hand to a specific value (stock-take / onboarding).
// This does NOT trigger a ROP check; it is an intentional manual override.
func (uc *inventoryUseCase) InitStock(ctx context.Context, nodeID, itemID string, qtyBU float64) error {
	if qtyBU < 0 {
		return fmt.Errorf("inventory: InitStock: qty must be ≥ 0, got %v", qtyBU)
	}
	stock := &models.NodeStock{
		NodeID:        nodeID,
		ItemID:        itemID,
		QtyOnHand:     qtyBU,
		LastUpdatedAt: time.Now(),
	}
	return uc.stockRepo.Upsert(ctx, stock)
}

// StockIn increases qty_on_hand by qtyBU.
// Called when a GoodsReceipt is confirmed (from ITO or PurchaseOrder).
// No ROP check is fired — stock is rising, not falling.
func (uc *inventoryUseCase) StockIn(ctx context.Context, nodeID, itemID string, qtyBU float64) error {
	if qtyBU <= 0 {
		return fmt.Errorf("inventory: StockIn: qty must be > 0, got %v", qtyBU)
	}

	current, err := uc.stockRepo.Get(ctx, nodeID, itemID)
	if err != nil {
		return fmt.Errorf("inventory: StockIn: load stock: %w", err)
	}

	var newQty float64
	if current == nil {
		// First-ever stock event for this (node, item) pair — initialize from zero.
		newQty = qtyBU
	} else {
		newQty = current.QtyOnHand + qtyBU
	}

	return uc.stockRepo.Upsert(ctx, &models.NodeStock{
		NodeID:        nodeID,
		ItemID:        itemID,
		QtyOnHand:     newQty,
		LastUpdatedAt: time.Now(),
	})
}

// StockOut decreases qty_on_hand by qtyBU and fires a ROP check after the decrement.
// Called when:
//   - A GoodsIssue is confirmed (ITO dispatch or B2B shipment) — triggers Stock Out at provider.
//   - A ProductionBatch completes and writes a StockConsumption record.
//
// The returned ROPCheckResult tells the SupplyChainFacade whether to create an ITO or PO draft.
// The caller (facade) is responsible for acting on the result — this service never calls
// ITOUseCase or POUseCase directly, keeping dependency direction clean.
func (uc *inventoryUseCase) StockOut(ctx context.Context, nodeID, itemID string, qtyBU float64) (*services.ROPCheckResult, error) {
	if qtyBU <= 0 {
		return nil, fmt.Errorf("inventory: StockOut: qty must be > 0, got %v", qtyBU)
	}

	current, err := uc.stockRepo.Get(ctx, nodeID, itemID)
	if err != nil {
		return nil, fmt.Errorf("inventory: StockOut: load stock: %w", err)
	}

	var currentQty float64
	if current != nil {
		currentQty = current.QtyOnHand
	}

	if currentQty < qtyBU {
		return nil, fmt.Errorf("inventory: StockOut: insufficient stock at node %s for item %s: have %.4f, need %.4f",
			nodeID, itemID, currentQty, qtyBU)
	}

	newQty := currentQty - qtyBU

	if err := uc.stockRepo.Upsert(ctx, &models.NodeStock{
		NodeID:        nodeID,
		ItemID:        itemID,
		QtyOnHand:     newQty,
		LastUpdatedAt: time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("inventory: StockOut: persist: %w", err)
	}

	// ROP check runs after every stock-decreasing event.
	return uc.checkROPInternal(ctx, nodeID, itemID, newQty)
}

// CheckROP evaluates the ROP without changing any stock.
// Useful for manual dashboard triggers or on-demand checks.
func (uc *inventoryUseCase) CheckROP(ctx context.Context, nodeID, itemID string) (*services.ROPCheckResult, error) {
	qty, err := uc.GetStock(ctx, nodeID, itemID)
	if err != nil {
		return nil, err
	}
	return uc.checkROPInternal(ctx, nodeID, itemID, qty)
}

// checkROPInternal is the shared ROP evaluation logic.
// It compares currentQty against NodeItemConfig.ReorderPoint and returns the result.
// It never creates documents — that is the SupplyChainFacade's responsibility.
func (uc *inventoryUseCase) checkROPInternal(ctx context.Context, nodeID, itemID string, currentQty float64) (*services.ROPCheckResult, error) {
	cfg, err := uc.configRepo.Get(ctx, nodeID, itemID)
	if err != nil {
		return nil, fmt.Errorf("inventory: ROP check: load config(%s, %s): %w", nodeID, itemID, err)
	}

	// No config means this item has no ROP configured for this node — skip.
	if cfg == nil {
		return &services.ROPCheckResult{
			Breached:   false,
			CurrentQty: currentQty,
		}, nil
	}

	breached := currentQty <= cfg.ReorderPoint

	return &services.ROPCheckResult{
		Breached:     breached,
		CurrentQty:   currentQty,
		ReorderPoint: cfg.ReorderPoint,
		Strategy:     cfg.SourcingStrategy,
		Config:       cfg,
	}, nil
}
