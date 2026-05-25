package models

import "time"

// ─── Order ────────────────────────────────────────────────────────────────────

// OrderStatus represents the lifecycle state of a customer order.
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "PENDING"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusCompleted  OrderStatus = "COMPLETED"
	OrderStatusCancelled  OrderStatus = "CANCELLED"
)

// OrderSource identifies the channel through which the order was placed.
type OrderSource string

const (
	// OrderSourcePlatform — delivery platform order (e.g., Grab, ShopeeFood).
	// Staff scan the physical bill; AI OCR extracts items.
	OrderSourcePlatform OrderSource = "PLATFORM"
	// OrderSourceDirectPOS — order placed directly via the store's POS terminal.
	OrderSourceDirectPOS OrderSource = "DIRECT_POS"
)

// Order is a customer sales order at a Store node.
//
// Source:
//   PLATFORM  — Staff scans physical bill → AI OCR extracts items → system creates Order.
//   DIRECT_POS — POS terminal creates Order directly.
//
// Priority for Kitchen Display System (KDS) is derived from:
//   1. SLA Pressure — DeadlineAt proximity (platform delivery deadlines).
//   2. Batching — grouping identical items across concurrent orders for efficiency.
type Order struct {
	ID          string      `json:"id"`
	NodeID      string      `json:"node_id"`    // FK → Node (the Store handling this order)
	Source      OrderSource `json:"source"`     // PLATFORM | DIRECT_POS
	Platform    *string     `json:"platform,omitempty"` // e.g., "GRAB", "SHOPEEFOOD" (nil for DIRECT_POS)
	Status      OrderStatus `json:"status"`
	TotalAmount float64     `json:"total_amount"`
	Items       []OrderItem `json:"items"`
	// DeadlineAt is the customer-facing SLA deadline used for KDS priority scoring.
	// Set from delivery platform data for PLATFORM orders. Nil for DIRECT_POS orders.
	DeadlineAt  *time.Time  `json:"deadline_at,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// OrderItem is a single item line within an Order.
type OrderItem struct {
	ItemID   string  `json:"item_id"`  // FK → Item (must be PRODUCT type)
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`    // Unit selling price at time of order
}
