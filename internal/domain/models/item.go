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
