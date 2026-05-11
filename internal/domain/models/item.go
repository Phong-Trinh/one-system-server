package models

// ─── Item ─────────────────────────────────────────────────────────────────────

type ItemType string

const (
	ItemTypeProduct      ItemType = "PRODUCT"
	ItemTypeSemiProduct  ItemType = "SEMI_PRODUCT"
	ItemTypeRawMaterial  ItemType = "RAW_MATERIAL"
	ItemTypeAssetSupply  ItemType = "ASSET_SUPPLY"
)

// Item is the base entity for all physical goods in the system.
type Item struct {
	ID       string   `json:"id"`
	OrgID    string   `json:"org_id"`   // FK → Organization (HQ defines the master catalog)
	Name     string   `json:"name"`
	SKU      string   `json:"sku"`
	Type     ItemType `json:"type"`      // PRODUCT | SEMI_PRODUCT | RAW_MATERIAL | ASSET_SUPPLY
	BaseUnit string   `json:"base_unit"` // Smallest consumable unit: "piece", "ml", "gram", etc.
}

// ─── Unit of Measure ──────────────────────────────────────────────────────────

// UoM defines a packaging/ordering unit for an item and its conversion to base units.
// Example: Burger bun — pkg_unit="bag", conversion=10 (1 bag = 10 pieces)
type UoM struct {
	ItemID     string  `json:"item_id"`    // FK → Item
	PkgUnit    string  `json:"pkg_unit"`   // e.g., "bag", "case", "bottle"
	Conversion float64 `json:"conversion"` // How many base units in one pkg_unit
}

// ─── Item Capacity Config ─────────────────────────────────────────────────────

// ItemCapacityConfig links an Item to a StationType and defines slot consumption
// and mixing rules, feeding directly into the bin-packing allocation engine.
type ItemCapacityConfig struct {
	ItemID          string  `json:"item_id"`          // FK → Item
	StationTypeID   string  `json:"station_type_id"`  // FK → StationType
	SlotConsumption float64 `json:"slot_consumption"` // Capacity units per 1 base unit of item
	AllowMix        bool    `json:"allow_mix"`        // false = exclusive machine use during cycle
}
