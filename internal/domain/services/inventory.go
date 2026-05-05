package services

import (
    "context"
    "one-system-server/internal/domain/models"
)

type InventoryService interface {
    // GetStock returns the current stock level for an item at a specific node
    GetStock(ctx context.Context, nodeID string, itemID string) (float64, error)
    
    // UpdateStock adjusts stock levels (positive for addition, negative for deduction)
    UpdateStock(ctx context.Context, nodeID string, itemID string, quantity float64, reason string) error
    
    // TransferItems handles the movement of goods between nodes (execution)
    TransferItems(ctx context.Context, request TransferRequest) error

    // CreateSupplyRequest initiates a request for items from another node
    CreateSupplyRequest(ctx context.Context, req models.SupplyRequest) error
    
    // ApproveSupplyRequest marks a request as approved
    ApproveSupplyRequest(ctx context.Context, requestID string) error
    
    // FulfillRequest marks the items as received at the destination
    FulfillRequest(ctx context.Context, requestID string) error
}

type TransferRequest struct {
    FromNodeID string
    ToNodeID   string
    Items      []TransferItem
}

type TransferItem struct {
    ItemID   string
    Quantity float64
}

// Basic implementation of Inventory Logic
type inventoryServiceImpl struct {
    // repo InventoryRepository (to be implemented)
}

func (s *inventoryServiceImpl) TransferItems(ctx context.Context, req TransferRequest) error {
    // 1. Verify stock at FromNode
    // 2. Deduct stock from FromNode
    // 3. Add stock to ToNode
    // 4. Record the transfer in history
    return nil
}
