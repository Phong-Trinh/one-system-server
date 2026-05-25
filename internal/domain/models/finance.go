package models

import "time"

// ─── Supplier Invoice ─────────────────────────────────────────────────────────

// SupplierInvoiceStatus represents the 3-Way Matching state of a supplier invoice.
type SupplierInvoiceStatus string

const (
	// SupplierInvoicePending — invoice received from supplier; awaiting HQ matching.
	SupplierInvoicePending SupplierInvoiceStatus = "PENDING"
	// SupplierInvoiceMatched — all 3 documents (PO + Invoice + GR) reconcile; payment authorized.
	SupplierInvoiceMatched SupplierInvoiceStatus = "MATCHED"
	// SupplierInvoiceDisputed — discrepancy found between PO, Invoice, or GR; under investigation.
	SupplierInvoiceDisputed SupplierInvoiceStatus = "DISPUTED"
	// SupplierInvoicePaid — payment to supplier has been settled.
	SupplierInvoicePaid SupplierInvoiceStatus = "PAID"
)

// SupplierInvoice is a billing document received from a supplier, linked to a PurchaseOrder.
// HQ performs 3-Way Matching (PurchaseOrder + SupplierInvoice + GoodsReceipt) before
// authorizing supplier payment. This is the finance record for external procurement.
type SupplierInvoice struct {
	ID              string                `json:"id"`
	OrgID           string                `json:"org_id"`           // FK → Organization
	PurchaseOrderID string                `json:"purchase_order_id"` // FK → PurchaseOrder
	SupplierID      string                `json:"supplier_id"`       // FK → Supplier
	// GRID links the matching GoodsReceipt — set when the receiving node confirms delivery.
	// 3-Way Matching is possible once PurchaseOrderID + SupplierInvoice + GRID are all populated.
	GRID            *string               `json:"gr_id,omitempty"`   // FK → GoodsReceipt
	InvoiceNumber   string                `json:"invoice_number"`    // Supplier's reference number
	TotalAmount     float64               `json:"total_amount"`      // Invoice total (before tax)
	TaxAmount       float64               `json:"tax_amount"`
	ImageURL        string                `json:"image_url"`         // URL to scanned invoice document
	Status          SupplierInvoiceStatus `json:"status"`
	MatchedBy       *string               `json:"matched_by,omitempty"` // FK → Staff (HQ who approved matching)
	MatchedAt       *time.Time            `json:"matched_at,omitempty"`
	PaidAt          *time.Time            `json:"paid_at,omitempty"`
	InvoiceDate     time.Time             `json:"invoice_date"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

// SupplierInvoiceLine is a single item line within a SupplierInvoice.
// Used during 3-Way Matching to compare per-line quantities and prices against PurchaseOrderLines.
type SupplierInvoiceLine struct {
	ID          string  `json:"id"`
	InvoiceID   string  `json:"invoice_id"`   // FK → SupplierInvoice
	ItemID      *string `json:"item_id,omitempty"` // FK → Item (nil if line cannot be matched to a catalog item)
	RawLineText string  `json:"raw_line_text"` // Original line text from OCR or supplier document
	Qty         float64 `json:"qty"`
	UnitPrice   float64 `json:"unit_price"`
	LineTotal   float64 `json:"line_total"` // qty × unit_price
}

// ─── General Ledger Transaction ───────────────────────────────────────────────

// TransactionType classifies a general ledger entry.
type TransactionType string

const (
	TxIncome  TransactionType = "INCOME"
	TxExpense TransactionType = "EXPENSE"
)

// TransactionRefType identifies the source document type for a ledger entry.
type TransactionRefType string

const (
	TxRefOrder          TransactionRefType = "ORDER"            // Revenue from a customer Order
	TxRefSupplierInvoice TransactionRefType = "SUPPLIER_INVOICE" // Expense from a settled SupplierInvoice
	TxRefITO            TransactionRefType = "ITO"              // OpEx cost allocated on ITO receipt (transfer price + shipping)
	TxRefProductionOrder TransactionRefType = "PRODUCTION_ORDER" // OpEx cost from a completed ProductionOrder
	TxRefAsset          TransactionRefType = "ASSET"            // CapEx depreciation entry linked to an Asset
)

// Transaction is a general ledger entry recording a financial event at a node.
// Each entry is immutable — corrections use a new offsetting Transaction with a reason.
type Transaction struct {
	ID          string             `json:"id"`
	NodeID      string             `json:"node_id"`      // FK → Node (the node where the event occurred)
	OrgID       string             `json:"org_id"`       // FK → Organization
	Amount      float64            `json:"amount"`       // Always positive; direction determined by Type
	Type        TransactionType    `json:"type"`         // INCOME | EXPENSE
	RefType     TransactionRefType `json:"ref_type"`     // Source document category
	ReferenceID string             `json:"reference_id"` // FK to the source document (Order.ID, SupplierInvoice.ID, etc.)
	Description string             `json:"description"`  // Human-readable summary
	Timestamp   time.Time          `json:"timestamp"`
}
