# OneSystem — Supply Chain Domain Models
**Version:** 2.1
**Status:** DRAFT

This document describes the domain models, fields, and relationships required to implement the Supply Chain workflows defined in [OneSystem Supply Chain & Inventory](../OneSystem%20Supply%20Chain%20&%20Inventory.md).

---

## 1. Core Config & Virtual Nodes

To ensure strict financial accounting (Cost Center vs Profit Center) and clear inventory accountability, OneSystem uses a **Strict Typology** for nodes. Virtual Nodes are grouped by `site_id` to support co-located operations.

### Node
A physical or virtual operational location.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `type` | NodeType | `HQ` \| `STORE` \| `FACTORY` |
| `site_id` | string | Identifier grouping nodes that share the same physical address |

*Rule:* If a client operates a Hybrid Location (e.g., Bakery + Storefront in one building), they MUST create two virtual nodes sharing the same `site_id` (e.g., F1 and S1). This ensures 2 separate inventory ledgers.

---

### NodeItemConfig
Per-item, per-node configuration. **This is the engine that drives all automatic replenishment.** When the system detects that a node's stock for an item has fallen to or below `reorder_point`, it fires the action defined by `sourcing_strategy`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `node_id` | string | FK → `Node` |
| `item_id` | string | FK → `Item` |
| `sourcing_strategy` | SourcingStrategy | `INTERNAL_TRANSFER` \| `EXTERNAL_PROCUREMENT` |
| `provider_node_id` | string? | FK → `Node` — **Required** when `sourcing_strategy = INTERNAL_TRANSFER`. The node that will fulfill internal transfer requests (e.g., Factory or another Store). |
| `preferred_supplier_id` | string? | FK → `Supplier` — Optional hint for HQ when auto-drafting a PurO under `EXTERNAL_PROCUREMENT`. |
| `reorder_point` | float64 | ROP threshold in **Base Units**. System fires replenishment when `NodeStock.qty_on_hand ≤ NodeItemConfig.reorder_point`. |
| `safety_stock` | float64 | Buffer stock in Base Units. Included in ROP calculation. |
| `lead_time_days` | float64 | Expected days from order to delivery. Used in ROP formula. |
| `c_prod` | float64? | **Factory only.** Average daily BOM production consumption (Base Units/day). Auto-updated by system from rolling average. |
| `c_transfer` | float64? | **Factory only.** Average daily outbound transfer to Stores via `InternalTransferOrder` (Base Units/day). Auto-updated by system from rolling average. |
| `rolling_avg_window_days` | int | Window in days for computing `c_prod` and `c_transfer` averages. Default: `30`. |

**ROP Formulas:**

```
-- Standard (Store or single-consumption Factory item):
ROP = (daily_consumption × lead_time_days) + safety_stock

-- Factory dual-consumption item:
ROP = ((c_prod + c_transfer) × lead_time_days) + safety_stock
```

**System Action on ROP Breach (by `sourcing_strategy`):**

| `sourcing_strategy` | System Action | Resulting Document |
|---|---|---|
| `INTERNAL_TRANSFER` | Auto-create `InternalTransferOrder` (`type = AUTO_REPLENISHMENT`) to `provider_node_id` | `InternalTransferOrder` |
| `EXTERNAL_PROCUREMENT` | Auto-create `PurchaseOrder` (`status = DRAFT`) on HQ Dashboard with `delivery_to = node_id` | `PurchaseOrder` |

---

## 2. Inventory Ledger

### NodeStock
The live stock balance for one item at one node. This is **the source of truth for current quantity on hand** and the table the ROP engine reads to decide whether to fire replenishment.

| Field | Type | Description |
|---|---|---|
| `node_id` | string | FK → `Node` — composite PK (part 1) |
| `item_id` | string | FK → `Item` — composite PK (part 2) |
| `qty_on_hand` | float64 | Current stock in **Base Units**. The value checked against `NodeItemConfig.reorder_point`. |
| `last_updated_at` | time | Timestamp of the last mutation |

**Stock Mutation Rules — what writes to `qty_on_hand`:**

| Event | Direction | Operation | ROP Check? | Source |
|---|---|---|---|---|
| `GoodsReceipt` confirmed (any `source_ref_type`) | ➕ Stock In | `qty_on_hand += GR.qty_received` (converted to Base Units) | ❌ No — stock rising | §5 Execution Logistics |
| `GoodsIssue` confirmed (ITO dispatch) | ➖ Stock Out | `qty_on_hand -= GI.qty_dispatched` (converted to Base Units) | ✅ Yes — check if `qty_on_hand ≤ reorder_point` after decrement | §5 Execution Logistics |
| `GoodsIssue` confirmed (B2B dispatch) | ➖ Stock Out | `qty_on_hand -= GI.qty_dispatched` (converted to Base Units) | ✅ Yes — check if `qty_on_hand ≤ reorder_point` after decrement | §6 Exception & B2B |
| `StockConsumption` written at batch completion | ➖ Production | `qty_on_hand -= StockConsumption.qty_consumed` (Base Units) | ✅ Yes — check if `qty_on_hand ≤ reorder_point` after decrement | Production Domain |

**Rules:**
- `NodeStock` is initialized when a node is first configured for an item (e.g., during onboarding or when a `NodeItemConfig` is created). Initial `qty_on_hand` is set manually via a stock-take.
- The system runs a **ROP check** (`NodeStock.qty_on_hand ≤ NodeItemConfig.reorder_point`) after every **stock-decreasing** mutation (GI dispatch, StockConsumption). Stock-increasing mutations (GR) do not trigger a ROP check.
- HQ node does **not** have a `NodeStock` record — HQ never holds physical inventory.
- `qty_on_hand` is always in Base Units. UI layers convert to display units using `UoM.conversion`.

---

## 3. Internal Logistics (OpEx)

### InternalTransferOrder (ITO)
Used exclusively for moving goods between internal nodes (e.g., F → S, or S → S). Replaces the old `SupplyRequest`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `type` | ITOType | `AUTO_REPLENISHMENT` \| `MANUAL_REQUEST` |
| `config_ref_id` | string? | FK → `NodeItemConfig` — Populated for `AUTO_REPLENISHMENT` orders; identifies the config entry that triggered this order. `null` for `MANUAL_REQUEST`. |
| `requester_node_id` | string | FK → `Node` (who needs the goods) |
| `provider_node_id` | string | FK → `Node` (who supplies the goods) |
| `item_id` | string | FK → `Item` |
| `qty_requested` | float64 | Amount requested (in Packaging Units; system converts to Base Units internally) |
| `status` | ITOStatus | See status flow below |

**Status Flow:**
```
AUTO_REPLENISHMENT:  AUTO_APPROVED → IN_PROGRESS → IN_TRANSIT → COMPLETED
                                                              ↘ DISCREPANCY (if GI qty ≠ GR qty)
MANUAL_REQUEST:      PENDING_APPROVAL → APPROVED → IN_PROGRESS → IN_TRANSIT → COMPLETED
                                      ↘ REJECTED
                     (any state) → CANCELLED
```

### Same-Site Transfer (UX Optimization)
When `requester_node_id.site_id == provider_node_id.site_id`, the ITO executes in **1-click mode**:
- GI is auto-created with `is_same_site = true` (no driver info, no media proof, zero shipping fee).
- GR is auto-confirmed immediately at the destination node.
- The ITO skips the `IN_TRANSIT` state entirely — transitions directly from `IN_PROGRESS` to `COMPLETED`.

---

## 4. External Procurement

### PurchaseRequisition (PR)
**Scope: CapEx only.** Used for long-term assets (equipment, tools) or exceptional one-off purchases. Routine OpEx replenishment does NOT use PR — it goes directly to an auto-draft `PurchaseOrder`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `requester_node_id` | string | FK → `Node` (Store or Factory submitting the request) |
| `item_name` | string | Name of the requested asset/equipment |
| `estimated_cost` | float64 | Expected cost |
| `justification` | string | Reason for request (e.g., "Old fryer broken, needs replacement") |
| `status` | PRStatus | `PENDING_HQ_APPROVAL` → `APPROVED` → `REJECTED` |
| `linked_puro_id` | string? | FK → `PurchaseOrder` — Populated once HQ converts the approved PR into a PurO |

---

### PurchaseOrder (PurO)
The official external order sent to a Supplier. **Exclusively created by HQ.** Goods are delivered directly to the `destination_node_id` — HQ never physically handles the goods.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `trigger_type` | PurOTriggerType | `AUTO_DRAFT` \| `PR_TRIGGERED` — Identifies how this PurO was created |
| `creator_node_id` | string | FK → `Node` — Always an HQ node |
| `supplier_id` | string | FK → `Supplier` |
| `destination_node_id` | string | FK → `Node` — Where the supplier must deliver (Factory or Store). **Never HQ.** |
| `linked_pr_id` | string? | FK → `PurchaseRequisition` — Populated only when `trigger_type = PR_TRIGGERED` |
| `linked_config_ref_id` | string? | FK → `NodeItemConfig` — Populated only when `trigger_type = AUTO_DRAFT`; identifies the ROP config that triggered this order |
| `items` | []PurOLineItem | Line items: `item_id`, `qty_ordered` (Packaging Units), `unit_price` |
| `total_amount` | float64 | Total value of order |
| `status` | PurOStatus | See status flow below |

**Status Flow — `AUTO_DRAFT` path (ROP-triggered):**

| Step | Transition | Actor | Fields Set / Action |
|---|---|---|---|
| 1 | *(ROP breach detected)* | System | Reads `NodeItemConfig`: `node_id`, `item_id`, `preferred_supplier_id` |
| 2 | → `DRAFT` | System | Sets `id`, `trigger_type=AUTO_DRAFT`, `creator_node_id` *(HQ node, system-assigned)*, `destination_node_id` *(= triggering node)*, `linked_config_ref_id`, `items[].item_id`, `items[].qty_ordered` *(ROP gap)*. ⚠️ `supplier_id`, `unit_price`, `total_amount` are **not set yet** — HQ fills these in Step 3. |
| 3 | `DRAFT` → `ISSUED` | HQ | HQ fills: `supplier_id`, `items[].unit_price`. System computes `total_amount`. **Price is first attached here.** PurO formally sent to supplier. |
| 4 | `ISSUED` → `IN_TRANSIT` | HQ / Supplier | No new PurO fields set. Supplier has received the order and ships goods. |
| 5 | `IN_TRANSIT` → `DELIVERED` | Store / Factory | Destination node creates `GoodsReceipt` (`source_ref_id = PurO.id`, `qty_received`, `media_proof_urls`). |
| 6 | `DELIVERED` → `PAYMENT_SETTLED` | HQ | 3-Way Matching: `PurO` + `Supplier Invoice` + `GoodsReceipt` validated. Payment authorized. |

**Status Flow — `PR_TRIGGERED` path (CapEx / manual PR):**

| Step | Transition | Actor | Fields Set / Action |
|---|---|---|---|
| 1 | *(PR approved)* | HQ | `PR.status = APPROVED` |
| 2 | → `ISSUED` | HQ | Sets all fields at creation: `id`, `trigger_type=PR_TRIGGERED`, `creator_node_id`, `supplier_id`, `destination_node_id` *(= PR.requester_node_id)*, `linked_pr_id`, `items[].item_id`, `items[].qty_ordered`, `items[].unit_price`, `total_amount`. Also sets `PR.linked_puro_id = PurO.id`. **Price attached at creation** (HQ already has supplier quote from PR review). Skips `DRAFT`. |
| 3 | `ISSUED` → `IN_TRANSIT` | HQ / Supplier | No new PurO fields set. |
| 4 | `IN_TRANSIT` → `DELIVERED` | Store / Factory | Destination node creates `GoodsReceipt` (`source_ref_id = PurO.id`, `qty_received`). |
| 5 | `DELIVERED` → `PAYMENT_SETTLED` | HQ | 3-Way Matching validated. Payment authorized. → Triggers `Asset` creation (see §5.3). |

> **Key rule — When is price attached to a PurO?** For `AUTO_DRAFT`: at **Step 3** when HQ reviews the draft. For `PR_TRIGGERED`: at **Step 2** at PurO creation.

---

## 5. Execution Logistics (GI & GR)

### GoodsIssue (GI)
Created by the Provider node to dispatch goods. Triggers a **Stock Out** at the issuer node.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `source_ref_type` | string | `ITO` \| `B2B_SALES_ORDER` — Discriminator for the FK below |
| `source_ref_id` | string | FK → `InternalTransferOrder` OR `B2BSalesOrder` |
| `issuer_node_id` | string | FK → `Node` (the node dispatching goods) |
| `is_same_site` | bool | `true` if `issuer` and `receiver` share the same `site_id`. Skips driver info, media proof, and shipping fee requirements. |
| `driver_name` | string? | Logistics provider name — Required when `is_same_site = false` |
| `driver_phone` | string? | Contact number — Required when `is_same_site = false` |
| `shipping_fee` | float64 | Cost of transport (allocated to OpEx). `0` when `is_same_site = true`. |
| `media_proof_urls` | []string | Image/video URLs of packed goods — Required when `is_same_site = false` |
| `qty_dispatched` | float64 | Actual quantity sent (in Packaging Units) |
| `status` | GIStatus | `PENDING` → `CONFIRMED` |

---

### GoodsReceipt (GR)
Created by the Destination node upon receiving goods. Triggers a **Stock In** at the receiver node.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `source_ref_type` | string | `ITO` \| `PURCHASE_ORDER` — Discriminator for the FK below |
| `source_ref_id` | string | FK → `InternalTransferOrder` OR `PurchaseOrder` |
| `linked_gi_id` | string? | FK → `GoodsIssue` — Populated for ITO-linked GRs. Used for discrepancy detection: if `GI.qty_dispatched > GR.qty_received`, a `DiscrepancyTicket` is auto-generated. `null` for PurO-linked GRs (supplier delivers directly). |
| `receiver_node_id` | string | FK → `Node` |
| `media_proof_urls` | []string | Image URLs — Required when a discrepancy is detected |
| `qty_received` | float64 | Actual usable quantity received (in Packaging Units; system converts to Base Units for stock) |
| `has_discrepancy` | bool | **Computed.** `true` when `linked_gi_id` is set AND `linked_gi.qty_dispatched > qty_received` |

---

### Asset
The financial and administrative record of a CapEx equipment item, registered after procurement. **Created by HQ upon `PAYMENT_SETTLED` for a `PR_TRIGGERED` PurchaseOrder.** Serves as the bridge between the Supply Chain domain and the Production domain (`Machine`).

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `name` | string | Equipment name — copied from `PR.item_name` |
| `node_id` | string | FK → `Node` — Where the asset is physically located and used |
| `linked_pr_id` | string | FK → `PurchaseRequisition` — The original request |
| `linked_puro_id` | string | FK → `PurchaseOrder` — The order that procured it |
| `linked_gr_id` | string | FK → `GoodsReceipt` — Proof of delivery |
| `acquisition_cost` | float64 | Actual cost paid — sourced from `PurO.total_amount` |
| `acquisition_date` | date | Date `GoodsReceipt` was confirmed |
| `status` | AssetStatus | See status flow below |
| `linked_machine_id` | string? | FK → `Machine` (Production domain) — Populated when HQ/Factory registers this asset into the production system. `null` until registration is complete. |
| `depreciation_method` | string | `STRAIGHT_LINE` \| `DECLINING_BALANCE` |
| `useful_life_years` | int | Expected lifespan — used for depreciation schedule |
| `current_book_value` | float64 | Computed. Decremented periodically by system based on depreciation schedule. |

**AssetStatus Flow:**
```
PENDING_REGISTRATION  →  IN_USE            (once linked_machine_id is set by HQ/Factory)
IN_USE                →  UNDER_MAINTENANCE  →  IN_USE
                      →  DECOMMISSIONED     (triggers replacement PR cycle)
```

**Rules:**
- Only `PR_TRIGGERED` PurOs result in an `Asset` record. `AUTO_DRAFT` PurOs procure inventory items (OpEx) — not assets.
- `linked_machine_id` is set when the Factory/HQ registers the physical equipment as a `Machine` in the Production system. At this point `Asset.status` transitions from `PENDING_REGISTRATION` → `IN_USE`.
- When `Asset.status → UNDER_MAINTENANCE`, the linked `Machine.status` is synchronized to `UNDER_MAINTENANCE`.
- When `Asset.status → DECOMMISSIONED`, the linked `Machine.status` is set to `DECOMMISSIONED` and excluded from batch allocation. A new PR cycle must begin for any replacement equipment.

**Full PR → Machine Registration Workflow:**

| Step | Actor | Action | Fields Written |
|---|---|---|---|
| 1 | Store / Factory Manager | Submits `PurchaseRequisition` | `requester_node_id`, `item_name`, `estimated_cost`, `justification`; `PR.status = PENDING_HQ_APPROVAL` |
| 2 | HQ | Approves PR | `PR.status = APPROVED` |
| 3 | HQ | Creates `PurchaseOrder` from PR | `trigger_type=PR_TRIGGERED`, `supplier_id`, `destination_node_id`, `linked_pr_id`, `items[]`, `unit_price`, `total_amount`; `PR.linked_puro_id = PurO.id`; `PurO.status = ISSUED` |
| 4 | Supplier | Ships goods to destination node | `PurO.status = IN_TRANSIT` |
| 5 | Store / Factory | Creates `GoodsReceipt` | `source_ref_type=PURCHASE_ORDER`, `source_ref_id=PurO.id`, `receiver_node_id`, `qty_received`; `PurO.status = DELIVERED` |
| 6 | HQ | 3-Way Matching → settles payment | `PurO.status = PAYMENT_SETTLED` |
| 7 | System | **Auto-creates `Asset` record** | `name=PR.item_name`, `node_id=PR.requester_node_id`, `linked_pr_id`, `linked_puro_id`, `linked_gr_id`, `acquisition_cost=PurO.total_amount`, `acquisition_date=GR.confirmed_at`; `Asset.status = PENDING_REGISTRATION`; `Asset.linked_machine_id = null` |
| 8 | Store / Factory | **Creates `Machine` record** for the node | `Machine.id` (e.g. `M_FRYER_03`), `Machine.station_type_id` (e.g. `FRYER`), `Machine.node_id = Asset.node_id`, `Machine.max_slots`, `Machine.status = IDLE`, `Machine.linked_asset_id = Asset.id` |
| 9 | System | **Links Asset ↔ Machine** | `Asset.linked_machine_id = Machine.id`; `Asset.status = IN_USE` |

---

## 6. Exception & B2B

### DiscrepancyTicket
Auto-generated when `GoodsIssue.qty_dispatched > GoodsReceipt.qty_received` on an ITO delivery.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `source_ref_type` | string | `ITO` (current scope) |
| `source_ref_id` | string | FK → `InternalTransferOrder` that triggered the discrepancy |
| `gi_id` | string | FK → `GoodsIssue` |
| `gr_id` | string | FK → `GoodsReceipt` |
| `missing_qty` | float64 | `GI.qty_dispatched - GR.qty_received` |
| `loss_cost_value` | float64 | Monetary value of the lost/damaged items |
| `status` | TicketStatus | `OPEN` → `RESOLVED_VENDOR_CLAIM` \| `RESOLVED_INTERNAL_LOSS` |

---

### B2BSalesOrder
For wholesale fulfillment to external customers. Initiated by HQ, executed by Factory. No `GoodsReceipt` — closed via Proof of Delivery.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `external_customer_id` | string | FK → `Customer` |
| `fulfiller_node_id` | string | FK → `Node` (Factory that will pack and ship) |
| `revenue_amount` | float64 | Selling price recognized by HQ |
| `cogs_amount` | float64 | Cost of Goods Sold — derived from the Factory's `GoodsIssue.qty_dispatched × unit_cost` |
| `status` | B2BStatus | `PENDING_FULFILLMENT` → `IN_TRANSIT` → `DELIVERED` |

*Note: `DELIVERED` is triggered by Proof of Delivery from the logistics provider. No system GR is created since the receiver is an external customer.*
