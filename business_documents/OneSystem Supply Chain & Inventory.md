# OneSystem Supply Chain & Inventory
**Version:** 2.0
**Status:** DRAFT

This document details the supply chain workflows, procurement types, and inventory management mechanics for the OneSystem platform. 

## 1. Procurement & Logistics Entities

The system strictly divides supply chain operations into three distinct flows based on the source of goods and the nature of the expense (OpEx vs CapEx).

### 1.1 Internal Transfer Order (Auto-Replenishment / S.RO / F.SO)
*Goal: Managing recurring internal stock replenishment (OpEx) between system nodes without external procurement.*

- **Trigger**: 
  - *Primary (Automatic)*: System generates an `InternalTransferOrder` when a Store's item stock drops below its Reorder Point (ROP).
  - *Secondary (Manual)*: Store Manager manually creates an order anticipating high demand (e.g., events, holidays) or to correct unexpected stock loss.
- **Approval**: `AUTO_APPROVED` by default for system triggers. Manual requests can be configured to auto-approve, or route to `PENDING_APPROVAL` (reviewed by Factory Manager/Area Manager) to prevent stock hoarding.
- **Participants**: Store (Requester) ↔ Factory/Other Store (Provider).
- **Logistics Flow (Cross-Site)**:
  1. **Goods Issue (GI)**: Provider (e.g., Factory) packs items, creates a GI capturing driver info, photo/video evidence, and shipping fee. **Triggers Stock Out.**
  2. **In Transit**: Goods move to the requester.
  3. **Goods Receipt (GR)**: Requester (Store) receives items. If there is a quantity mismatch (transit damage), the Store only checks in the usable quantity. **Triggers Stock In.**
  4. **Discrepancy Handling**: Any missing/damaged quantity automatically generates a `DiscrepancyTicket` sent to HQ for accounting and logistics resolution.
- **Logistics Flow (Same-Site / In-Site Transfer)**:
  - *Condition*: When `Requester.site_id == Provider.site_id` (e.g., a Factory and Store operating in the same building).
  - *1-Click Transfer*: Provider uses a simplified "Move to Store" action.
  - *Auto GI & GR*: System generates a simplified GI (no driver, no media proof, zero shipping fee) and immediately triggers and confirms a GR at the destination node.
  - *Result*: Instant **Stock Out** at Provider and **Stock In** at Requester, bypassing the `IN_TRANSIT` phase entirely for a seamless UX.

### 1.2 Purchase Requisition (PR)
*Goal: Formal request for CapEx items (equipment, tools) or exceptional purchases.*

- **Trigger**: ALWAYS manual.
- **Approval**: Requires HQ approval.
- **Participants**: Store/Factory (Requester) ➔ HQ (Approver).
- **Rules**:
  - A PR is just a request; it does not directly trigger a supplier shipment or stock change.
  - Used for long-term assets that undergo depreciation (e.g., grills, POS systems).
  - If approved, HQ translates the PR into an external Purchase Order (`HQ.PurO`).

### 1.3 Purchase Order (HQ.PurO)
*Goal: External procurement from third-party suppliers for both CapEx (equipment) and OpEx (raw materials).*

- **Trigger**: 
  - *Manual*: HQ generates a PurO to fulfill an approved Purchase Requisition (PR).
  - *System Alert*: When a Factory's raw material stock drops below its Reorder Point (ROP), the system sends a **"Low Stock Alert"** to HQ, prompting HQ to issue a PurO.
- **Authority**: ONLY HQ can issue a `PurO`. Stores and Factories **never** buy directly from suppliers.
- **Logistics Flow**:
  1. HQ issues `PurO` to Supplier.
  2. Supplier delivers goods to the specified destination (Factory or Store).
  3. Destination node creates a `Goods Receipt (GR)` linked directly to the `PurO`.
  4. **3-Way Matching**: HQ validates `PurO` + `Invoice` + `Goods Receipt` before authorizing supplier payment.

### 1.4 B2B Sales Order (Wholesale Fulfillment)
*Goal: Fulfilling large wholesale orders to external customers directly from the Factory.*

- **Trigger**: HQ Sales Team creates a B2B Sales Order based on an external client's request.
- **Authority**: HQ negotiates the sale. Factory only executes the fulfillment.
- **Participants**: HQ (Seller) ➔ Factory (Fulfiller) ➔ External Wholesale Customer (Receiver).
- **Logistics Flow**:
  1. HQ assigns the approved B2B Sales Order to a Factory.
  2. **Goods Issue (GI)**: Factory packs items, creates a GI capturing driver info and media proof. **Triggers Stock Out.**
  3. **In Transit**: Goods move to the external customer.
  4. **Completion**: Since the customer is external, there is NO system `Goods Receipt (GR)`. The order is marked `COMPLETED` upon Proof of Delivery from the logistics provider.
  5. **Accounting**: HQ recognizes Revenue and uses the Factory's GI value as Cost of Goods Sold (COGS).

## 2. Inventory & Stock Management

### 2.1 Reorder Point (ROP) & Safety Stock

- Each OpEx item at each node has a configurable **Reorder Point (ROP)**.
- ROP Calculation: `(Daily consumption × Lead time) + Safety stock`.
- **System Action upon hitting ROP** depends on the sourcing strategy of the node:
  - **Internal Replenishment (e.g., Store)**: The system **automatically triggers an `InternalTransferOrder`** routing the request to the designated internal provider (Factory or another Store).
  - **External Procurement (e.g., Factory Raw Materials)**: Because the Factory cannot "request" physical goods from HQ (HQ is not a warehouse), the system generates a **Low Stock Alert / Draft PurO** to the HQ Dashboard. HQ then issues an external `PurchaseOrder` to restock the Factory.

### 2.2 Unit of Measure (UoM) & Conversion

**Solution: Base Unit + Packaging Unit model with automatic conversion.**

Each item is defined with:
- A **Base Unit (BU)** — the smallest consumable unit. All internal stock levels, BOM quantities, and kitchen consumption are tracked in base units.
- One or more **Packaging Units (PU)** — the unit used for ordering, receiving, and dispatching between nodes. Each PU has a defined conversion ratio to the base unit.

| Item | Base Unit | Packaging Unit | Conversion |
|---|---|---|---|
| Burger bun | piece | bag | 1 bag = 10 pieces |
| Soft drink can | can | case | 1 case = 24 cans (4 blocks × 6) |
| Cooking oil | ml | bottle | 1 bottle = 1,000 ml |

**Rules:**
- **HQ.PurO to Supplier** — ordered in Packaging Units.
- **Goods Receipt (GR)** — received in Packaging Units; system converts to Base Units for stock.
- **Goods Issue (GI)** — dispatched in Packaging Units; system converts to Base Units on destination receipt.
- **BOM & kitchen consumption** — always in Base Units.
- **Minimum stock threshold / ROP** — configured and displayed in Base Units for precision.

This means staff always work in familiar packaging quantities when ordering/receiving, while the system maintains accurate base-unit stock counts internally.

### 2.3 Price Variance & Cost Allocation

- **OpEx Costing**: The value of goods received via internal transfer (Transfer Price) plus the shipping fee is immediately allocated to the Store's operational expenses (OpEx) for that period.
- **CapEx Costing**: Equipment bought via PR is placed in the Asset Register and undergoes gradual depreciation; it is NOT fully expensed in the current period.

---

## 3. Open Issues & Decisions Required

| # | Question | Status | Owner |
|---|---|---|---|
| 1 | What is the exact workflow for resolving a `DiscrepancyTicket` with third-party logistics providers (e.g., Lalamove/Ahamove)? | ⏳ Open | Business Owner |

-> when an S was run out of raw-material, which node would it request to ? F or HQ?

nếu bắt buộc tất cả các item của S đều phải được châm hàng từ F, vậy có nên định nghĩa trước các item sẽ cần filled cho S hay ko?
-> liên quan đến việc trigger tồn kho của F, vì raw-material vừa dùng để sản xuất ở F, vừa dùng để châm hàng cho S

-> phải làm cả 2 case: S vừa có thể được châm hàng từ F, vừa có thể được châm hàng từ HQ bằng cách nhận hàng từ suppliers -> vì mỗi hệ thống cửa hàng sẽ cấu hình khác nhau