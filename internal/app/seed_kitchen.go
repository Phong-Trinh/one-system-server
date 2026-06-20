package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// SeedKitchenData seeds all demo data required for the triple-pane KDS UI:
//   - Equipment types (ST_BEP_NUONG = Grill, ST_MAY_CHIEN = Fryer, ST_BAN_RAP = Assembly, ST_SO_CHE = Prep)
//   - Machines (M1_BEP_NUONG, M2_MAY_CHIEN_1, etc.) — IDs match app.js kdsMachines
//   - Items: raw materials, semi-products (Patty, Khoai Tây Cắt Sợi), and finished products
//   - BOMs with lines referencing the correct items
//   - SOPs with step DAGs; each SOPStep embeds slot_consumption + allow_mix
//     (the bin-packing inputs — there is no separate item_capacity_configs collection)
//
// All ops are idempotent — safe to call on every startup.
func SeedKitchenData(
	ctx context.Context,
	equipTypeRepo services.EquipmentTypeRepository,
	machineRepo services.MachineRepository,
	itemRepo services.ItemRepository,
	bomRepo services.BOMRepository,
	sopRepo services.SOPRepository,
	nodeItemConfigRepo services.NodeItemConfigRepository,
	supplierRepo services.SupplierRepository,
	stockRepo services.NodeStockRepository,
) error {
	const nodeID = "STORE"
	const orgID = "SNAPBITE_ORG"

	// ── 1. Equipment Types ────────────────────────────────────────────────────
	equipTypes := []models.EquipmentType{
		{ID: "ST_BEP_NUONG", Name: "Bếp Nướng", CapacityUnit: "slot"},
		{ID: "ST_MAY_CHIEN", Name: "Máy Chiên", CapacityUnit: "liter"},
		{ID: "ST_BAN_RAP", Name: "Bàn Ráp", CapacityUnit: "slot"},
		{ID: "ST_SO_CHE", Name: "Sơ Chế", CapacityUnit: "slot"},
	}
	for _, et := range equipTypes {
		_ = equipTypeRepo.Delete(ctx, et.ID)
		e := et
		if err := equipTypeRepo.Create(ctx, &e); err != nil {
			return fmt.Errorf("seed equipment type %s: %w", et.ID, err)
		}
		log.Info().Str("id", et.ID).Msg("[Seed] Created EquipmentType")
	}

	// ── 1.5 Suppliers ─────────────────────────────────────────────────────────
	suppliers := []models.Supplier{
		{ID: "SUP_VINMART", OrgID: orgID, Name: "VinMart Wholesale", ContactInfo: "0901234567", Address: "Hanoi, VN"},
		{ID: "SUP_MEAT_DELI", OrgID: orgID, Name: "MeatDeli Fresh", ContactInfo: "0909876543", Address: "Hanoi, VN"},
	}
	for _, sup := range suppliers {
		existing, _ := supplierRepo.FindByID(ctx, sup.ID)
		if existing == nil {
			if err := supplierRepo.Create(ctx, &sup); err != nil {
				return fmt.Errorf("seed Supplier: %w", err)
			}
			log.Info().Str("supplier_id", sup.ID).Msg("[Seed] Created Supplier")
		}
	}

	// ── 2. Machines ───────────────────────────────────────────────────────────
	machines := []models.Machine{
		// STORE Machines
		{ID: "M1_BEP_NUONG", EquipmentTypeID: "ST_BEP_NUONG", NodeID: "STORE", MaxCapacity: 8, Status: models.MachineIdle},
		{ID: "M2_MAY_CHIEN_1", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "STORE", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "M3_MAY_CHIEN_2", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "STORE", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "M4_BAN_RAP", EquipmentTypeID: "ST_BAN_RAP", NodeID: "STORE", MaxCapacity: 10, Status: models.MachineIdle},
		{ID: "M5_SO_CHE", EquipmentTypeID: "ST_SO_CHE", NodeID: "STORE", MaxCapacity: 10, Status: models.MachineIdle},
		// STORE2 Machines
		{ID: "S2_M1_BEP_NUONG", EquipmentTypeID: "ST_BEP_NUONG", NodeID: "STORE2", MaxCapacity: 8, Status: models.MachineIdle},
		{ID: "S2_M2_MAY_CHIEN_1", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "STORE2", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "S2_M3_MAY_CHIEN_2", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "STORE2", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "S2_M4_BAN_RAP", EquipmentTypeID: "ST_BAN_RAP", NodeID: "STORE2", MaxCapacity: 10, Status: models.MachineIdle},
		{ID: "S2_M5_SO_CHE", EquipmentTypeID: "ST_SO_CHE", NodeID: "STORE2", MaxCapacity: 10, Status: models.MachineIdle},
		// FACTORY Machines
		{ID: "F_M1_BEP_NUONG", EquipmentTypeID: "ST_BEP_NUONG", NodeID: "FACTORY", MaxCapacity: 80, Status: models.MachineIdle},
		{ID: "F_M2_MAY_CHIEN_1", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "FACTORY", MaxCapacity: 20, Status: models.MachineIdle},
		{ID: "F_M3_MAY_CHIEN_2", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "FACTORY", MaxCapacity: 20, Status: models.MachineIdle},
		{ID: "F_M4_BAN_RAP", EquipmentTypeID: "ST_BAN_RAP", NodeID: "FACTORY", MaxCapacity: 100, Status: models.MachineIdle},
		{ID: "F_M5_SO_CHE", EquipmentTypeID: "ST_SO_CHE", NodeID: "FACTORY", MaxCapacity: 50, Status: models.MachineIdle},
	}
	for _, m := range machines {
		_ = machineRepo.Delete(ctx, m.ID)
		mCopy := m
		if err := machineRepo.Create(ctx, &mCopy); err != nil {
			return fmt.Errorf("seed machine %s: %w", m.ID, err)
		}
		log.Info().Str("id", m.ID).Msg("[Seed] Created Machine")
	}

	// ── 3. Items ──────────────────────────────────────────────────────────────
	items := []models.Item{
		// Raw Materials
		{ID: "ITEM_BO_TUOI", OrgID: orgID, Name: "Thịt Bò Tươi", SKU: "RM-BO-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_GA_TUOI", OrgID: orgID, Name: "Thịt Gà Tươi", SKU: "RM-GA-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_BANH_MI", OrgID: orgID, Name: "Bánh Mì Tròn", SKU: "RM-BM-001", Type: models.ItemTypeRawMaterial, BaseUnit: "piece"},
		{ID: "ITEM_HANH_TAY", OrgID: orgID, Name: "Hành Tây", SKU: "RM-HT-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_KHOAI_TAY", OrgID: orgID, Name: "Khoai Tây Raw", SKU: "RM-KT-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_PHO_MAI", OrgID: orgID, Name: "Phô Mai Lát", SKU: "RM-PM-001", Type: models.ItemTypeRawMaterial, BaseUnit: "piece"},
		{ID: "ITEM_XA_LACH", OrgID: orgID, Name: "Xà Lách", SKU: "RM-XL-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		// Semi-Products
		{ID: "ITEM_PATTY_BO", OrgID: orgID, Name: "Patty Bò", SKU: "SP-PB-001", Type: models.ItemTypeSemiProduct, BaseUnit: "piece"},
		{ID: "ITEM_PATTY_GA", OrgID: orgID, Name: "Patty Gà", SKU: "SP-PG-001", Type: models.ItemTypeSemiProduct, BaseUnit: "piece"},
		{ID: "ITEM_KHOAI_TAY_CAT_SOI", OrgID: orgID, Name: "Khoai Tây Cắt Sợi", SKU: "SP-KTC-001", Type: models.ItemTypeSemiProduct, BaseUnit: "gram"},
		// Finished Products
		{ID: "ITEM_HAMBURGER_BO", OrgID: orgID, Name: "Hamburger Bò", SKU: "PRD-HB-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
		{ID: "ITEM_HAMBURGER_GA", OrgID: orgID, Name: "Hamburger Gà", SKU: "PRD-HG-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
		{ID: "ITEM_CHEESEBURGER", OrgID: orgID, Name: "Cheeseburger", SKU: "PRD-CB-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
		{ID: "ITEM_BIT_TET", OrgID: orgID, Name: "Bò Bít Tết", SKU: "PRD-BT-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
		{ID: "ITEM_KHOAI_TAY_CHIEN", OrgID: orgID, Name: "Khoai Tây Chiên", SKU: "PRD-KTC-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
	}
	for _, it := range items {
		_ = itemRepo.Delete(ctx, it.ID)
		item := it
		if err := itemRepo.Create(ctx, &item); err != nil {
			return fmt.Errorf("seed item %s: %w", it.ID, err)
		}
		log.Info().Str("id", it.ID).Msg("[Seed] Created Item")
	}

	// ── 4. BOMs + SOPs ───────────────────────────────────────────────────────
	// SOPStep now holds SlotConsumption and AllowMix configuration.
	type bomSeed struct {
		bomID        string
		outputItemID string
		lines        []struct {
			itemID string
			qty    float64
		}
		steps []models.SOPStep
	}

	equipPtr := func(id string) *string { return &id }

	seeds := []bomSeed{
		// SEMI PRODUCTS
		{
			bomID:        "BOM_PATTY_BO",
			outputItemID: "ITEM_PATTY_BO",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_BO_TUOI", 150},
			},
			steps: []models.SOPStep{
				{ID: "STEP_XAY_BO", Description: "Xay thịt bò và nặn", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 30, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_PATTY_BO_LINE_1"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		{
			bomID:        "BOM_PATTY_GA",
			outputItemID: "ITEM_PATTY_GA",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_GA_TUOI", 150},
			},
			steps: []models.SOPStep{
				{ID: "STEP_XAY_GA", Description: "Xay thịt gà và nặn", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 30, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_PATTY_GA_LINE_1"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		{
			bomID:        "BOM_KHOAI_TAY_CAT_SOI",
			outputItemID: "ITEM_KHOAI_TAY_CAT_SOI",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_KHOAI_TAY", 200},
			},
			steps: []models.SOPStep{
				{ID: "STEP_CAT_KHOAI", Description: "Cắt sợi khoai tây", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_KHOAI_TAY_CAT_SOI_LINE_1"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		// FINISHED PRODUCTS
		{
			bomID:        "BOM_HAMBURGER_BO",
			outputItemID: "ITEM_HAMBURGER_BO",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_PATTY_BO", 1},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
				{"ITEM_XA_LACH", 20},
			},
			steps: []models.SOPStep{
				{ID: "STEP_HB_NUONG_BO", Description: "Nướng Patty Bò", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 20, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_1"}, SlotConsumption: 2, AllowMix: true},
				{ID: "STEP_HB_NUONG_BANH", Description: "Nướng Bánh Mì", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_2"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HB_CHIEN_HANH", Description: "Chiên Hành", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_3"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HB_SO_CHE_RAU", Description: "Sơ Chế Rau", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_4"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HB_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 10, DependsOn: []string{"STEP_HB_NUONG_BO", "STEP_HB_NUONG_BANH", "STEP_HB_CHIEN_HANH", "STEP_HB_SO_CHE_RAU"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		{
			bomID:        "BOM_HAMBURGER_GA",
			outputItemID: "ITEM_HAMBURGER_GA",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_PATTY_GA", 1},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
				{"ITEM_XA_LACH", 20},
			},
			steps: []models.SOPStep{
				{ID: "STEP_HG_CHIEN_GA", Description: "Chiên Patty Gà", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 18, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_1"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HG_NUONG_BANH", Description: "Nướng Bánh Mì", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_2"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HG_CHIEN_HANH", Description: "Chiên Hành", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_3"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HG_SO_CHE_RAU", Description: "Sơ Chế Rau", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_4"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_HG_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 10, DependsOn: []string{"STEP_HG_CHIEN_GA", "STEP_HG_NUONG_BANH", "STEP_HG_CHIEN_HANH", "STEP_HG_SO_CHE_RAU"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		{
			bomID:        "BOM_CHEESEBURGER",
			outputItemID: "ITEM_CHEESEBURGER",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_PATTY_BO", 1},
				{"ITEM_PHO_MAI", 1},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
				{"ITEM_XA_LACH", 20},
			},
			steps: []models.SOPStep{
				{ID: "STEP_CB_NUONG_BO", Description: "Nướng Patty Bò", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 20, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_CHEESEBURGER_LINE_1"}, SlotConsumption: 2, AllowMix: true},
				{ID: "STEP_CB_NUONG_BANH", Description: "Nướng Bánh Mì", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_CHEESEBURGER_LINE_3"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_CB_CHIEN_HANH", Description: "Chiên Hành", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_CHEESEBURGER_LINE_4"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_CB_SO_CHE_RAU", Description: "Sơ Chế Rau", EquipmentTypeID: equipPtr("ST_SO_CHE"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_CHEESEBURGER_LINE_5"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_CB_SAP_XEP", Description: "Ráp Cheeseburger", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 12, DependsOn: []string{"STEP_CB_NUONG_BO", "STEP_CB_NUONG_BANH", "STEP_CB_CHIEN_HANH", "STEP_CB_SO_CHE_RAU"}, IngredientBOMLineIDs: []string{"BOM_CHEESEBURGER_LINE_2"}, SlotConsumption: 1, AllowMix: true},
			},
		},
		{
			bomID:        "BOM_BIT_TET",
			outputItemID: "ITEM_BIT_TET",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_BO_TUOI", 250},
			},
			steps: []models.SOPStep{
				{ID: "STEP_BT_NUONG", Description: "Nướng Bít Tết", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 25, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_BIT_TET_LINE_1"}, SlotConsumption: 3, AllowMix: false},
				{ID: "STEP_BT_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 12, DependsOn: []string{"STEP_BT_NUONG"}, SlotConsumption: 1, AllowMix: false},
			},
		},
		{
			bomID:        "BOM_KHOAI_TAY_CHIEN",
			outputItemID: "ITEM_KHOAI_TAY_CHIEN",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_KHOAI_TAY_CAT_SOI", 200},
			},
			steps: []models.SOPStep{
				{ID: "STEP_KC_CHIEN", Description: "Chiên Khoai Tây", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 20, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_KHOAI_TAY_CHIEN_LINE_1"}, SlotConsumption: 1, AllowMix: true},
				{ID: "STEP_KC_SAP_XEP", Description: "Lắc Muối & Ráp", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 8, DependsOn: []string{"STEP_KC_CHIEN"}, SlotConsumption: 1, AllowMix: true},
			},
		},
	}

	for _, seed := range seeds {
		sopID := "SOP_" + seed.bomID
		_ = sopRepo.DeleteStepsBySOPID(ctx, sopID) // cascade: clear steps before dropping SOP
		_ = bomRepo.Delete(ctx, seed.bomID)
		_ = sopRepo.Delete(ctx, sopID)

		if err := bomRepo.Create(ctx, &models.BOM{
			ID:           seed.bomID,
			OutputItemID: seed.outputItemID,
			Version:      1,
		}); err != nil {
			return fmt.Errorf("seed BOM %s: %w", seed.bomID, err)
		}

		for i, line := range seed.lines {
			if err := bomRepo.AddLine(ctx, &models.BOMLine{
				ID:     fmt.Sprintf("%s_LINE_%d", seed.bomID, i+1),
				BOMID:  seed.bomID,
				ItemID: line.itemID,
				Qty:    line.qty,
			}); err != nil {
				return fmt.Errorf("seed BOM line %d for %s: %w", i+1, seed.bomID, err)
			}
		}
		log.Info().Str("bom_id", seed.bomID).Int("lines", len(seed.lines)).Msg("[Seed] Created BOM")

		if err := sopRepo.Create(ctx, &models.SOP{
			ID:    sopID,
			BOMID: seed.bomID,
		}); err != nil {
			return fmt.Errorf("seed SOP for %s: %w", seed.bomID, err)
		}

		for i, step := range seed.steps {
			s := step
			s.SOPID = sopID
			s.SeqNo = i + 1
			if err := sopRepo.AddStep(ctx, &s); err != nil {
				return fmt.Errorf("seed SOP step %s: %w", step.ID, err)
			}
		}
		log.Info().Str("sop_id", sopID).Int("steps", len(seed.steps)).Msg("[Seed] Created SOP")
	}

	// ── 5. NodeItemConfig — ROP & Sourcing Strategy ───────────────────────────
	// STORE (Store): all items sourced via INTERNAL_TRANSFER from FACTORY.
	// FACTORY: raw materials sourced via EXTERNAL_PROCUREMENT.
	// Semi-products and finished products are produced on-site (no ROP config needed at Factory).
	factoryID := "FACTORY"

	type nicSeed struct {
		nodeID   string
		itemID   string
		strategy models.SourcingStrategy
		// providerNodeID set when strategy = INTERNAL_TRANSFER
		providerNodeID *string
		// supplierID set when strategy = EXTERNAL_PROCUREMENT
		supplierID   *string
		reorderPoint float64 // base units
		safetyStock  float64 // base units
		leadTimeDays int
	}

	strPtr := func(s string) *string { return &s }

	nicSeeds := []nicSeed{
		// ── STORE: Raw Materials (transferred from FACTORY) ───────────────
		{nodeID: "STORE", itemID: "ITEM_BO_TUOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_GA_TUOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_BANH_MI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 50, safetyStock: 10, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_HANH_TAY", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 1000, safetyStock: 200, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_KHOAI_TAY", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 3000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_PHO_MAI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 30, safetyStock: 10, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_XA_LACH", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 500, safetyStock: 100, leadTimeDays: 1},
		// ── STORE: Semi-Products (transferred from FACTORY) ───────────────
		{nodeID: "STORE", itemID: "ITEM_PATTY_BO", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 20, safetyStock: 5, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_PATTY_GA", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 20, safetyStock: 5, leadTimeDays: 1},
		{nodeID: "STORE", itemID: "ITEM_KHOAI_TAY_CAT_SOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		// ── STORE2: Raw Materials (transferred from FACTORY) ───────────────
		{nodeID: "STORE2", itemID: "ITEM_BO_TUOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_GA_TUOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_BANH_MI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 50, safetyStock: 10, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_HANH_TAY", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 1000, safetyStock: 200, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_KHOAI_TAY", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 3000, safetyStock: 500, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_PHO_MAI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 30, safetyStock: 10, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_XA_LACH", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 500, safetyStock: 100, leadTimeDays: 1},
		// ── STORE2: Semi-Products (transferred from FACTORY) ───────────────
		{nodeID: "STORE2", itemID: "ITEM_PATTY_BO", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 20, safetyStock: 5, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_PATTY_GA", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 20, safetyStock: 5, leadTimeDays: 1},
		{nodeID: "STORE2", itemID: "ITEM_KHOAI_TAY_CAT_SOI", strategy: models.SourcingInternalTransfer, providerNodeID: strPtr(factoryID), reorderPoint: 2000, safetyStock: 500, leadTimeDays: 1},
		// ── FACTORY: Raw Materials (purchased externally) ───────────────────────
		{nodeID: "FACTORY", itemID: "ITEM_BO_TUOI", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_MEAT_DELI"), reorderPoint: 20000, safetyStock: 5000, leadTimeDays: 2},
		{nodeID: "FACTORY", itemID: "ITEM_GA_TUOI", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_MEAT_DELI"), reorderPoint: 20000, safetyStock: 5000, leadTimeDays: 2},
		{nodeID: "FACTORY", itemID: "ITEM_BANH_MI", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_VINMART"), reorderPoint: 500, safetyStock: 100, leadTimeDays: 1},
		{nodeID: "FACTORY", itemID: "ITEM_HANH_TAY", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_VINMART"), reorderPoint: 10000, safetyStock: 2000, leadTimeDays: 2},
		{nodeID: "FACTORY", itemID: "ITEM_KHOAI_TAY", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_VINMART"), reorderPoint: 30000, safetyStock: 5000, leadTimeDays: 2},
		{nodeID: "FACTORY", itemID: "ITEM_PHO_MAI", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_VINMART"), reorderPoint: 300, safetyStock: 50, leadTimeDays: 3},
		{nodeID: "FACTORY", itemID: "ITEM_XA_LACH", strategy: models.SourcingExternalProcurement, supplierID: strPtr("SUP_VINMART"), reorderPoint: 5000, safetyStock: 1000, leadTimeDays: 1},
	}

	for _, s := range nicSeeds {
		cfg := &models.NodeItemConfig{
			ItemID:                s.itemID,
			NodeID:                s.nodeID,
			SourcingStrategy:      s.strategy,
			ProviderNodeID:        s.providerNodeID,
			SupplierID:            s.supplierID,
			ReorderPoint:          s.reorderPoint,
			SafetyStock:           s.safetyStock,
			SupplierLeadTimeDays:  s.leadTimeDays,
			ConsumptionWindowDays: 30,
		}
		if err := nodeItemConfigRepo.Upsert(ctx, cfg); err != nil {
			return fmt.Errorf("seed NodeItemConfig %s@%s: %w", s.itemID, s.nodeID, err)
		}
		log.Info().Str("node", s.nodeID).Str("item", s.itemID).Str("strategy", string(s.strategy)).Msg("[Seed] Upserted NodeItemConfig")
	}

	// ── 6. Initial NodeStock ──────────────────────────────────────────────────────
	// Seed some starting stock so the UI is not empty and users can test workflows.
	for _, s := range nicSeeds {
		// e.g. Factory gets 10,000, Store gets 100
		startQty := 100.0
		if s.nodeID == "FACTORY" {
			startQty = 10000.0
		}
		
		stock := &models.NodeStock{
			NodeID:        s.nodeID,
			ItemID:        s.itemID,
			QtyOnHand:     startQty,
			LastUpdatedAt: time.Now(),
		}
		if err := stockRepo.Upsert(ctx, stock); err != nil {
			return fmt.Errorf("seed NodeStock %s@%s: %w", s.itemID, s.nodeID, err)
		}
	}


	log.Info().Msg("[Seed] ✅ Kitchen demo data seeded successfully")
	return nil
}
