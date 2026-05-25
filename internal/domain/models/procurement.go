package models

import "time"

// ─── Asset ────────────────────────────────────────────────────────────────────

// AssetStatus mirrors the MachineStatus lifecycle for equipment assets.
// After an asset is DECOMMISSIONED, any Machine linked via Machine.linked_asset_id
// must also be transitioned to DECOMMISSIONED and removed from bin-packing.
type AssetStatus string

const (
	// AssetActive — asset is in service.
	AssetActive AssetStatus = "ACTIVE"
	// AssetUnderMaintenance — asset is temporarily out of service for repair.
	// Synced to Machine.status = UNDER_MAINTENANCE for equipment assets.
	AssetUnderMaintenance AssetStatus = "UNDER_MAINTENANCE"
	// AssetDecommissioned — asset is permanently retired.
	// Synced to Machine.status = DECOMMISSIONED for equipment assets.
	AssetDecommissioned AssetStatus = "DECOMMISSIONED"
)

// Asset is an organizational asset record created automatically after a CapEx procurement cycle
// completes: PurchaseRequisition (PR) → PurchaseOrder (PO) → GoodsReceipt (GR) → 3-Way Matching.
//
// Per §2.3 of the Supply Chain doc:
//   - Created and locked by HQ after 3-Way Matching is settled.
//   - Value is NOT expensed immediately; it is depreciated over its useful life.
//   - After creation, a node manager registers it as a Machine in the production system.
//   - Machine.linked_asset_id points back to this record, tracing the full PR → PO → GR provenance.
type Asset struct {
	ID     string `json:"id"`
	OrgID  string `json:"org_id"`  // FK → Organization
	NodeID string `json:"node_id"` // FK → Node (the node that owns this asset)
	EquipmentTypeID string `json:"equipment_type_id"` // FK → EquipmentType
	// Provenance chain — traces the full procurement lifecycle.
	PRID *string `json:"pr_id,omitempty"` // FK → PurchaseRequisition (nil if no PR was raised)
	POID string  `json:"po_id"`           // FK → PurchaseOrder
	GRID string  `json:"gr_id"`           // FK → GoodsReceipt (the confirming receipt)
	// Financial fields — set at acquisition time and used for depreciation.
	PurchaseValue    float64     `json:"purchase_value"`     // Total acquisition cost (from PO + GR)
	DepreciationRate float64     `json:"depreciation_rate"`  // Annual depreciation rate (e.g., 0.20 = 20%)
	UsefulLifeMonths int         `json:"useful_life_months"` // Expected useful life in months
	Status           AssetStatus `json:"status"`
	AcquiredAt       time.Time   `json:"acquired_at"` // Date the GR was confirmed
	DecommissionedAt *time.Time  `json:"decommissioned_at,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}
