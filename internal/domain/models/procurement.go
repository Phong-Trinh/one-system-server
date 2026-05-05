package models

import "time"

type PurchaseStatus string

const (
    PurchasePending  PurchaseStatus = "PENDING"
    PurchaseApproved PurchaseStatus = "APPROVED"
    PurchaseRejected PurchaseStatus = "REJECTED"
)

type ExternalPurchase struct {
    ID           string         `json:"id"`
    NodeID       string         `json:"node_id"`
    VendorName   string         `json:"vendor_name"`
    TotalAmount  float64        `json:"total_amount"`
    Items        []PurchaseItem `json:"items"`
    InvoiceImage string         `json:"invoice_image"` // URL or path to scanned image
    Status       PurchaseStatus `json:"status"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
}

type PurchaseItem struct {
    ItemID    string  `json:"item_id"`   // Internal ID
    RawName   string  `json:"raw_name"`  // Original name from bill
    Quantity  float64 `json:"quantity"`
    UnitPrice float64 `json:"unit_price"`
}
