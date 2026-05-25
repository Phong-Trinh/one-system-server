package models

import "time"

// ─── §2.4 NodeStock — Live Inventory Ledger ───────────────────────────────────

// NodeStock is the live inventory record for every (Item, Node) pair.
// It is the single source of truth for current stock on hand.
//
// The ROP engine reads NodeStock.QtyOnHand after every stock-decreasing event to decide
// whether to fire replenishment. The Production Gate checks it before transitioning a
// ProductionOrder from PENDING to IN_PROGRESS.
//
// Stock events and their effects:
//   GoodsReceipt confirmed (from PurchaseOrder or ITO)  →  ➕ Stock In  (QtyOnHand increases)
//   GoodsIssue confirmed (ITO dispatch or B2B shipment) →  ➖ Stock Out (QtyOnHand decreases)
//   ProductionBatch completed (raw material consumed)   →  ➖ Stock Out (QtyOnHand decreases)
//
// All quantities are in the Item's base unit.
type NodeStock struct {
	ItemID        string    `json:"item_id"`         // FK → Item (composite PK part 1)
	NodeID        string    `json:"node_id"`         // FK → Node (composite PK part 2)
	QtyOnHand     float64   `json:"qty_on_hand"`     // Current quantity on hand (base units)
	LastUpdatedAt time.Time `json:"last_updated_at"` // Timestamp of last stock-changing event
}

// ─── §2.1 NodeItemConfig — ROP & Sourcing Strategy Config ─────────────────────

// NodeItemConfig holds per-item, per-node operational configuration including:
//   - Reorder Point (ROP) and Safety Stock for the replenishment engine
//   - Sourcing Strategy determining which document is auto-created when ROP is breached
//   - Dual-Consumption parameters for Factory items (C_prod + C_transfer)
//
// Every item at every node that participates in replenishment must have a NodeItemConfig.
//
// ROP Formulas:
//
//   Standard (Store or simple Factory item):
//     ROP = (DailyConsumption × SupplierLeadTimeDays) + SafetyStock
//
//   Factory Dual-Consumption (item consumed by both production BOM and outbound ITOs):
//     ROP = ((CProd + CTransfer) × SupplierLeadTimeDays) + SafetyStock
//     where:
//       CProd      = avg daily BOM consumption (base units/day)
//       CTransfer  = avg daily outbound ITO volume (base units/day)
//       Both are computed from a rolling historical average window (default: 30 days)
type NodeItemConfig struct {
	ItemID          string           `json:"item_id"`           // FK → Item (composite PK part 1)
	NodeID          string           `json:"node_id"`           // FK → Node (composite PK part 2)
	SourcingStrategy SourcingStrategy `json:"sourcing_strategy"` // INTERNAL_TRANSFER | EXTERNAL_PROCUREMENT
	// ProviderNodeID is required when SourcingStrategy = INTERNAL_TRANSFER.
	// Identifies the Factory or Store that will fulfill replenishment ITOs.
	ProviderNodeID  *string  `json:"provider_node_id,omitempty"` // FK → Node
	// SupplierID is relevant when SourcingStrategy = EXTERNAL_PROCUREMENT.
	// Used to pre-populate the auto-generated Draft PurchaseOrder.
	SupplierID      *string  `json:"supplier_id,omitempty"` // FK → Supplier

	// ─── ROP Parameters ───────────────────────────────────────────────────────

	// ReorderPoint is the stock level (base units) at which replenishment is triggered.
	// After every stock-decreasing event, the engine checks QtyOnHand <= ReorderPoint.
	ReorderPoint         float64 `json:"reorder_point"`          // In base units
	SafetyStock          float64 `json:"safety_stock"`           // Buffer stock in base units
	SupplierLeadTimeDays int     `json:"supplier_lead_time_days"` // Lead time from supplier to this node

	// ─── Factory Dual-Consumption Parameters (Factory nodes only) ─────────────

	// CProd is the average daily consumption of this item by BOM production runs at this Factory.
	// Nil for non-Factory nodes or items not used in production.
	CProd *float64 `json:"c_prod,omitempty"` // Base units / day
	// CTransfer is the average daily volume of this item dispatched outbound via ITOs to Stores.
	// Nil for non-Factory nodes or items not transferred out.
	CTransfer *float64 `json:"c_transfer,omitempty"` // Base units / day

	// ConsumptionWindowDays is the rolling historical window (in days) used to compute
	// the CProd and CTransfer rolling averages. Default: 30 days.
	ConsumptionWindowDays int `json:"consumption_window_days"`

	UpdatedAt time.Time `json:"updated_at"`
}
