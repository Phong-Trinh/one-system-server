package models

import "time"

// ─── Node / Org ───────────────────────────────────────────────────────────────

type NodeType string

const (
	NodeHQ      NodeType = "HQ"
	NodeStore   NodeType = "STORE"
	NodeFactory NodeType = "FACTORY"
)

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Node is a physical or logical operational location within a Tenant (ORG).
// SiteID groups nodes that are co-located in the same physical building.
// When RequesterNode.SiteID == ProviderNode.SiteID, the system uses the
// 1-click in-site transfer path (auto GI + GR, no IN_TRANSIT phase).
type Node struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"` // FK → Organization
	Type      NodeType  `json:"type"`   // HQ | STORE | FACTORY
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	SiteID    *string   `json:"site_id"` // Optional: groups co-located nodes for in-site transfers
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── Station / Machine ────────────────────────────────────────────────────────

// EquipmentType defines a category of kitchen equipment (e.g., FRYER, OVEN, GRILL).
type EquipmentType struct {
	ID           string `json:"id"`            // Enum-style key: "FRYER", "OVEN", "GRILL", etc.
	Name         string `json:"name"`          // Human-readable display label
	CapacityUnit string `json:"capacity_unit"` // Unit of capacity measurement: "slots", "liters", "trays", etc.
}

// MachineStatus represents the lifecycle state of a physical machine.
// IDLE/BUSY are managed exclusively by the batch allocation engine.
// UNDER_MAINTENANCE/DECOMMISSIONED are driven by the Asset lifecycle (Supply Chain domain).
type MachineStatus string

const (
	// MachineIdle — machine is available for the bin-packing engine to assign batches.
	MachineIdle MachineStatus = "IDLE"
	// MachineBusy — machine is currently executing a ProductionBatch. Set by the batch engine.
	MachineBusy MachineStatus = "BUSY"
	// MachineUnderMaintenance — machine is temporarily out of service. Set by the Asset lifecycle.
	MachineUnderMaintenance MachineStatus = "UNDER_MAINTENANCE"
	// MachineDecommissioned — machine is permanently retired. Excluded from bin-packing.
	// Cannot accept new ProductionBatch entries.
	MachineDecommissioned MachineStatus = "DECOMMISSIONED"
)

// Machine is a specific physical machine instance at a node.
type Machine struct {
	ID             string        `json:"id"`              // e.g., "M_FRYER_01", "M_OVEN_02"
	EquipmentTypeID  string        `json:"equipment_type_id"` // FK → EquipmentType
	NodeID         string        `json:"node_id"`         // FK → Node
	MaxCapacity    float64       `json:"max_capacity"`    // Total capacity in EquipmentType.capacity_unit (e.g., 6.0 liters)
	Status         MachineStatus `json:"status"`
	CurrentBatchID *string       `json:"current_batch_id,omitempty"` // FK → ProductionBatch (null when IDLE)
	// LinkedAssetID is populated when this machine was procured via PR → PurO → GR.
	// Null for pre-existing or manually registered machines.
	LinkedAssetID *string `json:"linked_asset_id,omitempty"` // FK → Asset (Supply Chain domain)
}

// ─── Staff ────────────────────────────────────────────────────────────────────

// Staff is a staff member working at a node, used for production labor costing.
type Staff struct {
	ID       string  `json:"id"`
	NodeID   string  `json:"node_id"` // FK → Node
	Name     string  `json:"name"`
	WageRate float64 `json:"wage_rate"` // Hourly wage rate in base currency
}
