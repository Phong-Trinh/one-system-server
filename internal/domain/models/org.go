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
type Node struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Type      NodeType  `json:"type"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ─── Station / Machine ────────────────────────────────────────────────────────

// StationType defines a category of kitchen equipment (e.g., FRYER, OVEN, GRILL).
type StationType struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	CapacityUnit    string             `json:"capacity_unit"`
	DefaultStrategy AllocationStrategy `json:"default_strategy"`
}

type MachineStatus string

const (
	MachineIdle MachineStatus = "IDLE"
	MachineBusy MachineStatus = "BUSY" // Used mainly for BATCH_SYNC or when at Threshold
)

type AllocationStrategy string

const (
	StrategySync  AllocationStrategy = "BATCH_SYNC"  // Group all, start together (Fryers/Ovens)
	StrategyAsync AllocationStrategy = "SLOT_ASYNC" // Allow overlapping starts (Grills/Open surfaces)
)

// Machine is a specific physical machine instance at a node.
type Machine struct {
	ID                   string             `json:"id"`                    // e.g., "M_FRYER_01"
	StationTypeID        string             `json:"station_type_id"`       // FK → StationType
	NodeID               string             `json:"node_id"`               // FK → Node
	MaxCapacity          float64            `json:"max_capacity"`          // Physical limit (e.g., 6.0 liters or 4.0 slots)
	AllocationStrategy   AllocationStrategy `json:"allocation_strategy"`
	Status               MachineStatus      `json:"status"` // IDLE | BUSY
	CurrentBatchID       *string            `json:"current_batch_id,omitempty"`
}


// ─── Staff ────────────────────────────────────────────────────────────────────

// Staff is a staff member working at a node, used for production labor costing.
type Staff struct {
	ID       string  `json:"id"`
	NodeID   string  `json:"node_id"`   // FK → Node
	Name     string  `json:"name"`
	WageRate float64 `json:"wage_rate"` // Hourly wage rate in base currency
}
