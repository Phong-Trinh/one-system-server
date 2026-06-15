package app

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// SeedKitchenData seeds all demo data required for the triple-pane KDS UI:
//   - Equipment types (ST_BEP_NUONG = Grill, ST_MAY_CHIEN = Fryer, ST_BAN_RAP = Assembly)
//   - Machines (M1_BEP_NUONG, M2_MAY_CHIEN_1, etc.) — IDs match app.js kdsMachines
//   - Items (raw materials + finished products)
//   - ItemCapacityConfigs — slot_consumption + allow_mix per (item × equipment type)
//   - BOMs with exact IDs matching app.js DEMO_ORDERS
//   - SOPs with 1–2 step flows referencing equipment types above
//
// NOTE: SlotConsumption and AllowMix are now in ItemCapacityConfig (per item × equipment type),
// not on SOPStep. SOPStep only carries EquipmentTypeID (the category pointer).
//
// All ops are idempotent — safe to call on every startup.
func SeedKitchenData(
	ctx context.Context,
	equipTypeRepo services.EquipmentTypeRepository,
	machineRepo services.MachineRepository,
	itemRepo services.ItemRepository,
	bomRepo services.BOMRepository,
	sopRepo services.SOPRepository,
	capRepo services.ItemCapacityConfigRepository,
) error {
	const nodeID = "CUA_HANG_01"
	const orgID = "SNAPBITE_ORG"

	// ── 1. Equipment Types ────────────────────────────────────────────────────
	equipTypes := []models.EquipmentType{
		{ID: "ST_BEP_NUONG", Name: "Bếp Nướng", CapacityUnit: "slot"},
		{ID: "ST_MAY_CHIEN", Name: "Máy Chiên", CapacityUnit: "liter"},
		{ID: "ST_BAN_RAP", Name: "Bàn Ráp", CapacityUnit: "slot"},
	}
	for _, et := range equipTypes {
		_ = equipTypeRepo.Delete(ctx, et.ID)
		e := et
		if err := equipTypeRepo.Create(ctx, &e); err != nil {
			return fmt.Errorf("seed equipment type %s: %w", et.ID, err)
		}
		log.Info().Str("id", et.ID).Msg("[Seed] Created EquipmentType")
	}

	// ── 2. Machines ───────────────────────────────────────────────────────────
	machines := []models.Machine{
		// CUA_HANG_01 Machines
		{ID: "M1_BEP_NUONG", EquipmentTypeID: "ST_BEP_NUONG", NodeID: "CUA_HANG_01", MaxCapacity: 8, Status: models.MachineIdle},
		{ID: "M2_MAY_CHIEN_1", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "CUA_HANG_01", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "M3_MAY_CHIEN_2", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "CUA_HANG_01", MaxCapacity: 2, Status: models.MachineIdle},
		{ID: "M4_BAN_RAP", EquipmentTypeID: "ST_BAN_RAP", NodeID: "CUA_HANG_01", MaxCapacity: 10, Status: models.MachineIdle},
		// FACTORY Machines
		{ID: "F_M1_BEP_NUONG", EquipmentTypeID: "ST_BEP_NUONG", NodeID: "FACTORY", MaxCapacity: 80, Status: models.MachineIdle},
		{ID: "F_M2_MAY_CHIEN_1", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "FACTORY", MaxCapacity: 20, Status: models.MachineIdle},
		{ID: "F_M3_MAY_CHIEN_2", EquipmentTypeID: "ST_MAY_CHIEN", NodeID: "FACTORY", MaxCapacity: 20, Status: models.MachineIdle},
		{ID: "F_M4_BAN_RAP", EquipmentTypeID: "ST_BAN_RAP", NodeID: "FACTORY", MaxCapacity: 100, Status: models.MachineIdle},
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
		{ID: "ITEM_BO_TUOI", OrgID: orgID, Name: "Thịt Bò Tươi", SKU: "RM-BO-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_GA_TUOI", OrgID: orgID, Name: "Thịt Gà Tươi", SKU: "RM-GA-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_BANH_MI", OrgID: orgID, Name: "Bánh Mì Tròn", SKU: "RM-BM-001", Type: models.ItemTypeRawMaterial, BaseUnit: "piece"},
		{ID: "ITEM_HANH_TAY", OrgID: orgID, Name: "Hành Tây", SKU: "RM-HT-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_KHOAI_TAY", OrgID: orgID, Name: "Khoai Tây Raw", SKU: "RM-KT-001", Type: models.ItemTypeRawMaterial, BaseUnit: "gram"},
		{ID: "ITEM_HAMBURGER_BO", OrgID: orgID, Name: "Hamburger Bò", SKU: "PRD-HB-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
		{ID: "ITEM_HAMBURGER_GA", OrgID: orgID, Name: "Hamburger Gà", SKU: "PRD-HG-001", Type: models.ItemTypeProduct, BaseUnit: "piece"},
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

	// ── 4. ItemCapacityConfigs ─────────────────────────────────────────────────
	// SlotConsumption and AllowMix are now per (item × equipment_type), not on SOPStep.
	// This table drives the bin-packing allocation engine.
	capConfigs := []models.ItemCapacityConfig{
		// Beef patty: 2 grill slots per piece, allows mixing with other grill items
		{ItemID: "ITEM_HAMBURGER_BO", EquipmentTypeID: "ST_BEP_NUONG", SlotConsumption: 2, AllowMix: true},
		// Chicken patty: 2 grill slots per piece, allows mixing
		{ItemID: "ITEM_HAMBURGER_GA", EquipmentTypeID: "ST_BEP_NUONG", SlotConsumption: 2, AllowMix: true},
		// Steak: 3 grill slots, no mixing (exclusive cycle)
		{ItemID: "ITEM_BIT_TET", EquipmentTypeID: "ST_BEP_NUONG", SlotConsumption: 3, AllowMix: false},
		// Fries: 1 liter fryer slot, allows mixing
		{ItemID: "ITEM_KHOAI_TAY_CHIEN", EquipmentTypeID: "ST_MAY_CHIEN", SlotConsumption: 1, AllowMix: true},
		// Onion rings: 1 liter fryer slot, allows mixing
		{ItemID: "ITEM_HANH_TAY", EquipmentTypeID: "ST_MAY_CHIEN", SlotConsumption: 1, AllowMix: true},
		// Assembly station: 1 slot per item, all allow mixing
		{ItemID: "ITEM_HAMBURGER_BO", EquipmentTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true},
		{ItemID: "ITEM_HAMBURGER_GA", EquipmentTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true},
		{ItemID: "ITEM_BIT_TET", EquipmentTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: false},
		{ItemID: "ITEM_KHOAI_TAY_CHIEN", EquipmentTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true},
	}
	for _, cc := range capConfigs {
		_ = capRepo.Delete(ctx, cc.ItemID, cc.EquipmentTypeID)
		c := cc
		if err := capRepo.Save(ctx, &c); err != nil {
			return fmt.Errorf("seed cap config (%s, %s): %w", cc.ItemID, cc.EquipmentTypeID, err)
		}
	}
	log.Info().Int("count", len(capConfigs)).Msg("[Seed] Created ItemCapacityConfigs")

	// ── 5. BOMs + SOPs ───────────────────────────────────────────────────────
	// SOPStep now only has EquipmentTypeID (*string). SlotConsumption/AllowMix are in capConfigs above.
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
		{
			bomID:        "BOM_HAMBURGER_BO",
			outputItemID: "ITEM_HAMBURGER_BO",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_BIT_TET", 1},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
			},
			steps: []models.SOPStep{
				{ID: "STEP_HB_NUONG_BO", Description: "Nướng Bò", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 20, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_1"}},
				{ID: "STEP_HB_NUONG_BANH", Description: "Nướng Bánh Mì", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_2"}},
				{ID: "STEP_HB_CHIEN_HANH", Description: "Chiên Hành", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_BO_LINE_3"}},
				{ID: "STEP_HB_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 10, DependsOn: []string{"STEP_HB_NUONG_BO", "STEP_HB_NUONG_BANH", "STEP_HB_CHIEN_HANH"}},
			},
		},
		{
			bomID:        "BOM_HAMBURGER_GA",
			outputItemID: "ITEM_HAMBURGER_GA",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_GA_TUOI", 150},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
			},
			steps: []models.SOPStep{
				{ID: "STEP_HG_NUONG_GA", Description: "Nướng Gà", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 18, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_1"}}, // ITEM_GA_TUOI
				{ID: "STEP_HG_NUONG_BANH", Description: "Nướng Bánh Mì", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 10, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_2"}}, // ITEM_BANH_MI
				{ID: "STEP_HG_CHIEN_HANH", Description: "Chiên Hành", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 15, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_HAMBURGER_GA_LINE_3"}}, // ITEM_HANH_TAY
				{ID: "STEP_HG_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 10, DependsOn: []string{"STEP_HG_NUONG_GA", "STEP_HG_NUONG_BANH", "STEP_HG_CHIEN_HANH"}},
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
				{ID: "STEP_BT_NUONG", Description: "Nướng Bít Tết", EquipmentTypeID: equipPtr("ST_BEP_NUONG"), Duration: 25, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_BIT_TET_LINE_1"}}, // ITEM_BO_TUOI
				{ID: "STEP_BT_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 12, DependsOn: []string{"STEP_BT_NUONG"}},
			},
		},
		{
			bomID:        "BOM_KHOAI_TAY_CHIEN",
			outputItemID: "ITEM_KHOAI_TAY_CHIEN",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_KHOAI_TAY", 200},
			},
			steps: []models.SOPStep{
				{ID: "STEP_KC_CHIEN", Description: "Chiên Khoai Tây", EquipmentTypeID: equipPtr("ST_MAY_CHIEN"), Duration: 20, DependsOn: []string{}, IngredientBOMLineIDs: []string{"BOM_KHOAI_TAY_CHIEN_LINE_1"}}, // ITEM_KHOAI_TAY
				{ID: "STEP_KC_SAP_XEP", Description: "Sắp Xếp Món", EquipmentTypeID: equipPtr("ST_BAN_RAP"), Duration: 8, DependsOn: []string{"STEP_KC_CHIEN"}},
			},
		},
	}

	for _, seed := range seeds {
		_ = bomRepo.Delete(ctx, seed.bomID)
		_ = sopRepo.Delete(ctx, "SOP_"+seed.bomID)

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

		sopID := "SOP_" + seed.bomID
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

	log.Info().Msg("[Seed] ✅ Kitchen demo data seeded successfully")
	return nil
}
