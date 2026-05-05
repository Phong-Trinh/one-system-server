package models

import "time"

type TransactionType string

const (
    TxIncome  TransactionType = "INCOME"
    TxExpense TransactionType = "EXPENSE"
)

type Transaction struct {
    ID          string          `json:"id"`
    NodeID      string          `json:"node_id"`
    Amount      float64         `json:"amount"`
    Type        TransactionType `json:"type"`
    Description string          `json:"description"`
    Source      string          `json:"source"` // e.g., "POS_SCAN", "UTILITY_BILL"
    ReferenceID string          `json:"reference_id"` // Link to Order or Invoice
    Timestamp   time.Time       `json:"timestamp"`
}

type Invoice struct {
    ID        string    `json:"id"`
    NodeID    string    `json:"node_id"`
    Vendor    string    `json:"vendor"`
    Amount    float64   `json:"amount"`
    Tax       float64   `json:"tax"`
    Date      time.Time `json:"date"`
    Items     []string  `json:"items"` // Raw line items from OCR
    ImageURL  string    `json:"image_url"`
    Status    string    `json:"status"` // e.g., "PENDING", "VERIFIED"
}
