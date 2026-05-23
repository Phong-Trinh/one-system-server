# OneSystem — Supply Chain Domain Models
**Version:** 2.0
**Status:** DRAFT

This document describes the domain models, fields, and relationships required to support the new 3-tier Supply Chain architecture (InternalTransferOrder, PR, PurO) and the In-house / Single-Node workflows.

---

## 1. Core Config & Virtual Nodes (Strict Typology)

To ensure strict financial accounting (Cost Center vs Profit Center) and clear inventory accountability, OneSystem uses a **Strict Typology** for nodes. The Capability Matrix has been abandoned in favor of Virtual Nodes grouped by `site_id`.

### Node
A physical or virtual operational location. 

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `type` | NodeType | `HQ` \| `STORE` \| `FACTORY` |
| `site_id` | string | Identifier grouping nodes that share the same physical address |

*Rule:* If a client operates a Hybrid Location (e.g., Bakery + Storefront in one building), they MUST create two virtual nodes sharing the same `site_id` (e.g., F1 and S1). This ensures 2 separate inventory ledgers. 

### Local Auto-Transfer (UX Optimization)
When an `InternalTransferOrder` is created between two nodes where `Requester.site_id == Provider.site_id` (e.g., F1 to S1):
- The `GoodsIssue` (GI) UI is simplified to a 1-click **"Move to Store"** button.
- It requires NO driver info, NO media proof, and NO shipping fee.
- It automatically creates and confirms the `GoodsReceipt` (GR) at the destination node, ensuring seamless UX for staff in the same building.

---

## 2. Internal Logistics (OpEx)

### InternalTransferOrder (ITO)
Replaces the old `SupplyRequest`. Used exclusively for moving goods between internal nodes (e.g., F to S, or S to S).

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `type` | ITOType | `AUTO_REPLENISHMENT` \| `MANUAL_REQUEST` |
| `requester_node_id` | string | FK → `Node` (who needs the goods) |
| `provider_node_id` | string | FK → `Node` (who supplies the goods) |
| `item_id` | string | FK → `Item` |
| `qty_requested` | float64 | Amount requested (in Base Unit or Packaging Unit) |
| `status` | ITOStatus | `PENDING_APPROVAL` → `AUTO_APPROVED` → `IN_TRANSIT` → `COMPLETED` |

---

## 3. External Procurement

### PurchaseRequisition (PR)
Used for CapEx (Assets/Equipment) or exceptional requests from a Store/Factory to HQ. 

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `requester_node_id` | string | FK → `Node` |
| `item_name` | string | Name of requested equipment/tool |
| `estimated_cost` | float64 | Expected cost |
| `justification` | string | Why is this needed? (e.g., "Old fryer broken") |
| `status` | PRStatus | `PENDING_HQ_APPROVAL` → `APPROVED` → `REJECTED` |
| `linked_puro_id` | string? | FK → `PurchaseOrder` (populated if approved and bought) |

### PurchaseOrder (PurO)
The official order sent to an External Supplier. Only created by nodes with `can_procure_external = true`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `creator_node_id` | string | FK → `Node` (usually HQ) |
| `supplier_id` | string | FK → `Supplier` (External vendor) |
| `destination_node_id` | string | FK → `Node` (Where the supplier must deliver) |
| `total_amount` | float64 | Total value of order |
| `status` | PurOStatus | `ISSUED` → `DELIVERED` → `PAYMENT_SETTLED` |

---

## 4. Execution Logistics (GI & GR)

### GoodsIssue (GI)
Created by the Provider node to dispatch goods. Triggers a **Stock Out**.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `source_ref_id` | string | FK → `InternalTransferOrder` OR `B2BSalesOrder` |
| `issuer_node_id` | string | FK → `Node` (the factory/warehouse issuing goods) |
| `driver_name` | string | Logistics provider info |
| `driver_phone` | string | Contact number |
| `shipping_fee` | float64 | Cost of transport (added to OpEx) |
| `media_proof_urls` | []string | Array of image/video URLs of the packed goods |
| `qty_dispatched` | float64 | Actual quantity sent |

### GoodsReceipt (GR)
Created by the Destination node to receive goods. Triggers a **Stock In**.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `source_ref_id` | string | FK → `InternalTransferOrder` OR `PurchaseOrder` |
| `receiver_node_id` | string | FK → `Node` |
| `media_proof_urls` | []string | Array of image URLs (mandatory if discrepancy) |
| `qty_received` | float64 | Actual usable quantity received |

---

## 5. Exception & B2B

### DiscrepancyTicket
Auto-generated when `GI.qty_dispatched` > `GR.qty_received`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `internal_transfer_id` | string | FK → `InternalTransferOrder` |
| `missing_qty` | float64 | Difference between GI and GR |
| `loss_cost_value` | float64 | Monetary value of the lost items |
| `status` | TicketStatus | `OPEN` (Requires HQ review) → `RESOLVED_VENDOR_CLAIM` → `RESOLVED_INTERNAL_LOSS` |

### B2BSalesOrder
For wholesale external customers. Fulfills via Factory but skips `GoodsReceipt`.

| Field | Type | Description |
|---|---|---|
| `id` | string | Unique identifier |
| `external_customer_id`| string | FK → `Customer` |
| `fulfiller_node_id` | string | FK → `Node` (Factory that will pack/ship) |
| `revenue_amount` | float64 | Selling price |
| `cogs_amount` | float64 | Cost of Goods Sold (derived from Factory's GI) |
| `status` | B2BStatus | `PENDING_FULFILLMENT` → `DELIVERED` (Closed via Proof of Delivery) |
