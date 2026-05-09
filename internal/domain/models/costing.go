package models

import "time"

// ─── Overhead Config ──────────────────────────────────────────────────────────

// OverheadConfig defines the overhead cost rate per unit produced, set at HQ level per item.
// An optional node-level override can be specified.
type OverheadConfig struct {
	ItemID       string   `json:"item_id"`        // FK → Item
	NodeID       *string  `json:"node_id"`        // FK → Node (null = applies to all nodes)
	RatePerUnit  float64  `json:"rate_per_unit"`  // Overhead cost per 1 base unit of output
}

// ─── PO Cost Record ───────────────────────────────────────────────────────────

// POCostRecord is the immutable cost summary written and locked when a ProductionOrder
// reaches COMPLETED status.
// Formula:
//   material_cost  = Σ (StockConsumption.qty_consumed × StockConsumption.unit_cost)
//   labor_cost     = Σ (POStaffAssignment.hours × Staff.wage_rate)
//   overhead_cost  = ProductionOrder.actual_output × OverheadConfig.rate_per_unit
//   total_cost     = material_cost + labor_cost + overhead_cost
//   cost_per_unit  = total_cost / ProductionOrder.actual_output
type POCostRecord struct {
	POID         string    `json:"po_id"`          // FK → ProductionOrder (1:1)
	MaterialCost float64   `json:"material_cost"`
	LaborCost    float64   `json:"labor_cost"`
	OverheadCost float64   `json:"overhead_cost"`
	TotalCost    float64   `json:"total_cost"`
	CostPerUnit  float64   `json:"cost_per_unit"`
	LockedAt     time.Time `json:"locked_at"` // Timestamp when record was sealed
}

// ─── Cost Adjustment ─────────────────────────────────────────────────────────

// CostAdjustment is an audit-trail record for any manual correction to a finalized
// POCostRecord. The original record is never modified.
type CostAdjustment struct {
	ID         string    `json:"id"`
	POCostID   string    `json:"po_cost_id"`   // FK → POCostRecord
	Delta      float64   `json:"delta"`        // Correction amount (positive or negative)
	Reason     string    `json:"reason"`       // Mandatory human-readable justification
	AdjustedBy string    `json:"adjusted_by"`  // FK → Staff
	AdjustedAt time.Time `json:"adjusted_at"`
}
