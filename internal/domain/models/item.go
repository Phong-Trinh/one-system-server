package models

type ItemCategory string

const (
    CatRawMaterial ItemCategory = "RAW_MATERIAL"
    CatPostProduct ItemCategory = "POST_PRODUCT"
    CatProduct     ItemCategory = "PRODUCT"
)

type Item struct {
    ID       string       `json:"id"`
    Name     string       `json:"name"`
    SKU      string       `json:"sku"`
    Category ItemCategory `json:"category"`
    Price    float64      `json:"price"`
    Unit     string       `json:"unit"`
}

type Recipe struct {
    ID            string       `json:"id"`
    TargetItemID  string       `json:"target_item_id"`
    Ingredients   []Ingredient `json:"ingredients"`
    Steps         []string     `json:"steps"`
}

type Ingredient struct {
    ItemID   string  `json:"item_id"`
    Quantity float64 `json:"quantity"`
    Unit     string  `json:"unit"`
}
