package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// InventoryService defines all operations that read or mutate NodeStock.
//
// Stock mutations fall into two categories:
//   - StockIn  (GoodsReceipt confirmed)          → qty_on_hand increases; no ROP check.
//   - StockOut (GoodsIssue or StockConsumption)  → qty_on_hand decreases; ROP check fired.
//
// This service is called by GIUseCase, GRUseCase, and the Production domain's
// StockConsumption writer. It never calls those usecases back — the facade wires
// the ROP result outward to ITOUseCase or POUseCase.
type InventoryService interface {
	// GetStock returns the current qty_on_hand for (nodeID, itemID) in base units.
	// Returns 0, nil when no NodeStock record exists for the pair.
	GetStock(ctx context.Context, nodeID, itemID string) (float64, error)

	// InitStock creates or overwrites a NodeStock record with a manual quantity.
	// Used during onboarding or periodic stock-take corrections.
	InitStock(ctx context.Context, nodeID, itemID string, qtyBU float64) (*ROPCheckResult, error)

	// StockIn increases qty_on_hand by qtyBU. Called when a GoodsReceipt is confirmed.
	// Does NOT trigger a ROP check (stock is rising, not falling).
	StockIn(ctx context.Context, nodeID, itemID string, qtyBU float64) error

	// StockOut decreases qty_on_hand by qtyBU. Called when a GoodsIssue is confirmed
	// or when a ProductionBatch writes a StockConsumption record.
	// After decrement, it runs a ROP check and returns the result so the caller
	// (via the SupplyChainFacade) can fire the appropriate replenishment document.
	StockOut(ctx context.Context, nodeID, itemID string, qtyBU float64) (*ROPCheckResult, error)

	// CheckROP evaluates whether qty_on_hand has breached the reorder point for (nodeID, itemID).
	// Returns the result without modifying any stock. Useful for explicit manual checks.
	CheckROP(ctx context.Context, nodeID, itemID string) (*ROPCheckResult, error)

	// GetConfig fetches the NodeItemConfig for a given node and item.
	// Used by the Facade to run Material Requirements Planning (MRP) and evaluate Sourcing Strategies.
	GetConfig(ctx context.Context, nodeID, itemID string) (*models.NodeItemConfig, error)
}

// ROPCheckResult is returned by StockOut and CheckROP to communicate the outcome
// of the reorder point evaluation. The SupplyChainFacade uses this to decide
// whether to create an ITO (INTERNAL_TRANSFER) or a draft PO (EXTERNAL_PROCUREMENT).
type ROPCheckResult struct {
	// Breached is true when qty_on_hand ≤ reorder_point after the stock event.
	Breached bool

	// CurrentQty is the qty_on_hand after the stock event (base units).
	CurrentQty float64

	// ReorderPoint is the configured threshold for this (node, item) pair (base units).
	ReorderPoint float64

	// Strategy is the sourcing strategy configured in NodeItemConfig.
	// Only meaningful when Breached == true.
	Strategy models.SourcingStrategy

	// NodeItemConfig is the full config record — carried so the facade can pass it
	// directly to ITOUseCase or POUseCase without a second DB lookup.
	// Only populated when Breached == true.
	Config *models.NodeItemConfig
}
