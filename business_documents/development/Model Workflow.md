# OneSystem — Domain Model Workflow
**Version:** 1.0
**Status:** DRAFT — Derived from Model Graph v1.2

This document describes every domain model, its fields, relationships, and the data flow across the six architectural layers of OneSystem's production subsystem.

---

## Layer Overview

```
┌─────────────────────────────────────────────┐
│  Layer 1 — Config / Org                     │
│  Node · EquipmentType · Machine · Staff       │
├─────────────────────────────────────────────┤
│  Layer 2 — Item Config                      │
│  ItemCapacityConfig                         │
├─────────────────────────────────────────────┤
│  Layer 3 — Definition                       │
│  Item · UoM · BOM · BOMLine · SOP · SOPStep │
├─────────────────────────────────────────────┤
│  Layer 4 — Execution                        │
│  ProductionOrder · BOMSnapshot              │
│  POStaffAssignment                          │
├─────────────────────────────────────────────┤
│  Layer 5 — Batch                            │
│  ProductionBatch · StockConsumption         │
├─────────────────────────────────────────────┤
│  Layer 6 — Costing                          │
│  POCostRecord · OverheadConfig              │
│  CostAdjustment                             │
└─────────────────────────────────────────────┘
```

---

## Layer 1 — Config / Org

### Node
Represents a physical or logical operational location within a Tenant (ORG).

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `org_id` | string | FK → Organization (Tenant) |
| `type` | NodeType | `HQ` \| `STORE` \| `FACTORY` |
| `name` | string | Human-readable name |
| `address` | string | Physical address |

**Rules:**
- A Tenant has exactly one `HQ`, zero or more `STORE`s, and one `FACTORY` (v1).
- Machines and Staff are scoped to a `node_id`.

---

### EquipmentType
Defines a category of kitchen equipment (e.g., Fryer, Oven, Grill).

| Field | Type | Description |
|---|---|---|
| `id` | string | Enum-style key: `FRYER`, `OVEN`, `GRILL`, etc. |
| `name` | string | Display label |
| `capacity_unit` | string | Unit of capacity: `slots`, `liters`, `trays`, etc. |

---

### Machine
A specific physical machine instance at a node.

| Field | Type | Description |
|---|---|---|
| `id` | string | e.g., `M_FRYER_01`, `M_OVEN_02` |
| `equipment_type_id` | string | FK → `EquipmentType` |
| `node_id` | string | FK → `Node` |
| `max_slots` | int | Total capacity in `EquipmentType.capacity_unit` |
| `status` | MachineStatus | `IDLE` \| `BUSY` \| `UNDER_MAINTENANCE` \| `DECOMMISSIONED` |
| `current_batch_id` | string? | FK → `ProductionBatch` (null when IDLE) |
| `linked_asset_id` | string? | FK → `Asset` (Supply Chain domain) — Populated when this machine was procured via PR → PurO → GR. `null` for pre-existing or manually registered machines. |

**Rules:**
- A machine belongs to exactly one Node.
- `IDLE` and `BUSY` are transitioned by the batch allocation engine — never set manually.
- `UNDER_MAINTENANCE` and `DECOMMISSIONED` are driven by the Asset lifecycle (Supply Chain domain), synchronized from `Asset.status`. They are never set by the batch engine.
- A `DECOMMISSIONED` machine is excluded from the bin-packing algorithm and cannot accept new `ProductionBatch` entries.

---

### Staff
A staff member working at a node, relevant to production labor costing.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `node_id` | string | FK → `Node` |
| `name` | string | Full name |
| `wage_rate` | float64 | Hourly wage rate (in base currency) |

---

## Layer 2 — Item Config

### ItemCapacityConfig
Links an **Item** to a **EquipmentType** and defines how much machine capacity one base unit of that item consumes, and whether it tolerates batch mixing.

| Field | Type | Description |
|---|---|---|
| `item_id` | string | FK → `Item` |
| `equipment_type_id` | string | FK → `EquipmentType` |
| `slot_consumption` | float64 | Capacity units consumed per 1 base unit of the item |
| `allow_mix` | bool | If `false`, this item requires exclusive machine use during its cycle |

**Example:**

| Item | EquipmentType | slot_consumption | allow_mix |
|---|---|---|---|
| Burger bun | OVEN | 1 (slot) | true |
| Egg | FRYER | 1 (liter) | false |
| Potato portion | FRYER | 2 (liters) | false |

**Rules:**
- An item may have configs for multiple equipment types (e.g., an item that can be grilled or fried).
- This record is the input to the bin-packing algorithm.

---

## Layer 3 — Definition

### Item
The base entity for all physical goods in the system.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `name` | string | Display name |
| `type` | ItemType | `PRODUCT` \| `SEMI_PRODUCT` \| `RAW_MATERIAL` |
| `base_unit` | string | The smallest consumable unit (e.g., `piece`, `ml`, `gram`) |
| `sku` | string | Stock-keeping unit code |

---

### UoM (Unit of Measure)
Defines packaging/ordering units for an item and their conversion to base units.

| Field | Type | Description |
|---|---|---|
| `item_id` | string | FK → `Item` |
| `pkg_unit` | string | Packaging unit name (e.g., `bag`, `case`, `bottle`) |
| `conversion` | float64 | How many base units are in one packaging unit |

**Example:** For "Burger bun" with base_unit `piece` — `{ pkg_unit: "bag", conversion: 10 }`.

---

### BOM (Bill of Materials)
Defines what components are needed to produce one unit of an output item.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `output_item_id` | string | FK → `Item` (the item being produced) |
| `version` | int | Incrementing version number |

**Rules:**
- Defined at HQ only. Stores and Factory cannot modify BOMs.
- When a Production Order is created, the current BOM version is snapshotted (see `BOMSnapshot`).

---

### BOMLine
Each component (ingredient) in a BOM.

| Field | Type | Description |
|---|---|---|
| `bom_id` | string | FK → `BOM` |
| `item_id` | string | FK → `Item` (the component) |
| `qty` | float64 | Quantity required in base units |

---

### SOP (Standard Operating Procedure)
The step-by-step execution procedure for a BOM.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `bom_id` | string | FK → `BOM` |
| `version` | int | Incrementing version number |

---

### SOPStep
An individual step in a SOP.

| Field | Type | Description |
|---|---|---|
| `sop_id` | string | FK → `SOP` |
| `seq_no` | int | Sequence number (execution order) |
| `equipment_type_id` | string | FK → `EquipmentType` — machine category required for this step |
| `duration` | int | Estimated duration in seconds |
| `description` | string | Human-readable instruction |

**Key field:** `equipment_type_id` is what drives the queue allocation — when a Production Order is decomposed into tasks, each task's required equipment type comes from this field.

---

## Layer 4 — Execution

### ProductionOrder
The central execution record. Generated from a BOM + SOP to produce a target quantity of an item.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `bom_id` | string | FK → `BOM` |
| `sop_id` | string | FK → `SOP` |
| `node_id` | string | FK → `Node` (where production happens) |
| `target_qty` | float64 | How many units to produce |
| `yield_rate` | float64 | Expected yield (e.g., 0.95 = 95% expected output) |
| `planned_input` | float64 | `target_qty / yield_rate` — raw input needed |
| `actual_output` | float64 | Recorded at completion |
| `status` | POStatus | `PENDING` → `IN_PROGRESS` → `COMPLETED` / `CANCELLED` |
| `scheduled_start` | time | Planned start |
| `scheduled_end` | time | Planned end |
| `created_at` | time | |
| `updated_at` | time | |

**Rules:**
- **Stock Availability Check:** Before transitioning `PENDING → IN_PROGRESS`, the system verifies `NodeStock.qty_on_hand ≥ (BOMLine.qty × planned_input)` for every ingredient in the BOM snapshot. If any ingredient has insufficient stock, the PO remains `PENDING` and the Factory Manager is notified.
- A blocked PO **auto-resumes** when a new `GoodsReceipt` or replenishment raises the relevant `NodeStock.qty_on_hand` above the required threshold.
- `PENDING` means: created but awaiting stock or scheduling. `IN_PROGRESS` means: stock confirmed and execution has begun.

---

### BOMSnapshot
Locks the BOM version at the moment of Production Order creation. Ensures cost calculations and material consumption always reference the exact BOM used, even if the live BOM is later revised.

| Field | Type | Description |
|---|---|---|
| `po_id` | string | FK → `ProductionOrder` |
| `locked_bom_version` | int | The BOM version snapshotted at PO creation |
| `snapshot_data` | JSON | Full copy of BOM + BOMLines at time of locking |

---

### POStaffAssignment
Records which staff members are assigned to a Production Order and how many hours they worked (used for labor cost calculation).

| Field | Type | Description |
|---|---|---|
| `po_id` | string | FK → `ProductionOrder` |
| `staff_id` | string | FK → `Staff` |
| `hours` | float64 | Actual hours worked on this PO |

---

## Layer 5 — Batch

### ProductionBatch
The atomic execution unit that locks one machine for one cook cycle. A single `ProductionOrder` may generate multiple batches.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `po_id` | string | FK → `ProductionOrder` |
| `machine_id` | string | FK → `Machine` |
| `item_id` | string | FK → `Item` (the single item type in this batch) |
| `qty` | float64 | Units in this batch (base unit) |
| `slots_used` | float64 | Total capacity consumed (`qty × slot_consumption`) |
| `status` | BatchStatus | `QUEUED` → `IN_PROGRESS` → `COMPLETED` / `FAILED` |
| `started_at` | time | When machine was claimed |
| `estimated_completion` | time | `started_at + SOPStep.duration` |
| `actual_end` | time | Real completion time |

**Lifecycle transitions:**
```
QUEUED → IN_PROGRESS  (machine becomes BUSY, Machine.current_batch_id set)
IN_PROGRESS → COMPLETED  (machine returns to IDLE, StockConsumption recorded)
IN_PROGRESS → FAILED  (machine returns to IDLE, batch may be re-queued)
```

---

### StockConsumption
Records actual material consumed during a batch. Written at batch completion.

| Field | Type | Description |
|---|---|---|
| `po_id` | string | FK → `ProductionOrder` |
| `batch_id` | string | FK → `ProductionBatch` |
| `item_id` | string | FK → `Item` (the consumed component) |
| `qty_consumed` | float64 | Actual quantity consumed (base unit) |
| `unit_cost` | float64 | Unit cost of this item at the time of the supplying Goods Receipt |

**Purpose (dual effect):**
- **Costing:** Feeds directly into `POCostRecord.material_cost` (`Σ qty_consumed × unit_cost`).
- **Inventory:** Decrements `NodeStock.qty_on_hand` for `(ProductionOrder.node_id, item_id)` by `qty_consumed`. This is the production domain's write path into the shared inventory ledger. After each decrement, the system checks `NodeStock.qty_on_hand ≤ NodeItemConfig.reorder_point` to determine whether to fire replenishment.

---

## Layer 6 — Costing

### OverheadConfig
Configures the overhead cost rate per unit produced, set at HQ level per item.

| Field | Type | Description |
|---|---|---|
| `item_id` | string | FK → `Item` |
| `rate_per_unit` | float64 | Overhead cost per one base unit of output |
| `node_id` | string? | Optional: node-level override (null = applies to all nodes) |

---

### POCostRecord
The immutable cost summary for a completed Production Order. Written and locked when PO status transitions to `COMPLETED`.

| Field | Type | Description |
|---|---|---|
| `po_id` | string | FK → `ProductionOrder` (1:1) |
| `material_cost` | float64 | `Σ (qty_consumed × unit_cost)` from StockConsumption |
| `labor_cost` | float64 | `Σ (hours × wage_rate)` from POStaffAssignment |
| `overhead_cost` | float64 | `actual_output × OverheadConfig.rate_per_unit` |
| `total_cost` | float64 | `material_cost + labor_cost + overhead_cost` |
| `cost_per_unit` | float64 | `total_cost / actual_output` |
| `locked_at` | time | Timestamp when record was sealed |

> **Immutability rule:** Once written, this record is never modified. Corrections go through `CostAdjustment`.

---

### CostAdjustment
An audit trail record for any manual correction to a finalized `POCostRecord`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `po_cost_id` | string | FK → `POCostRecord` |
| `delta` | float64 | The correction amount (positive or negative) |
| `reason` | string | Mandatory human-readable justification |
| `adjusted_by` | string | FK → `Staff` who made the adjustment |
| `adjusted_at` | time | Timestamp |

---

## End-to-End Data Flow

```
[Order arrives at Store]
        │
        ▼
[ProductionOrder created]
  ├── BOMSnapshot locked (captures BOM version)
  ├── POStaffAssignment records assigned staff
  ├── Stock Availability Check (per BOMLine ingredient):
  │     NodeStock.qty_on_hand ≥ BOMLine.qty × planned_input?
  │     ├── ✅ All available → PO.status = IN_PROGRESS
  │     └── ❌ Insufficient → PO.status = PENDING (blocked; Factory Manager notified)
  │                         PO auto-resumes when replenishment stock arrives
  └── If IN_PROGRESS: PO decomposed into Tasks (one per SOPStep)
        │
        ▼
[Tasks enter Priority Queue by EquipmentType]
        │
        ▼
[Machine transitions to IDLE]
        │
        ▼
[Bin-Packing: Greedy fill]
  ├── Filter by equipment_type_id
  ├── Respect allow_mix constraint
  └── Pack until max_slots reached
        │
        ▼
[ProductionBatch created → Machine BUSY]
        │
        ▼
[Batch executes]
        │
        ▼
[Batch COMPLETED]
  ├── StockConsumption records written (qty_consumed × unit_cost)
  ├── NodeStock.qty_on_hand decremented for each consumed item  ← Stock Out
  ├── System checks: qty_on_hand ≤ reorder_point? → fire replenishment if true
  └── Machine → IDLE (next batch dequeued)
        │
        ▼ (when all batches for PO are COMPLETED)
[ProductionOrder → COMPLETED]
        │
        ▼
[POCostRecord written & locked]
  ├── material_cost  = Σ StockConsumption
  ├── labor_cost     = Σ POStaffAssignment hours × wage_rate
  ├── overhead_cost  = actual_output × OverheadConfig.rate_per_unit
  └── cost_per_unit  = total_cost / actual_output
```

---

## Model Dependency Map

| Model | Depends On |
|---|---|
| `Node` | `Organization` |
| `Machine` | `Node`, `EquipmentType`, `Asset`? (Supply Chain — optional provenance) |
| `Staff` | `Node` |
| `ItemCapacityConfig` | `Item`, `EquipmentType` |
| `UoM` | `Item` |
| `BOM` | `Item` (output) |
| `BOMLine` | `BOM`, `Item` (component) |
| `SOP` | `BOM` |
| `SOPStep` | `SOP`, `EquipmentType` |
| `ProductionOrder` | `BOM`, `SOP`, `Node` |
| `BOMSnapshot` | `ProductionOrder`, `BOM` |
| `POStaffAssignment` | `ProductionOrder`, `Staff` |
| `ProductionBatch` | `ProductionOrder`, `Machine`, `Item` |
| `StockConsumption` | `ProductionOrder`, `ProductionBatch`, `Item`, `NodeStock` (mutates) |
| `OverheadConfig` | `Item`, `Node`? |
| `POCostRecord` | `ProductionOrder` |
| `CostAdjustment` | `POCostRecord`, `Staff` |

---

*Document Owner: BA Team*
*Status: DRAFT v1.0 — Aligned with Business Specification v1.2 and Model Graph v1.2*
