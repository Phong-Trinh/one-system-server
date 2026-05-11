package models

import "time"

// ─── BOM (Bill of Materials) ──────────────────────────────────────────────────

// BOM defines what components are needed to produce one unit of an output item.
// Defined at HQ level only.
type BOM struct {
	ID           string `json:"id"`
	OutputItemID string `json:"output_item_id"` // FK → Item (the produced item)
	Version      int    `json:"version"`        // Incremented on every change
}

// BOMLine is a single component (ingredient) in a BOM.
type BOMLine struct {
	ID     string  `json:"id"`      // Unique identifier for the BOM line
	BOMID  string  `json:"bom_id"`  // FK → BOM
	ItemID string  `json:"item_id"` // FK → Item (the component)
	Qty    float64 `json:"qty"`     // Quantity required in base units
}

// ─── SOP (Standard Operating Procedure) ──────────────────────────────────────

// SOP defines the step-by-step execution procedure for a given BOM.
type SOP struct {
	ID      string `json:"id"`
	BOMID   string `json:"bom_id"`  // FK → BOM
	Version int    `json:"version"` // Incremented on every change
}

// SOPStep is a single step in a SOP.
// station_type_id drives the production queue allocation engine.
type SOPStep struct {
	ID                     string   `json:"id"`                        // Unique identifier for the step
	SOPID                  string   `json:"sop_id"`                    // FK → SOP
	SeqNo                  int      `json:"seq_no"`                    // Sequence number (execution order)
	DependsOn              []string `json:"depends_on"`                // IDs of steps that must complete before this one
	StationTypeID          string   `json:"station_type_id,omitempty"` // FK → StationType — optional (manual steps)
	IngredientBOMLineIDs   []string `json:"ingredient_bom_line_ids"`   // FKs → BOMLine.ID (ingredients added in this step)
	Duration               int      `json:"duration"`                  // Estimated duration in seconds
	Description            string   `json:"description"`               // Human-readable instruction
}

// ─── Production Order ─────────────────────────────────────────────────────────

type POStatus string

const (
	POPending    POStatus = "PENDING"
	POInProgress POStatus = "IN_PROGRESS"
	POCompleted  POStatus = "COMPLETED"
	POCancelled  POStatus = "CANCELLED"
)

// ProductionOrder is the central execution record generated from a BOM + SOP.
type ProductionOrder struct {
	ID             string    `json:"id"`
	BOMID          string    `json:"bom_id"`        // FK → BOM
	SOPID          string    `json:"sop_id"`        // FK → SOP
	NodeID         string    `json:"node_id"`       // FK → Node (where production happens)
	TargetQty      float64   `json:"target_qty"`    // Units to produce
	YieldRate      float64   `json:"yield_rate"`    // Expected yield (e.g., 0.95 = 95%)
	PlannedInput   float64   `json:"planned_input"` // target_qty / yield_rate
	ActualOutput   float64   `json:"actual_output"` // Recorded at completion
	Status         POStatus  `json:"status"`
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BOMSnapshot locks the BOM version at the moment of Production Order creation.
// Ensures cost calculations always reference the exact BOM used.
type BOMSnapshot struct {
	POID             string `json:"po_id"`              // FK → ProductionOrder (1:1)
	LockedBOMVersion int    `json:"locked_bom_version"` // BOM.version at time of PO creation
	SnapshotData     string `json:"snapshot_data"`      // JSON: full BOM + BOMLines copy
}

// POStaffAssignment records staff assigned to a Production Order and hours worked.
// Used for labor cost calculation in POCostRecord.
type POStaffAssignment struct {
	POID    string  `json:"po_id"`    // FK → ProductionOrder
	StaffID string  `json:"staff_id"` // FK → Staff
	Hours   float64 `json:"hours"`    // Actual hours worked on this PO
}

// ─── Production Batch ─────────────────────────────────────────────────────────

type BatchStatus string

const (
	BatchQueued     BatchStatus = "QUEUED"
	BatchInProgress BatchStatus = "IN_PROGRESS"
	BatchCompleted  BatchStatus = "COMPLETED"
	BatchFailed     BatchStatus = "FAILED"
)

// ProductionBatch is the atomic execution unit that locks one machine for one cook cycle.
// A single ProductionOrder may generate multiple batches (capacity overflow, or mix constraint).
type ProductionBatch struct {
	ID                  string      `json:"id"`
	POID                string      `json:"po_id"`       // FK → ProductionOrder
	SOPStepID           string      `json:"sop_step_id"` // FK → SOPStep
	MachineID           string      `json:"machine_id"`  // FK → Machine
	ItemID              string      `json:"item_id"`     // Single item type in this batch
	Qty                 float64     `json:"qty"`         // Units in this batch (base unit)
	SlotsUsed           float64     `json:"slots_used"` // qty × ItemCapacityConfig.slot_consumption
	Status              BatchStatus `json:"status"`
	StartedAt           *time.Time  `json:"started_at"`           // When machine was claimed
	EstimatedCompletion *time.Time  `json:"estimated_completion"` // started_at + SOPStep.duration
	ActualEnd           *time.Time  `json:"actual_end"`           // Real completion time
}

// StockConsumption records actual material consumed during a batch.
// Written at batch completion and feeds into POCostRecord.material_cost.
type StockConsumption struct {
	POID        string  `json:"po_id"`        // FK → ProductionOrder
	BatchID     string  `json:"batch_id"`     // FK → ProductionBatch
	ItemID      string  `json:"item_id"`      // FK → Item (the consumed component)
	QtyConsumed float64 `json:"qty_consumed"` // Actual quantity consumed (base unit)
	UnitCost    float64 `json:"unit_cost"`    // Unit cost at the time of the supplying Goods Receipt
}
