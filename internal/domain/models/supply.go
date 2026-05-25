package models

import "time"

// ─── Sourcing Strategy ────────────────────────────────────────────────────────

// SourcingStrategy determines which replenishment document is auto-generated
// when a node's item stock hits its Reorder Point (ROP).
// Configured per item per node in NodeItemConfig.
type SourcingStrategy string

const (
	// SourcingInternalTransfer — system auto-creates an InternalTransferOrder
	// to the configured provider node (another Factory or Store).
	SourcingInternalTransfer SourcingStrategy = "INTERNAL_TRANSFER"
	// SourcingExternalProcurement — system auto-generates a Draft PurchaseOrder
	// on the HQ Dashboard with delivery_to = the triggering node.
	SourcingExternalProcurement SourcingStrategy = "EXTERNAL_PROCUREMENT"
)

// ─── §1.1 Internal Transfer Order (ITO) ──────────────────────────────────────

// ITOStatus represents the lifecycle of an InternalTransferOrder.
type ITOStatus string

const (
	// ITOPendingApproval — manual ITO awaiting review by Factory/Area Manager
	// (to prevent stock hoarding). Configured per node.
	ITOPendingApproval ITOStatus = "PENDING_APPROVAL"
	// ITOAutoApproved — system-triggered ITO, approved automatically per policy.
	ITOAutoApproved ITOStatus = "AUTO_APPROVED"
	// ITOGoodsIssued — provider has confirmed the GoodsIssue; stock is now out at the provider.
	ITOGoodsIssued ITOStatus = "GOODS_ISSUED"
	// ITOInTransit — goods are moving from provider to requester (cross-site transfers only).
	ITOInTransit ITOStatus = "IN_TRANSIT"
	// ITOCompleted — requester confirmed GoodsReceipt; stock is now in at the requester.
	ITOCompleted ITOStatus = "COMPLETED"
	// ITOCancelled — order was cancelled before fulfillment.
	ITOCancelled ITOStatus = "CANCELLED"
)

// ITOTrigger identifies how the ITO was initiated.
type ITOTrigger string

const (
	// ITOTriggerROP — system-initiated because NodeStock.qty_on_hand dropped below ROP.
	ITOTriggerROP ITOTrigger = "ROP_AUTOMATIC"
	// ITOTriggerManual — manually initiated by a Store Manager (e.g., anticipating high demand).
	ITOTriggerManual ITOTrigger = "MANUAL"
)

// InternalTransferOrder manages recurring internal stock replenishment between nodes.
// No external suppliers are involved; goods move from Provider to Requester within the system.
//
// For cross-site transfers, the lifecycle includes an IN_TRANSIT phase with GoodsIssue + GoodsReceipt.
// For same-site transfers (RequesterNode.SiteID == ProviderNode.SiteID), the system uses a
// 1-click path that auto-generates both GI and GR simultaneously (no IN_TRANSIT phase).
type InternalTransferOrder struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`            // FK → Organization
	RequesterNodeID string     `json:"requester_node_id"` // FK → Node (the node requesting stock)
	ProviderNodeID  string     `json:"provider_node_id"`  // FK → Node (Factory or other Store providing stock)
	Trigger         ITOTrigger `json:"trigger"`           // ROP_AUTOMATIC | MANUAL
	Status          ITOStatus  `json:"status"`
	// IsSameSite — true when requester and provider share the same SiteID.
	// Drives the 1-click in-site transfer path (auto GI + GR, no IN_TRANSIT).
	IsSameSite  bool      `json:"is_same_site"`
	RequestedBy *string   `json:"requested_by,omitempty"` // FK → Staff (nil for ROP-triggered)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ITOLine is a single item line within an InternalTransferOrder.
// Quantities are tracked in both packaging units (for staff UX) and base units (for stock).
type ITOLine struct {
	ID         string  `json:"id"`
	ITOID      string  `json:"ito_id"`      // FK → InternalTransferOrder
	ItemID     string  `json:"item_id"`     // FK → Item
	QtyOrdered float64 `json:"qty_ordered"` // Ordered quantity in packaging units
	PkgUnit    string  `json:"pkg_unit"`    // Packaging unit name (e.g., "bag", "case")
	Conversion float64 `json:"conversion"`  // Base units per pkg_unit at time of order
	// QtyOrderedBU = QtyOrdered × Conversion — computed base unit quantity
	QtyOrderedBU float64 `json:"qty_ordered_bu"`
	// QtyReceived is filled in by the GoodsReceipt. May differ from QtyOrdered due to transit damage.
	QtyReceived   *float64 `json:"qty_received,omitempty"`    // Received quantity in packaging units
	QtyReceivedBU *float64 `json:"qty_received_bu,omitempty"` // Received base units (what actually enters stock)
}

// ─── §1.2 Purchase Requisition (PR) ──────────────────────────────────────────

// PRStatus represents the approval lifecycle of a PurchaseRequisition.
type PRStatus string

const (
	// PRDraft — requisition is being filled out by the Store/Factory Manager.
	PRDraft PRStatus = "DRAFT"
	// PRPendingHQApproval — submitted to HQ for review.
	PRPendingHQApproval PRStatus = "PENDING_HQ_APPROVAL"
	// PRApproved — HQ approved the request; HQ will create a PurchaseOrder.
	PRApproved PRStatus = "APPROVED"
	// PRRejected — HQ rejected the request.
	PRRejected PRStatus = "REJECTED"
	// PRConvertedToPO — HQ has issued a PurchaseOrder from this PR.
	PRConvertedToPO PRStatus = "CONVERTED_TO_PO"
)

// PurchaseRequisition is a formal request submitted by a Store or Factory Manager to HQ
// for CapEx assets (equipment, tools) or exceptional one-off purchases outside routine replenishment.
//
// Rules:
//   - Always manual; never triggered by ROP.
//   - Scope: CapEx items only (assets subject to depreciation, e.g., grills, POS systems).
//   - Routine OpEx replenishment (consumables, raw materials) bypasses PR entirely — handled by ROP.
//   - A PR is only a request; it does not directly trigger supplier shipment or stock changes.
//   - If approved, HQ converts it to a PurchaseOrder with delivery_to = requesting node.
//   - Goods never transit through HQ.
//   - PR uses PRLine entries that can specify either OpEx ItemID or CapEx EquipmentTypeID.
type PurchaseRequisition struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`             // FK → Organization
	RequesterNodeID string     `json:"requester_node_id"`  // FK → Node (Store or Factory)
	RequesterStaff  string     `json:"requester_staff_id"` // FK → Staff (manager who submitted)
	Status          PRStatus   `json:"status"`
	Justification   string     `json:"justification"`         // Overall business justification
	ReviewedBy      *string    `json:"reviewed_by,omitempty"` // FK → Staff (HQ reviewer)
	ReviewNote      *string    `json:"review_note,omitempty"` // HQ approval/rejection note
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// PRLine is a single item line within a PurchaseRequisition.
type PRLine struct {
	ID                 string  `json:"id"`
	PRID               string  `json:"pr_id"`             // FK → PurchaseRequisition
	ItemID             *string `json:"item_id,omitempty"` // FK → Item (populated for OpEx exceptional requests)
	EquipmentTypeID    *string `json:"equipment_type_id,omitempty"` // FK → EquipmentType (populated for CapEx equipment/assets)
	Qty                float64 `json:"qty"`
	UnitOfMeasure      string  `json:"unit_of_measure"`      // e.g., "unit", "set"
	EstimatedUnitPrice float64 `json:"estimated_unit_price"` // Requester's cost estimate
	Justification      string  `json:"justification"`        // Per-line rationale
}

// ─── §1.3 Purchase Order (HQ.PurO) ───────────────────────────────────────────

// PurchaseOrderStatus represents the lifecycle of an external procurement order.
type PurchaseOrderStatus string

const (
	// PurchaseOrderDraft — auto-generated by ROP engine; awaiting HQ review and confirmation.
	PurchaseOrderDraft PurchaseOrderStatus = "DRAFT"
	// PurchaseOrderConfirmed — HQ confirmed and sent to supplier.
	PurchaseOrderConfirmed PurchaseOrderStatus = "CONFIRMED"
	// PurchaseOrderShipped — supplier has dispatched goods to the destination node.
	PurchaseOrderShipped PurchaseOrderStatus = "SHIPPED"
	// PurchaseOrderCompleted — GoodsReceipt confirmed + 3-Way Matching done; supplier payment authorized.
	PurchaseOrderCompleted PurchaseOrderStatus = "COMPLETED"
	// PurchaseOrderCancelled — order was cancelled.
	PurchaseOrderCancelled PurchaseOrderStatus = "CANCELLED"
)

// POTriggerType identifies how the PurchaseOrder was initiated.
type POTriggerType string

const (
	// POTriggerPR — HQ converted an approved PurchaseRequisition into this PO.
	POTriggerPR POTriggerType = "PR_TRIGGERED"
	// POTriggerAutoDraft — system auto-generated this PO when a node with
	// sourcing_strategy=EXTERNAL_PROCUREMENT hit its ROP.
	POTriggerAutoDraft POTriggerType = "AUTO_DRAFT"
)

// PurchaseOrder is an external procurement order issued exclusively by HQ to a third-party supplier.
// Goods are delivered directly to the destination node (Factory or Store) — HQ holds no inventory.
//
// Two trigger paths:
//
//	PR_TRIGGERED: HQ converts an approved PurchaseRequisition (CapEx) → PurchaseOrder.
//	AUTO_DRAFT:   ROP engine detects a node with EXTERNAL_PROCUREMENT strategy hitting its
//	              reorder point → system creates a Draft PO on the HQ dashboard.
//	              HQ reviews and confirms.
//
// Logistics: Supplier → DeliveryToNode (direct delivery, no HQ transit).
// 3-Way Matching: HQ validates PO + SupplierInvoice + GoodsReceipt before authorizing payment.
type PurchaseOrder struct {
	ID               string              `json:"id"`
	OrgID            string              `json:"org_id"`              // FK → Organization
	TriggerType      POTriggerType       `json:"trigger_type"`        // PR_TRIGGERED | AUTO_DRAFT
	PRID             *string             `json:"pr_id,omitempty"`     // FK → PurchaseRequisition (nil for AUTO_DRAFT)
	HQNodeID         string              `json:"hq_node_id"`          // FK → Node (HQ — the issuing authority)
	SupplierID       string              `json:"supplier_id"`         // FK → Supplier
	DeliveryToNodeID string              `json:"delivery_to_node_id"` // FK → Node (Factory or Store receiving the goods)
	Status           PurchaseOrderStatus `json:"status"`
	ConfirmedBy      *string             `json:"confirmed_by,omitempty"` // FK → Staff (HQ staff who confirmed)
	ConfirmedAt      *time.Time          `json:"confirmed_at,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

// PurchaseOrderLine is a single item line within a PurchaseOrder.
// Quantities are in packaging units (ordered with supplier); converted to base units on GR.
type PurchaseOrderLine struct {
	ID         string  `json:"id"`
	POID       string  `json:"po_id"`       // FK → PurchaseOrder
	ItemID     *string `json:"item_id,omitempty"` // FK → Item (nil for CapEx PR_TRIGGERED)
	EquipmentTypeID *string `json:"equipment_type_id,omitempty"` // FK → EquipmentType (used for CapEx when ItemID is nil)
	QtyOrdered float64 `json:"qty_ordered"` // Quantity in packaging units
	PkgUnit    string  `json:"pkg_unit"`    // e.g., "case", "pallet"
	Conversion float64 `json:"conversion"`  // Base units per pkg_unit at time of order
	UnitPrice  float64 `json:"unit_price"`  // Price per packaging unit (from supplier quote)
}

// ─── Supplier ─────────────────────────────────────────────────────────────────

// Supplier represents a third-party vendor from whom HQ procures goods.
type Supplier struct {
	ID          string `json:"id"`
	OrgID       string `json:"org_id"` // FK → Organization
	Name        string `json:"name"`
	ContactInfo string `json:"contact_info"` // Phone, email, or structured contact
	Address     string `json:"address"`
}

// ─── Goods Issue (GI) ─────────────────────────────────────────────────────────

// GoodsIssueStatus represents the lifecycle of a GoodsIssue document.
type GoodsIssueStatus string

const (
	GoodsIssueDraft     GoodsIssueStatus = "DRAFT"
	GoodsIssueConfirmed GoodsIssueStatus = "CONFIRMED" // Stock Out event fired
	GoodsIssueVoided    GoodsIssueStatus = "VOIDED"
)

// GoodsIssueRefType identifies which document type spawned this GoodsIssue.
type GoodsIssueRefType string

const (
	GoodsIssueRefITO GoodsIssueRefType = "ITO" // Internal Transfer Order dispatch
	GoodsIssueRefB2B GoodsIssueRefType = "B2B" // B2B Sales Order fulfillment shipment
)

// GoodsIssue is created by the provider node when dispatching goods.
// Confirming a GoodsIssue triggers a Stock Out event at the issuing node.
//
// For cross-site ITO: provider packs items, logs driver info and media evidence.
// For same-site ITO: system auto-generates a simplified GI (no driver, no media, zero shipping fee).
// For B2B Sales Orders: Factory creates GI to ship to the external wholesale customer.
type GoodsIssue struct {
	ID            string            `json:"id"`
	RefType       GoodsIssueRefType `json:"ref_type"`        // ITO | B2B
	RefID         string            `json:"ref_id"`          // FK → InternalTransferOrder.ID or B2BSalesOrder.ID
	IssuingNodeID string            `json:"issuing_node_id"` // FK → Node (the provider dispatching goods)
	// Driver information — required for cross-site transfers; empty for same-site transfers.
	DriverName   string           `json:"driver_name,omitempty"`
	DriverPhone  string           `json:"driver_phone,omitempty"`
	VehiclePlate string           `json:"vehicle_plate,omitempty"`
	MediaURL     string           `json:"media_url,omitempty"` // Photo/video evidence of packing
	ShippingFee  float64          `json:"shipping_fee"`        // Zero for same-site transfers
	Status       GoodsIssueStatus `json:"status"`
	IssuedAt     *time.Time       `json:"issued_at,omitempty"` // When status → CONFIRMED
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// GoodsIssueLine is a single item line within a GoodsIssue.
// Quantity in base units — this is what decrements NodeStock.qty_on_hand at the issuing node.
type GoodsIssueLine struct {
	ID        string  `json:"id"`
	GIID      string  `json:"gi_id"`      // FK → GoodsIssue
	ItemID    string  `json:"item_id"`    // FK → Item
	QtyIssued float64 `json:"qty_issued"` // Quantity dispatched in base units
}

// ─── Goods Receipt (GR) ───────────────────────────────────────────────────────

// GoodsReceiptStatus represents the lifecycle of a GoodsReceipt document.
type GoodsReceiptStatus string

const (
	GoodsReceiptDraft       GoodsReceiptStatus = "DRAFT"
	GoodsReceiptConfirmed   GoodsReceiptStatus = "CONFIRMED"   // Stock In event fired
	GoodsReceiptDiscrepancy GoodsReceiptStatus = "DISCREPANCY" // Quantity mismatch; DiscrepancyTicket auto-created
)

// GoodsReceiptRefType identifies which document type spawned this GoodsReceipt.
type GoodsReceiptRefType string

const (
	GoodsReceiptRefITO  GoodsReceiptRefType = "ITO"  // Receiving from InternalTransferOrder
	GoodsReceiptRefPurO GoodsReceiptRefType = "PURO" // Receiving from PurchaseOrder (external supplier)
)

// GoodsReceipt is created by the receiving node when goods arrive.
// Confirming a GoodsReceipt triggers a Stock In event at the receiving node.
//
// If the received quantity differs from the expected quantity (transit damage or loss),
// the system automatically creates a DiscrepancyTicket for HQ resolution.
// The GR status is set to DISCREPANCY and only the usable received quantity enters stock.
type GoodsReceipt struct {
	ID              string              `json:"id"`
	RefType         GoodsReceiptRefType `json:"ref_type"`          // ITO | PURO
	RefID           string              `json:"ref_id"`            // FK → ITO.ID or PurchaseOrder.ID
	ReceivingNodeID string              `json:"receiving_node_id"` // FK → Node
	Status          GoodsReceiptStatus  `json:"status"`
	ReceivedBy      string              `json:"received_by"` // FK → Staff
	ReceivedAt      *time.Time          `json:"received_at,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// GoodsReceiptLine is a single item line within a GoodsReceipt.
// QtyReceived may be less than QtyExpected if goods were damaged or lost in transit.
type GoodsReceiptLine struct {
	ID          string  `json:"id"`
	GRID        string  `json:"gr_id"`        // FK → GoodsReceipt
	ItemID      string  `json:"item_id"`      // FK → Item
	QtyExpected float64 `json:"qty_expected"` // Expected in base units (from GI or PO line)
	QtyReceived float64 `json:"qty_received"` // Actually received in base units (enters NodeStock)
}

// ─── Discrepancy Ticket ───────────────────────────────────────────────────────

// DiscrepancyTicketStatus represents the resolution state of a DiscrepancyTicket.
type DiscrepancyTicketStatus string

const (
	DiscrepancyOpen     DiscrepancyTicketStatus = "OPEN"
	DiscrepancyInReview DiscrepancyTicketStatus = "IN_REVIEW"
	DiscrepancyResolved DiscrepancyTicketStatus = "RESOLVED"
)

// DiscrepancyTicket is auto-generated when a GoodsReceipt has a quantity mismatch
// (QtyReceived < QtyExpected for any line). Sent to HQ for accounting and logistics resolution.
// This handles lost or damaged goods from third-party logistics providers.
type DiscrepancyTicket struct {
	ID         string                  `json:"id"`
	GRID       string                  `json:"gr_id"`       // FK → GoodsReceipt (the triggering receipt)
	ItemID     string                  `json:"item_id"`     // FK → Item
	QtyMissing float64                 `json:"qty_missing"` // QtyExpected − QtyReceived (base units)
	QtyDamaged float64                 `json:"qty_damaged"` // Quantity received but unusable
	Status     DiscrepancyTicketStatus `json:"status"`
	Resolution *string                 `json:"resolution,omitempty"`  // HQ resolution notes
	ResolvedBy *string                 `json:"resolved_by,omitempty"` // FK → Staff (HQ)
	ResolvedAt *time.Time              `json:"resolved_at,omitempty"`
	CreatedAt  time.Time               `json:"created_at"`
	UpdatedAt  time.Time               `json:"updated_at"`
}

// ─── §1.4 B2B Sales Order (Wholesale Fulfillment) ────────────────────────────

// B2BSalesStatus represents the lifecycle of a B2B wholesale order.
type B2BSalesStatus string

const (
	B2BSalesPending     B2BSalesStatus = "PENDING"      // Awaiting Factory assignment
	B2BSalesAssigned    B2BSalesStatus = "ASSIGNED"     // Assigned to a Factory for fulfillment
	B2BSalesGoodsIssued B2BSalesStatus = "GOODS_ISSUED" // Factory has created and confirmed GoodsIssue
	B2BSalesInTransit   B2BSalesStatus = "IN_TRANSIT"   // Goods moving to external customer
	B2BSalesCompleted   B2BSalesStatus = "COMPLETED"    // Proof of Delivery received; no system GR
	B2BSalesCancelled   B2BSalesStatus = "CANCELLED"
)

// B2BSalesOrder is a wholesale order negotiated by HQ Sales and fulfilled by a Factory.
// Authority: HQ sells. Factory executes fulfillment only.
// No system GoodsReceipt — the order is COMPLETED upon Proof of Delivery from the logistics provider.
// Accounting: HQ recognizes revenue; Factory's GoodsIssue value is used as COGS.
type B2BSalesOrder struct {
	ID              string         `json:"id"`
	OrgID           string         `json:"org_id"`          // FK → Organization
	HQNodeID        string         `json:"hq_node_id"`      // FK → Node (HQ — the seller)
	FactoryNodeID   string         `json:"factory_node_id"` // FK → Node (Factory — the fulfiller)
	CustomerName    string         `json:"customer_name"`   // External wholesale customer
	CustomerContact string         `json:"customer_contact"`
	Status          B2BSalesStatus `json:"status"`
	ProofOfDelivery *string        `json:"proof_of_delivery,omitempty"` // URL to logistics POD document
	CreatedBy       string         `json:"created_by"`                  // FK → Staff (HQ Sales Team member)
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// B2BSalesOrderLine is a single item line in a B2B Sales Order.
type B2BSalesOrderLine struct {
	ID         string  `json:"id"`
	OrderID    string  `json:"order_id"`    // FK → B2BSalesOrder
	ItemID     string  `json:"item_id"`     // FK → Item
	QtyOrdered float64 `json:"qty_ordered"` // Quantity in base units
	UnitPrice  float64 `json:"unit_price"`  // Negotiated wholesale price per base unit
}
