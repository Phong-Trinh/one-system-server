package usecase

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

// TestFactory_SC4_FullSimulation mô phỏng đầy đủ 4 thảm họa vận hành F&B:
// 1. Tải khủng 500 Bun, 600 Patty, 5L Sauce, 200 Chicken.
// 2. 07:30 AM - Máy nướng m_grill_A bị hỏng (UNDER_MAINTENANCE) -> Re-queue dồn sang m_grill_B.
// 3. 08:15 AM - Đơn gấp VIP 100 Burger (Deadline 10:30 AM) chèn ngang.
// 4. Theo dõi & tính chi phí rửa tay/chuyển vùng vệ sinh (Hygiene Zone Switching = 3m).
func TestFactory_SC4_FullSimulation(t *testing.T) {
	ctx, poRepo, sopRepo, batchRepo, shiftRepo, taskRepo, machineRepo, disp, engine := setupTestEnv()
	_ = batchRepo

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "factory_f"

	// ── Staff Shifts ──────────────────────────────────────────────────────────
	shiftRepo.shifts["shift_1"] = &models.StaffShift{ID: "shift_1", NodeID: nodeID, StaffID: "f_staff_1", Status: models.ShiftActive}
	shiftRepo.shifts["shift_2"] = &models.StaffShift{ID: "shift_2", NodeID: nodeID, StaffID: "f_staff_2", Status: models.ShiftActive}

	// ── Machines ──────────────────────────────────────────────────────────────
	machineRepo.machines["m_grill_A"] = &models.Machine{ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 48}
	machineRepo.machines["m_grill_B"] = &models.Machine{ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 48}
	machineRepo.machines["m_mixer"] = &models.Machine{ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer", Status: models.MachineIdle, MaxCapacity: 1}
	machineRepo.machines["m_proofer"] = &models.Machine{ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer", Status: models.MachineIdle, MaxCapacity: 1}
	machineRepo.machines["m_fryer"] = &models.Machine{ID: "m_fryer", NodeID: nodeID, EquipmentTypeID: "fryer", Status: models.MachineIdle, MaxCapacity: 50}

	// ── SOP Definitions ───────────────────────────────────────────────────────
	sopBun := "sop_bun_sc4"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["sc4_bun_dry"] = &models.SOPStep{ID: "sc4_bun_dry", SOPID: sopBun, SeqNo: 1, Duration: 10 * 60}
	sopRepo.steps["sc4_bun_mix"] = &models.SOPStep{
		ID: "sc4_bun_mix", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"sc4_bun_dry"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_bun_proof"] = &models.SOPStep{
		ID: "sc4_bun_proof", SOPID: sopBun, SeqNo: 3, DependsOn: []string{"sc4_bun_mix"},
		EquipmentTypeID: ptrStrF("proofer"), IsIdleStep: true, Duration: 45 * 60, ActiveTime: ptrIntF(5 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_bun_shape"] = &models.SOPStep{ID: "sc4_bun_shape", SOPID: sopBun, SeqNo: 4, DependsOn: []string{"sc4_bun_proof"}, Duration: 20 * 60}
	sopRepo.steps["sc4_bun_bake"] = &models.SOPStep{
		ID: "sc4_bun_bake", SOPID: sopBun, SeqNo: 5, DependsOn: []string{"sc4_bun_shape"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}

	sopPatty := "sop_patty_sc4"
	sopRepo.sops[sopPatty] = &models.SOP{ID: sopPatty}
	sopRepo.steps["sc4_patty_prep"] = &models.SOPStep{ID: "sc4_patty_prep", SOPID: sopPatty, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["sc4_patty_mix"] = &models.SOPStep{
		ID: "sc4_patty_mix", SOPID: sopPatty, SeqNo: 2, DependsOn: []string{"sc4_patty_prep"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true, Duration: 12 * 60, ActiveTime: ptrIntF(2 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_patty_weigh"] = &models.SOPStep{ID: "sc4_patty_weigh", SOPID: sopPatty, SeqNo: 3, DependsOn: []string{"sc4_patty_mix"}, Duration: 200 * 60, IsSplittable: true}
	sopRepo.steps["sc4_patty_mold"] = &models.SOPStep{ID: "sc4_patty_mold", SOPID: sopPatty, SeqNo: 4, DependsOn: []string{"sc4_patty_weigh"}, Duration: 150 * 60, IsSplittable: true}
	sopRepo.steps["sc4_patty_pack"] = &models.SOPStep{ID: "sc4_patty_pack", SOPID: sopPatty, SeqNo: 5, DependsOn: []string{"sc4_patty_mold"}, Duration: 10 * 60}

	sopSauce := "sop_sauce_sc4"
	sopRepo.sops[sopSauce] = &models.SOP{ID: sopSauce}
	sopRepo.steps["sc4_sauce_prep"] = &models.SOPStep{ID: "sc4_sauce_prep", SOPID: sopSauce, SeqNo: 1, Duration: 10 * 60}
	sopRepo.steps["sc4_sauce_mix"] = &models.SOPStep{ID: "sc4_sauce_mix", SOPID: sopSauce, SeqNo: 2, DependsOn: []string{"sc4_sauce_prep"}, Duration: 20 * 60}
	sopRepo.steps["sc4_sauce_pack"] = &models.SOPStep{ID: "sc4_sauce_pack", SOPID: sopSauce, SeqNo: 3, DependsOn: []string{"sc4_sauce_mix"}, Duration: 10 * 60}

	sopChicken := "sop_chicken_sc4"
	sopRepo.sops[sopChicken] = &models.SOP{ID: sopChicken}
	sopRepo.steps["sc4_chk_marinate"] = &models.SOPStep{ID: "sc4_chk_marinate", SOPID: sopChicken, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["sc4_chk_fry"] = &models.SOPStep{
		ID: "sc4_chk_fry", SOPID: sopChicken, SeqNo: 2, DependsOn: []string{"sc4_chk_marinate"},
		EquipmentTypeID: ptrStrF("fryer"), IsIdleStep: true, Duration: 18 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}
	sopRepo.steps["sc4_chk_pack"] = &models.SOPStep{ID: "sc4_chk_pack", SOPID: sopChicken, SeqNo: 3, DependsOn: []string{"sc4_chk_fry"}, Duration: 10 * 60}

	// ── 06:00 AM Initial POs ──────────────────────────────────────────────────
	poBun := &models.ProductionOrder{ID: "po_bun_sc4", NodeID: nodeID, SOPID: sopBun, TargetQty: 500, Status: models.POInProgress, CreatedAt: nowBase}
	poPatty := &models.ProductionOrder{ID: "po_patty_sc4", NodeID: nodeID, SOPID: sopPatty, TargetQty: 600, Status: models.POInProgress, CreatedAt: nowBase}
	poSauce := &models.ProductionOrder{ID: "po_sauce_sc4", NodeID: nodeID, SOPID: sopSauce, TargetQty: 5, Status: models.POInProgress, CreatedAt: nowBase}
	poChicken := &models.ProductionOrder{ID: "po_chicken_sc4", NodeID: nodeID, SOPID: sopChicken, TargetQty: 200, Status: models.POInProgress, CreatedAt: nowBase}

	_ = poRepo.Create(ctx, poBun)
	_ = poRepo.Create(ctx, poPatty)
	_ = poRepo.Create(ctx, poSauce)
	_ = poRepo.Create(ctx, poChicken)

	_, _ = engine.SchedulePO(ctx, poBun.ID)
	_, _ = engine.SchedulePO(ctx, poPatty.ID)
	_, _ = engine.SchedulePO(ctx, poSauce.ID)
	_, _ = engine.SchedulePO(ctx, poChicken.ID)

	// ── EVENT 1: 07:30 AM - m_grill_A Breakdown Event ────────────────────────
	fmt.Printf("\n⚡ EVENT 1 [07:30 AM]: Máy Nướng A (m_grill_A) BỊ HỎNG -> Trigger Re-queue & Dispatch\n")
	machineRepo.machines["m_grill_A"].Status = models.MachineUnderMaintenance
	_ = disp.Dispatch(ctx, nodeID)

	// ── EVENT 2: 08:15 AM - VIP Emergency Order Injection ────────────────────
	fmt.Printf("⚡ EVENT 2 [08:15 AM]: Đơn hàng VIP (100 Burger) chèn ngang (Deadline 10:30 AM)\n")
	vipDeadline := nowBase.Add(4*time.Hour + 30*time.Minute)
	poVIP := &models.ProductionOrder{
		ID: "po_vip_party", NodeID: nodeID, SOPID: sopPatty, TargetQty: 100,
		Status: models.POInProgress, DeadlineAt: &vipDeadline, CreatedAt: nowBase.Add(2*time.Hour + 15*time.Minute),
	}
	_ = poRepo.Create(ctx, poVIP)
	_, _ = engine.SchedulePO(ctx, poVIP.ID)

	// ── EXECUTION & HYGIENE ZONE AUDIT ───────────────────────────────────────
	allTasks, err := taskRepo.FindByNode(ctx, nodeID, nil)
	if err != nil {
		t.Fatalf("Failed to query tasks: %v", err)
	}

	// Sort tasks by ScheduledStart
	sort.Slice(allTasks, func(i, j int) bool {
		return allTasks[i].ScheduledStart.Before(allTasks[j].ScheduledStart)
	})

	// Hygiene Zone Tracking
	// Raw zone: Patty/Chicken prep/weigh/mold
	// Clean zone: Bun bake/dry/mix/proof, Sauce mix/pack
	staffLastZone := make(map[string]string)
	zoneSwitchCount := make(map[string]int)

	for _, task := range allTasks {
		if task.AssignedTo == "" {
			continue
		}
		currentZone := "CLEAN"
		if task.SOPStepID == "sc4_patty_prep" || task.SOPStepID == "sc4_patty_weigh" || task.SOPStepID == "sc4_patty_mold" || task.SOPStepID == "sc4_chk_marinate" {
			currentZone = "RAW"
		}

		lastZone, exists := staffLastZone[task.AssignedTo]
		if exists && lastZone != currentZone {
			zoneSwitchCount[task.AssignedTo]++
		}
		staffLastZone[task.AssignedTo] = currentZone
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("📋 SC4 FULL SIMULATION REPORT & AUDIT\n")
	fmt.Printf("=======================================================\n")
	fmt.Printf("Total Tasks Scheduled: %d\n", len(allTasks))
	fmt.Printf("Hygiene Zone Switches (Raw <-> Clean):\n")
	totalSwitches := 0
	for staff, count := range zoneSwitchCount {
		fmt.Printf(" - Staff %s: %d lần chuyển vùng (Tốn %d phút rửa tay)\n", staff, count, count*3)
		totalSwitches += count
	}
	fmt.Printf("Total Hygiene Switch Overhead: %d phút\n", totalSwitches*3)

	t.Run("A1_MachineBreakdownResilience", func(t *testing.T) {
		for _, tk := range allTasks {
			if tk.MachineID == "m_grill_A" && tk.Status == models.TaskPending {
				t.Errorf("Task %s is assigned to broken machine m_grill_A!", tk.ID)
			}
		}
	})

	t.Run("A2_HygieneSwitchWithinLimit", func(t *testing.T) {
		if totalSwitches > 8 {
			t.Errorf("Too many hygiene zone switches (%d > 8), inefficient dispatching!", totalSwitches)
		}
	})
}
