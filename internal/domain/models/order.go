package models

import "time"

type OrderStatus string

const (
    StatusPending    OrderStatus = "PENDING"
    StatusProcessing OrderStatus = "PROCESSING"
    StatusCompleted  OrderStatus = "COMPLETED"
    StatusCancelled  OrderStatus = "CANCELLED"
)

type Order struct {
    ID          string      `json:"id"`
    NodeID      string      `json:"node_id"`
    Status      OrderStatus `json:"status"`
    TotalAmount float64     `json:"total_amount"`
    Items       []OrderItem `json:"items"`
    Platform    string      `json:"platform"` // e.g., "GRAB", "SHOPEEFOOD", "DIRECT"
    CreatedAt   time.Time   `json:"created_at"`
}

type OrderItem struct {
    ItemID   string  `json:"item_id"`
    Quantity int     `json:"quantity"`
    Price    float64 `json:"price"`
}
