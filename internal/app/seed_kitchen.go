package app

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// SeedKitchenData seeds all demo data required for the triple-pane KDS UI:
//   - Station types (BEP_NUONG = Grill, MAY_CHIEN = Fryer)
//   - Machines (M1_BEP_NUONG, M2_MAY_CHIEN_1, M3_MAY_CHIEN_2) — IDs match app.js kdsMachines
//   - Items (raw materials + finished products)
//   - BOMs with exact IDs matching app.js DEMO_ORDERS: BOM_HAMBURGER_BO, BOM_HAMBURGER_GA, BOM_BIT_TET
//   - SOPs with 1–2 step flows using station types above
//
// All ops are idempotent — safe to call on every startup.
func SeedKitchenData(
	ctx context.Context,
	stationTypeRepo services.StationTypeRepository,
	machineRepo services.MachineRepository,
	itemRepo services.ItemRepository,
	bomRepo services.BOMRepository,
	sopRepo services.SOPRepository,
) error {
	const nodeID = "CUA_HANG_01"
	const orgID = "SNAPBITE_ORG"

	// ── 1. Station Types ──────────────────────────────────────────────────────
	stationTypes := []models.StationType{
		{ID: "ST_BEP_NUONG", Name: "Bếp Nướng", CapacityUnit: "slot", DefaultStrategy: models.StrategyAsync},
		{ID: "ST_MAY_CHIEN", Name: "Máy Chiên", CapacityUnit: "liter", DefaultStrategy: models.StrategySync},
		{ID: "ST_BAN_RAP", Name: "Bàn Ráp", CapacityUnit: "slot", DefaultStrategy: models.StrategyAsync},
	}
	for _, st := range stationTypes {
		_ = stationTypeRepo.Delete(ctx, st.ID)
		s := st
		if err := stationTypeRepo.Create(ctx, &s); err != nil {
			return fmt.Errorf("seed station type %s: %w", st.ID, err)
		}
		log.Info().Str("id", st.ID).Msg("[Seed] Created StationType")
	}

	// ── 2. Machines ───────────────────────────────────────────────────────────
	machines := []models.Machine{
		{ID: "M1_BEP_NUONG", StationTypeID: "ST_BEP_NUONG", NodeID: nodeID, MaxCapacity: 8, AllocationStrategy: models.StrategyAsync, Status: models.MachineIdle},
		{ID: "M2_MAY_CHIEN_1", StationTypeID: "ST_MAY_CHIEN", NodeID: nodeID, MaxCapacity: 2, AllocationStrategy: models.StrategySync, Status: models.MachineIdle},
		{ID: "M3_MAY_CHIEN_2", StationTypeID: "ST_MAY_CHIEN", NodeID: nodeID, MaxCapacity: 2, AllocationStrategy: models.StrategySync, Status: models.MachineIdle},
		{ID: "M4_BAN_RAP", StationTypeID: "ST_BAN_RAP", NodeID: nodeID, MaxCapacity: 10, AllocationStrategy: models.StrategyAsync, Status: models.MachineIdle},
	}
	for _, m := range machines {
		_ = machineRepo.Delete(ctx, m.ID)
		mc := m
		if err := machineRepo.Create(ctx, &mc); err != nil {
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

	// ── 4. BOMs + SOPs ───────────────────────────────────────────────────────
	type bomSeed struct {
		bomID        string
		outputItemID string
		lines        []struct {
			itemID string
			qty    float64
		}
		steps []models.SOPStep
	}

	seeds := []bomSeed{
		{
			bomID:        "BOM_HAMBURGER_BO",
			outputItemID: "ITEM_HAMBURGER_BO",
			lines: []struct {
				itemID string
				qty    float64
			}{
				{"ITEM_BO_TUOI", 150},
				{"ITEM_BANH_MI", 1},
				{"ITEM_HANH_TAY", 30},
			},
			steps: []models.SOPStep{
				{ID: "STEP_HB_NUONG_BO", Description: "Nướng Bò", StationTypeID: "ST_BEP_NUONG", SlotConsumption: 2, AllowMix: true, Duration: 20, DependsOn: []string{}},
				{ID: "STEP_HB_NUONG_BANH", Description: "Nướng Bánh Mì", StationTypeID: "ST_BEP_NUONG", SlotConsumption: 1, AllowMix: true, Duration: 10, DependsOn: []string{}},
				{ID: "STEP_HB_CHIEN_HANH", Description: "Chiên Hành", StationTypeID: "ST_MAY_CHIEN", SlotConsumption: 1, AllowMix: true, Duration: 15, DependsOn: []string{}},
				{ID: "STEP_HB_SAP_XEP", Description: "Sắp Xếp Món", StationTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true, Duration: 10, DependsOn: []string{"STEP_HB_NUONG_BO", "STEP_HB_NUONG_BANH", "STEP_HB_CHIEN_HANH"}},
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
				{ID: "STEP_HG_NUONG_GA", Description: "Nướng Gà", StationTypeID: "ST_BEP_NUONG", SlotConsumption: 2, AllowMix: true, Duration: 18, DependsOn: []string{}},
				{ID: "STEP_HG_NUONG_BANH", Description: "Nướng Bánh Mì", StationTypeID: "ST_BEP_NUONG", SlotConsumption: 1, AllowMix: true, Duration: 10, DependsOn: []string{}},
				{ID: "STEP_HG_CHIEN_HANH", Description: "Chiên Hành", StationTypeID: "ST_MAY_CHIEN", SlotConsumption: 1, AllowMix: true, Duration: 15, DependsOn: []string{}},
				{ID: "STEP_HG_SAP_XEP", Description: "Sắp Xếp Món", StationTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true, Duration: 10, DependsOn: []string{"STEP_HG_NUONG_GA", "STEP_HG_NUONG_BANH", "STEP_HG_CHIEN_HANH"}},
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
				{ID: "STEP_BT_NUONG", Description: "Nướng Bít Tết", StationTypeID: "ST_BEP_NUONG", SlotConsumption: 3, AllowMix: false, Duration: 25, DependsOn: []string{}},
				{ID: "STEP_BT_SAP_XEP", Description: "Sắp Xếp Món", StationTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: false, Duration: 12, DependsOn: []string{"STEP_BT_NUONG"}},
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
				{ID: "STEP_KC_CHIEN", Description: "Chiên Khoai Tây", StationTypeID: "ST_MAY_CHIEN", SlotConsumption: 1, AllowMix: true, Duration: 20, DependsOn: []string{}},
				{ID: "STEP_KC_SAP_XEP", Description: "Sắp Xếp Món", StationTypeID: "ST_BAN_RAP", SlotConsumption: 1, AllowMix: true, Duration: 8, DependsOn: []string{"STEP_KC_CHIEN"}},
			},
		},
	}

	for _, seed := range seeds {
		// Force delete existing BOM & SOP to recreate them
		_ = bomRepo.Delete(ctx, seed.bomID)
		_ = sopRepo.Delete(ctx, "SOP_"+seed.bomID)

		// Create BOM with exact ID (bypasses usecase UUID generation)
		if err := bomRepo.Create(ctx, &models.BOM{
			ID:           seed.bomID,
			OutputItemID: seed.outputItemID,
			Version:      1,
		}); err != nil {
			return fmt.Errorf("seed BOM %s: %w", seed.bomID, err)
		}

		// BOM Lines
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

		// Create SOP
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
