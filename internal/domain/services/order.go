package services

import (
    "context"
    "one-system-server/internal/domain/models"
)

type OrderService interface {
    // CreateOrder creates a new order, potentially from an AI-scanned bill
    CreateOrder(ctx context.Context, order models.Order) error
    
    // GetOrderDetails retrieves order with its production status
    GetOrderDetails(ctx context.Context, orderID string) (models.Order, error)
    
    // UpdateOrderStatus handles the state transition of an order
    UpdateOrderStatus(ctx context.Context, orderID string, status models.OrderStatus) error
}

type ProductionService interface {
    // AssignToKitchen pushes order items into the production queue
    AssignToKitchen(ctx context.Context, orderID string) error
    
    // GetKitchenQueue returns the prioritized list of items to be cooked
    GetKitchenQueue(ctx context.Context, nodeID string) ([]ProductionTask, error)
    
    // CompleteTask marks an item as cooked and triggers assembly
    CompleteTask(ctx context.Context, taskID string) error
}

type ProductionTask struct {
    ID        string
    OrderID   string
    ItemID    string
    Quantity  int
    Priority  int
    Status    string // "PENDING", "COOKING", "DONE"
}
