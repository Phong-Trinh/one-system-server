package services

import (
	"context"

	"one-system-server/internal/domain/models"
)

// ── §2.4 NodeStock ────────────────────────────────────────────────────────────

// NodeStockRepository defines persistence for the live inventory ledger.
// Every (item, node) pair has exactly one NodeStock record.
type NodeStockRepository interface {
	// Get returns the current stock record for (nodeID, itemID).
	// Returns nil, nil when no stock record exists yet.
	Get(ctx context.Context, nodeID, itemID string) (*models.NodeStock, error)

	// Upsert creates or atomically overwrites the NodeStock record.
	// Used by StockIn, StockOut, and InitStock operations.
	Upsert(ctx context.Context, stock *models.NodeStock) error

	// ListByNode returns all NodeStock records for a given node.
	ListByNode(ctx context.Context, nodeID string) ([]*models.NodeStock, error)
}

// ── §2.1 NodeItemConfig ───────────────────────────────────────────────────────

// NodeItemConfigRepository defines persistence for per-item, per-node ROP configuration.
type NodeItemConfigRepository interface {
	// Get returns the config for (nodeID, itemID).
	// Returns nil, nil when no config exists.
	Get(ctx context.Context, nodeID, itemID string) (*models.NodeItemConfig, error)

	// Upsert creates or updates the NodeItemConfig record.
	Upsert(ctx context.Context, cfg *models.NodeItemConfig) error

	// ListByNode returns all NodeItemConfig records for a given node.
	ListByNode(ctx context.Context, nodeID string) ([]*models.NodeItemConfig, error)
}

// ── Supplier ──────────────────────────────────────────────────────────────────

// SupplierRepository defines persistence for third-party vendors.
type SupplierRepository interface {
	Create(ctx context.Context, s *models.Supplier) error
	FindByID(ctx context.Context, id string) (*models.Supplier, error)
	FindByOrg(ctx context.Context, orgID string) ([]*models.Supplier, error)
	// FindByName does a case-insensitive lookup within an org. Used during supplier-check flow.
	FindByName(ctx context.Context, orgID, name string) (*models.Supplier, error)
	Update(ctx context.Context, s *models.Supplier) error
	Delete(ctx context.Context, id string) error
}

// ── §1.1 Internal Transfer Order ─────────────────────────────────────────────

// InternalTransferOrderRepository defines persistence for ITOs.
type InternalTransferOrderRepository interface {
	Create(ctx context.Context, ito *models.InternalTransferOrder) error
	FindByID(ctx context.Context, id string) (*models.InternalTransferOrder, error)
	// FindByNode returns all ITOs where the node is either requester or provider.
	FindByNode(ctx context.Context, nodeID string) ([]*models.InternalTransferOrder, error)
	UpdateStatus(ctx context.Context, id string, status models.ITOStatus) error
	Update(ctx context.Context, ito *models.InternalTransferOrder) error
	Delete(ctx context.Context, id string) error
}

// ITOLineRepository defines persistence for ITO line items.
type ITOLineRepository interface {
	AddLine(ctx context.Context, line *models.ITOLine) error
	ListByITO(ctx context.Context, itoID string) ([]*models.ITOLine, error)
	UpdateLine(ctx context.Context, line *models.ITOLine) error
	DeleteLine(ctx context.Context, id string) error
}

// ── §1.2 Purchase Requisition ────────────────────────────────────────────────

// PurchaseRequisitionRepository defines persistence for PRs.
type PurchaseRequisitionRepository interface {
	Create(ctx context.Context, pr *models.PurchaseRequisition) error
	FindByID(ctx context.Context, id string) (*models.PurchaseRequisition, error)
	// FindByNode returns all PRs submitted from a given requester node.
	FindByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error)
	// FindPendingByOrg returns all PRs with PENDING_HQ_APPROVAL status for HQ dashboard.
	FindPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error)
	Update(ctx context.Context, pr *models.PurchaseRequisition) error
	Delete(ctx context.Context, id string) error
}

// PRLineRepository defines persistence for PR line items.
type PRLineRepository interface {
	AddLine(ctx context.Context, line *models.PRLine) error
	ListByPR(ctx context.Context, prID string) ([]*models.PRLine, error)
	DeleteLine(ctx context.Context, id string) error
}

// ── §1.3 Purchase Order ───────────────────────────────────────────────────────

// PurchaseOrderRepository defines persistence for external procurement orders.
type PurchaseOrderRepository interface {
	Create(ctx context.Context, po *models.PurchaseOrder) error
	FindByID(ctx context.Context, id string) (*models.PurchaseOrder, error)
	// FindByStatus returns POs filtered by status — used by HQ dashboard (e.g., list all DRAFTs).
	FindByStatus(ctx context.Context, orgID string, status models.PurchaseOrderStatus) ([]*models.PurchaseOrder, error)
	// FindByDeliveryNode returns all POs targeting a specific destination node.
	FindByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error)
	Update(ctx context.Context, po *models.PurchaseOrder) error
	Delete(ctx context.Context, id string) error
}

// PurchaseOrderLineRepository defines persistence for PO line items.
type PurchaseOrderLineRepository interface {
	AddLine(ctx context.Context, line *models.PurchaseOrderLine) error
	ListByPurO(ctx context.Context, purOID string) ([]*models.PurchaseOrderLine, error)
	DeleteLine(ctx context.Context, id string) error
}

// ── Goods Issue ───────────────────────────────────────────────────────────────

// GoodsIssueRepository defines persistence for GI documents.
type GoodsIssueRepository interface {
	Create(ctx context.Context, gi *models.GoodsIssue) error
	FindByID(ctx context.Context, id string) (*models.GoodsIssue, error)
	// FindByRef returns GIs linked to a specific source document (ITO or B2B order).
	FindByRef(ctx context.Context, refType models.GoodsIssueRefType, refID string) ([]*models.GoodsIssue, error)
	UpdateStatus(ctx context.Context, id string, status models.GoodsIssueStatus) error
	Update(ctx context.Context, gi *models.GoodsIssue) error
}

// GoodsIssueLineRepository defines persistence for GI line items.
type GoodsIssueLineRepository interface {
	AddLine(ctx context.Context, line *models.GoodsIssueLine) error
	ListByGI(ctx context.Context, giID string) ([]*models.GoodsIssueLine, error)
}

// ── Goods Receipt ─────────────────────────────────────────────────────────────

// GoodsReceiptRepository defines persistence for GR documents.
type GoodsReceiptRepository interface {
	Create(ctx context.Context, gr *models.GoodsReceipt) error
	FindByID(ctx context.Context, id string) (*models.GoodsReceipt, error)
	// FindByRef returns GRs linked to a source document (ITO or PurchaseOrder).
	FindByRef(ctx context.Context, refType models.GoodsReceiptRefType, refID string) ([]*models.GoodsReceipt, error)
	UpdateStatus(ctx context.Context, id string, status models.GoodsReceiptStatus) error
	Update(ctx context.Context, gr *models.GoodsReceipt) error
}

// GoodsReceiptLineRepository defines persistence for GR line items.
type GoodsReceiptLineRepository interface {
	AddLine(ctx context.Context, line *models.GoodsReceiptLine) error
	ListByGR(ctx context.Context, grID string) ([]*models.GoodsReceiptLine, error)
}

// ── Discrepancy Ticket ────────────────────────────────────────────────────────

// DiscrepancyTicketRepository defines persistence for discrepancy tickets.
type DiscrepancyTicketRepository interface {
	Create(ctx context.Context, dt *models.DiscrepancyTicket) error
	FindByID(ctx context.Context, id string) (*models.DiscrepancyTicket, error)
	// FindByGR returns all discrepancy tickets associated with a GoodsReceipt.
	FindByGR(ctx context.Context, grID string) ([]*models.DiscrepancyTicket, error)
	UpdateStatus(ctx context.Context, id string, status models.DiscrepancyTicketStatus, resolution *string, resolvedBy *string) error
	Update(ctx context.Context, dt *models.DiscrepancyTicket) error
}

// ── Supplier Invoice ──────────────────────────────────────────────────────────

// SupplierInvoiceRepository defines persistence for supplier invoices.
type SupplierInvoiceRepository interface {
	Create(ctx context.Context, inv *models.SupplierInvoice) error
	FindByID(ctx context.Context, id string) (*models.SupplierInvoice, error)
	// FindByPurO returns all invoices linked to a PurchaseOrder.
	FindByPurO(ctx context.Context, purOID string) ([]*models.SupplierInvoice, error)
	Update(ctx context.Context, inv *models.SupplierInvoice) error
}

// SupplierInvoiceLineRepository defines persistence for invoice line items.
type SupplierInvoiceLineRepository interface {
	AddLine(ctx context.Context, line *models.SupplierInvoiceLine) error
	ListByInvoice(ctx context.Context, invoiceID string) ([]*models.SupplierInvoiceLine, error)
}

// ── General Ledger ────────────────────────────────────────────────────────────

// TransactionRepository defines persistence for general ledger entries.
// Entries are immutable — corrections are new offsetting transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx *models.Transaction) error
	FindByID(ctx context.Context, id string) (*models.Transaction, error)
	// ListByNode returns all ledger entries for a node, optionally filtered by type.
	ListByNode(ctx context.Context, nodeID string, txType *models.TransactionType) ([]*models.Transaction, error)
	// ListByRef returns ledger entries linked to a specific source document.
	ListByRef(ctx context.Context, refType models.TransactionRefType, refID string) ([]*models.Transaction, error)
}

// ── B2B Sales Order ───────────────────────────────────────────────────────────

// B2BSalesOrderRepository defines persistence for wholesale fulfillment orders.
type B2BSalesOrderRepository interface {
	Create(ctx context.Context, order *models.B2BSalesOrder) error
	FindByID(ctx context.Context, id string) (*models.B2BSalesOrder, error)
	// FindByFactory returns all B2B orders assigned to a factory node.
	FindByFactory(ctx context.Context, factoryNodeID string) ([]*models.B2BSalesOrder, error)
	Update(ctx context.Context, order *models.B2BSalesOrder) error
}

// B2BSalesOrderLineRepository defines persistence for B2B order line items.
type B2BSalesOrderLineRepository interface {
	AddLine(ctx context.Context, line *models.B2BSalesOrderLine) error
	ListByOrder(ctx context.Context, orderID string) ([]*models.B2BSalesOrderLine, error)
}

// ── Asset ─────────────────────────────────────────────────────────────────────

// AssetRepository defines persistence for CapEx asset records.
// Assets are auto-created by the system when a PR_TRIGGERED PO is payment-settled.
type AssetRepository interface {
	Create(ctx context.Context, asset *models.Asset) error
	FindByID(ctx context.Context, id string) (*models.Asset, error)
	// FindByNode returns all assets physically located at a node.
	FindByNode(ctx context.Context, nodeID string) ([]*models.Asset, error)
	// FindByPurO returns the asset linked to a specific PurchaseOrder (at most 1 for PR_TRIGGERED POs).
	FindByPurO(ctx context.Context, purOID string) (*models.Asset, error)
	Update(ctx context.Context, asset *models.Asset) error
}
