package models

import (
	"encoding/json"
	"time"
)

// ─── SOPStep V2 — Idle Time Modeling ─────────────────────────────────────────

// AttentionLevel mô tả mức độ chú ý mà nhân viên cần duy trì
// trong thời gian idle của một bước (is_idle_step = true).
// Scheduler dùng field này để quyết định fill-in task nào có thể chèn vào idle window.
type AttentionLevel string

const (
	// AttentionFullIdle — máy tự chạy hoàn toàn. Nhân viên tự do đến station khác.
	// → Fill-in: bất kỳ task nào fit trong idle window.
	AttentionFullIdle AttentionLevel = "FULL_IDLE"

	// AttentionNearbyIdle — nhân viên cần ở gần máy (<=max_distance_meters).
	// → Fill-in: chỉ task cùng station hoặc không cần di chuyển xa.
	AttentionNearbyIdle AttentionLevel = "NEARBY_IDLE"

	// AttentionPeriodicCheck — nhân viên cần check định kỳ mỗi check_interval_sec giây.
	// → Fill-in: task có duration < check_interval_sec, phải có is_interruptible=true.
	AttentionPeriodicCheck AttentionLevel = "PERIODIC_CHECK"

	// AttentionActiveWait — nhân viên không được rời máy, phải đứng chờ.
	// → Không fill-in. Log idle_duration cho analytics.
	AttentionActiveWait AttentionLevel = "ACTIVE_WAIT"
)

// ─── BOM (Bill of Materials) ──────────────────────────────────────────────────

// BOM defines what components are needed to produce one unit of an output item.
// Defined at HQ level only — Factory and Store cannot create or modify BOMs.
// When a ProductionOrder is created, the current BOM version is snapshotted (see BOMSnapshot).
type BOM struct {
	ID           string `json:"id"`
	OutputItemID string `json:"output_item_id"` // FK → Item (the item being produced)
	Version      int    `json:"version"`        // Incremented on every change
}

// BOMLine is a single component (ingredient) in a BOM.
// All quantities are expressed in base units.
type BOMLine struct {
	ID     string  `json:"id"`      // Unique identifier — referenced by SOPStep.ingredient_bom_line_ids
	BOMID  string  `json:"bom_id"`  // FK → BOM
	ItemID string  `json:"item_id"` // FK → Item (the component/ingredient)
	Qty    float64 `json:"qty"`     // Quantity required in base units
}

// ─── SOP (Standard Operating Procedure) ──────────────────────────────────────

// SOP defines the step-by-step execution procedure for a given BOM.
// Each BOM has exactly one linked SOP.
type SOP struct {
	ID      string `json:"id"`
	BOMID   string `json:"bom_id"`  // FK → BOM
	Version int    `json:"version"` // Incremented on every change
}

// SOPStep is a single step in a SOP.
// equipment_type_id drives the queue allocation engine: when a ProductionOrder is decomposed
// into tasks, each task's required station type comes from this field.
// Steps may be parallelised using the depends_on DAG.
type SOPStep struct {
	ID            string   `json:"id"`                        // Unique identifier
	SOPID         string   `json:"sop_id"`                    // FK → SOP
	SeqNo         int      `json:"seq_no"`                    // Sequence number (execution order)
	DependsOn     []string `json:"depends_on"`                // IDs of steps that must complete before this one (DAG)
	EquipmentTypeID *string  `json:"equipment_type_id,omitempty"` // FK → EquipmentType — nil for manual/non-machine steps
	// IngredientBOMLineIDs lists the BOMLine IDs for ingredients consumed or added in this step.
	// Drives per-step material tracking and cost attribution.
	IngredientBOMLineIDs []string `json:"ingredient_bom_line_ids"`
	Duration             int      `json:"duration"`     // Estimated duration in seconds
	Description          string   `json:"description"`  // Human-readable instruction for staff
	
	// Bin-packing inputs — defined at SOP authoring time, stored on the step itself.
	// slot_consumption: how many capacity units one batch unit requires on the assigned machine.
	// allow_mix: if false, the machine must be dedicated to this item type for the entire cycle.
	SlotConsumption float64 `json:"slot_consumption"` // capacity units consumed per 1 batch unit
	AllowMix        bool    `json:"allow_mix"`        // false = exclusive machine use required

	// ── V2: Idle Time Modeling ───────────────────────────────────────────────
	// IsIdleStep = true khi máy tự chạy sau khi setup, nhân viên có thể rời trong một khoảng thời gian.
	// Khi true, scheduler sẽ tách bước này thành SETUP + WAITING + RETRIEVE sub-tasks.
	IsIdleStep bool `json:"is_idle_step"`

	// ActiveTime là số giây nhân viên cần thao tác trực tiếp để setup máy.
	// Chỉ có nghĩa khi IsIdleStep = true.
	// idle_window = Duration - ActiveTime - RequiresAttentionAt
	ActiveTime *int `json:"active_time,omitempty"`

	// AttentionLevel xác định mức độ chú ý cần thiết trong idle window.
	// BẮT BUỘC khi IsIdleStep = true. Scheduler dùng để lọc fill-in tasks phù hợp.
	AttentionLevel AttentionLevel `json:"attention_level,omitempty"`

	// CheckIntervalSec chỉ dùng khi AttentionLevel = PERIODIC_CHECK.
	// Fill-in task phải có duration < CheckIntervalSec - safety_buffer.
	CheckIntervalSec *int `json:"check_interval_sec,omitempty"`

	// RequiresAttentionAt là số giây TRƯỚC KHI bước kết thúc mà nhân viên phải quay lại.
	// Scheduler dùng để tính: idle_end = scheduled_end - RequiresAttentionAt
	// Alert sequence: T-2:00, T-0:45, T-0:00 tính từ scheduled_end - RequiresAttentionAt.
	RequiresAttentionAt *int `json:"requires_attention_at,omitempty"`
}

// ─── Production Order ─────────────────────────────────────────────────────────

type POStatus string

const (
	// POPending — created but awaiting stock availability check or scheduling.
	POPending POStatus = "PENDING"
	// POInProgress — stock confirmed, execution has begun. Batches are being allocated and run.
	POInProgress POStatus = "IN_PROGRESS"
	// POCompleted — all batches completed. POCostRecord is written and locked.
	POCompleted POStatus = "COMPLETED"
	// POCancelled — order was cancelled before completion.
	POCancelled POStatus = "CANCELLED"
)

// ProductionOrder is the central execution record generated from a BOM + SOP.
// It captures what is being produced, where, by whom, and at what cost.
//
// Status transitions:
//   PENDING → IN_PROGRESS: requires all BOM ingredients to have sufficient NodeStock.qty_on_hand.
//   If any ingredient is short, the PO stays PENDING and the manager is notified.
//   The PO auto-resumes when replenishment stock arrives.
//
//   IN_PROGRESS → COMPLETED: all child ProductionBatches are COMPLETED.
//   POCostRecord is written and locked at this transition.
type ProductionOrder struct {
	ID             string     `json:"id"`
	ItemID         string     `json:"item_id"`       // FK → Item (the target item being produced)
	BOMID          string     `json:"bom_id"`        // FK → BOM
	SOPID          string     `json:"sop_id"`        // FK → SOP
	NodeID         string     `json:"node_id"`       // FK → Node (where production happens)
	ReferenceOrderID string   `json:"reference_order_id,omitempty"` // FK → Order (the POS order that triggered this)
	TargetQty      float64    `json:"target_qty"`    // Units to produce (in base unit)
	YieldRate      float64    `json:"yield_rate"`    // Expected yield ratio (e.g., 0.95 = 95% expected output)
	PlannedInput   float64    `json:"planned_input"` // target_qty / yield_rate — raw input units needed
	ActualOutput   float64    `json:"actual_output"` // Recorded at completion — enables yield tracking
	Status         POStatus   `json:"status"`
	// DeadlineAt is the customer-facing SLA deadline for KDS priority scoring.
	// If nil, the system uses CreatedAt + node-level MaxPoolWaitSeconds config.
	DeadlineAt     *time.Time `json:"deadline_at,omitempty"`
	ScheduledStart time.Time  `json:"scheduled_start"`
	ScheduledEnd   time.Time  `json:"scheduled_end"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// BOMSnapshot locks the BOM version at the moment of ProductionOrder creation.
// Ensures cost calculations and material consumption always reference the exact BOM
// used for this order, even if the live BOM is later revised.
type BOMSnapshot struct {
	POID             string          `json:"po_id"`              // FK → ProductionOrder (1:1)
	LockedBOMVersion int             `json:"locked_bom_version"` // BOM.version at time of PO creation
	SnapshotData     json.RawMessage `json:"snapshot_data"`      // Full copy of BOM + BOMLines at time of locking
	CreatedAt        time.Time       `json:"created_at"`
}

// POStaffAssignment records staff members assigned to a ProductionOrder and hours worked.
// Used for labor cost calculation in POCostRecord.
//   labor_cost = Σ (hours × Staff.wage_rate)
type POStaffAssignment struct {
	POID    string  `json:"po_id"`    // FK → ProductionOrder
	StaffID string  `json:"staff_id"` // FK → Staff
	Hours   float64 `json:"hours"`    // Actual hours worked on this PO
}

// ─── Production Batch ─────────────────────────────────────────────────────────

// BatchStatus represents the lifecycle of a single machine cook cycle.
type BatchStatus string

const (
	// BatchQueued — batch is waiting for a machine of the required EquipmentType to become IDLE.
	BatchQueued BatchStatus = "QUEUED"
	// BatchAllocated — system has reserved a machine slot; waiting for staff to confirm item placement.
	BatchAllocated BatchStatus = "ALLOCATED"
	// BatchInProgress — staff confirmed placement; cook timer is running; machine is BUSY.
	BatchInProgress BatchStatus = "IN_PROGRESS"
	// BatchCompleted — cook cycle finished; StockConsumption records written; machine returns to IDLE.
	BatchCompleted BatchStatus = "COMPLETED"
	// BatchFailed — cook cycle failed (e.g., machine fault); machine returns to IDLE; batch may be re-queued.
	BatchFailed BatchStatus = "FAILED"
)

// ProductionBatch is the atomic execution unit that locks one machine for one cook cycle.
// A single ProductionOrder may generate multiple batches when:
//   - total quantity exceeds machine capacity (partial fill → split into next cycle), or
//   - the item has allow_mix=false and cannot share the machine with other item types.
//
// Lifecycle:
//   QUEUED → ALLOCATED  (system finds an IDLE machine; Machine.current_batch_id set)
//   ALLOCATED → IN_PROGRESS  (staff confirms item placement)
//   IN_PROGRESS → COMPLETED  (staff confirms completion; StockConsumption written; Machine → IDLE)
//   IN_PROGRESS → FAILED     (machine fault; Machine → IDLE; batch optionally re-queued)
type ProductionBatch struct {
	ID        string      `json:"id"`
	POID      string      `json:"po_id"`       // FK → ProductionOrder
	SOPStepID string      `json:"sop_step_id"` // FK → SOPStep (the step this batch executes)
	NodeID    string      `json:"node_id"`     // FK → Node (where the machine lives)
	MachineID string      `json:"machine_id"`  // FK → Machine (the physical machine assigned)
	ReferenceOrderID string `json:"reference_order_id,omitempty"` // FK → Order
	ItemID    string      `json:"item_id"`     // Single item type processed in this batch
	Qty       float64     `json:"qty"`         // Units in this batch (base unit)
	// SlotsUsed = Qty × SOPStep.SlotConsumption — must not exceed Machine.max_capacity.
	SlotsUsed float64     `json:"slots_used"`
	Status    BatchStatus `json:"status"`

	AllocatedAt         *time.Time `json:"allocated_at,omitempty"`         // When system reserved the machine slot
	StartedAt           *time.Time `json:"started_at,omitempty"`           // When staff confirmed item placement
	EstimatedCompletion *time.Time `json:"estimated_completion,omitempty"` // started_at + SOPStep.duration
	ActualEnd           *time.Time `json:"actual_end,omitempty"`           // When staff confirmed completion
}

// ─── Stock Consumption ────────────────────────────────────────────────────────

// StockConsumption records the actual material consumed during a ProductionBatch.
// Written at batch completion and has two effects:
//
//  1. Costing: feeds into POCostRecord.material_cost via Σ(qty_consumed × unit_cost).
//  2. Inventory: decrements NodeStock.qty_on_hand for (ProductionOrder.node_id, item_id).
//     After each decrement, the system checks if qty_on_hand ≤ NodeItemConfig.reorder_point
//     to determine whether to trigger replenishment.
type StockConsumption struct {
	POID        string  `json:"po_id"`        // FK → ProductionOrder
	BatchID     string  `json:"batch_id"`     // FK → ProductionBatch
	ItemID      string  `json:"item_id"`      // FK → Item (the consumed component)
	QtyConsumed float64 `json:"qty_consumed"` // Actual quantity consumed (base unit)
	// UnitCost is snapshotted from the supplying GoodsReceipt at consumption time.
	// Reflects the real purchase price of the specific supplier batch used.
	UnitCost float64 `json:"unit_cost"`
}
