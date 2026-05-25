package models

// ─── Item ─────────────────────────────────────────────────────────────────────

type ItemType string

const (
	ItemTypeProduct     ItemType = "PRODUCT"
	ItemTypeSemiProduct ItemType = "SEMI_PRODUCT"
	ItemTypeRawMaterial ItemType = "RAW_MATERIAL"
	ItemTypeAssetSupply ItemType = "ASSET_SUPPLY"
)

// Item is the base entity for all physical goods in the system.
// All internal stock levels, BOM quantities, and kitchen consumption are tracked in BaseUnit.
type Item struct {
	ID       string   `json:"id"`
	OrgID    string   `json:"org_id"` // FK → Organization (HQ defines the master catalog)
	Name     string   `json:"name"`
	SKU      string   `json:"sku"`
	Type     ItemType `json:"type"`      // PRODUCT | SEMI_PRODUCT | RAW_MATERIAL | ASSET_SUPPLY
	BaseUnit string   `json:"base_unit"` // Smallest consumable unit: "piece", "ml", "gram", etc.
}

// ─── Unit of Measure ──────────────────────────────────────────────────────────

// UoM defines a packaging/ordering unit for an item and its conversion to base units.
// Staff work in familiar packaging quantities when ordering/receiving; the system internally
// maintains accurate base-unit stock counts.
//
// Examples:
//
//	Burger bun  — pkg_unit="bag",    conversion=10    (1 bag = 10 pieces)
//	Soft drink  — pkg_unit="case",   conversion=24    (1 case = 24 cans)
//	Cooking oil — pkg_unit="bottle", conversion=1000  (1 bottle = 1,000 ml)
type UoM struct {
	ID         string  `json:"id"`
	ItemID     string  `json:"item_id"`    // FK → Item
	PkgUnit    string  `json:"pkg_unit"`   // e.g., "bag", "case", "bottle"
	Conversion float64 `json:"conversion"` // How many base units are in one pkg_unit
}

// ─── Item Capacity Config ─────────────────────────────────────────────────────

// ItemCapacityConfig is the critical linking table between an Item and a StationType.
// For each (Item × StationType) combination, it defines:
//   - how much machine capacity one base unit of that item consumes, and
//   - whether that item tolerates sharing a machine with other item types in the same cycle.
//
// This is the primary input to the bin-packing algorithm in the batch allocation engine.
//
// Examples:
//
//	Burger bun  × OVEN  → slot_consumption=1,   allow_mix=true   (can share oven with other buns)
//	Egg         × FRYER → slot_consumption=1L,  allow_mix=false  (needs exclusive fryer cycle)
//	Potato      × FRYER → slot_consumption=2L,  allow_mix=false  (needs exclusive fryer cycle)
type ItemCapacityConfig struct {
	ItemID        string `json:"item_id"`         // FK → Item
	StationTypeID string `json:"station_type_id"` // FK → StationType
	// SlotConsumption is the capacity units consumed per one base unit of the item.
	// The unit corresponds to StationType.capacity_unit (e.g., slots, liters, trays).
	SlotConsumption float64 `json:"slot_consumption"`
	// AllowMix controls whether this item may share a machine batch with other item types.
	// If false, the item requires exclusive machine use for its entire cook cycle.
	// Enforced at batch creation time by the bin-packing engine.
	AllowMix bool `json:"allow_mix"`
}
