package usecase

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

// TestFactory_SC4_HappyPath đo lường thời gian hoàn thành lý tưởng (Happy Path) của SC4:
// - Cả 2 máy nướng (m_grill_A và m_grill_B) đều hoạt động 100% bình thường (không hỏng hóc).
// - Tất cả PO (500 Bun, 600 Patty, 5L Sauce, 200 Chicken) được submit cùng lúc 06:00 AM (không bị chèn đơn đột xuất).
// - 2 Nhân viên làm việc liên tục.
func TestFactory_SC4_HappyPath(t *testing.T) {
	ctx, poRepo, sopRepo, batchRepo, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()
	_ = batchRepo

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "factory_f"

	// ── Staff Shifts ──────────────────────────────────────────────────────────
	shiftRepo.shifts["shift_1"] = &models.StaffShift{ID: "shift_1", NodeID: nodeID, StaffID: "f_staff_1", Status: models.ShiftActive}
	shiftRepo.shifts["shift_2"] = &models.StaffShift{ID: "shift_2", NodeID: nodeID, StaffID: "f_staff_2", Status: models.ShiftActive}

	// ── Machines (Tất cả hoạt động 100%) ──────────────────────────────────────
	machineRepo.machines["m_grill_A"] = &models.Machine{ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 48}
	machineRepo.machines["m_grill_B"] = &models.Machine{ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 48}
	machineRepo.machines["m_mixer"] = &models.Machine{ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer", Status: models.MachineIdle, MaxCapacity: 1}
	machineRepo.machines["m_proofer"] = &models.Machine{ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer", Status: models.MachineIdle, MaxCapacity: 1}
	machineRepo.machines["m_fryer"] = &models.Machine{ID: "m_fryer", NodeID: nodeID, EquipmentTypeID: "fryer", Status: models.MachineIdle, MaxCapacity: 50}

	// ── SOP Definitions ───────────────────────────────────────────────────────
	sopBun := "sop_bun_sc4_hp"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["sc4_hp_bun_dry"] = &models.SOPStep{ID: "sc4_hp_bun_dry", SOPID: sopBun, SeqNo: 1, Duration: 10 * 60}
	sopRepo.steps["sc4_hp_bun_mix"] = &models.SOPStep{
		ID: "sc4_hp_bun_mix", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"sc4_hp_bun_dry"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_hp_bun_proof"] = &models.SOPStep{
		ID: "sc4_hp_bun_proof", SOPID: sopBun, SeqNo: 3, DependsOn: []string{"sc4_hp_bun_mix"},
		EquipmentTypeID: ptrStrF("proofer"), IsIdleStep: true, Duration: 45 * 60, ActiveTime: ptrIntF(5 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_hp_bun_shape"] = &models.SOPStep{ID: "sc4_hp_bun_shape", SOPID: sopBun, SeqNo: 4, DependsOn: []string{"sc4_hp_bun_proof"}, Duration: 20 * 60}
	sopRepo.steps["sc4_hp_bun_bake"] = &models.SOPStep{
		ID: "sc4_hp_bun_bake", SOPID: sopBun, SeqNo: 5, DependsOn: []string{"sc4_hp_bun_shape"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}

	sopPatty := "sop_patty_sc4_hp"
	sopRepo.sops[sopPatty] = &models.SOP{ID: sopPatty}
	sopRepo.steps["sc4_hp_patty_prep"] = &models.SOPStep{ID: "sc4_hp_patty_prep", SOPID: sopPatty, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["sc4_hp_patty_mix"] = &models.SOPStep{
		ID: "sc4_hp_patty_mix", SOPID: sopPatty, SeqNo: 2, DependsOn: []string{"sc4_hp_patty_prep"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true, Duration: 12 * 60, ActiveTime: ptrIntF(2 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_hp_patty_weigh"] = &models.SOPStep{ID: "sc4_hp_patty_weigh", SOPID: sopPatty, SeqNo: 3, DependsOn: []string{"sc4_hp_patty_mix"}, Duration: 200 * 60, IsSplittable: true}
	sopRepo.steps["sc4_hp_patty_mold"] = &models.SOPStep{ID: "sc4_hp_patty_mold", SOPID: sopPatty, SeqNo: 4, DependsOn: []string{"sc4_hp_patty_weigh"}, Duration: 150 * 60, IsSplittable: true}
	sopRepo.steps["sc4_hp_patty_pack"] = &models.SOPStep{ID: "sc4_hp_patty_pack", SOPID: sopPatty, SeqNo: 5, DependsOn: []string{"sc4_hp_patty_mold"}, Duration: 10 * 60}

	sopSauce := "sop_sauce_sc4_hp"
	sopRepo.sops[sopSauce] = &models.SOP{ID: sopSauce}
	sopRepo.steps["sc4_hp_sauce_prep"] = &models.SOPStep{ID: "sc4_hp_sauce_prep", SOPID: sopSauce, SeqNo: 1, Duration: 10 * 60}
	sopRepo.steps["sc4_hp_sauce_mix"] = &models.SOPStep{ID: "sc4_hp_sauce_mix", SOPID: sopSauce, SeqNo: 2, DependsOn: []string{"sc4_hp_sauce_prep"}, Duration: 20 * 60}
	sopRepo.steps["sc4_hp_sauce_pack"] = &models.SOPStep{ID: "sc4_hp_sauce_pack", SOPID: sopSauce, SeqNo: 3, DependsOn: []string{"sc4_hp_sauce_mix"}, Duration: 10 * 60}

	sopChicken := "sop_chicken_sc4_hp"
	sopRepo.sops[sopChicken] = &models.SOP{ID: sopChicken}
	sopRepo.steps["sc4_hp_chk_marinate"] = &models.SOPStep{ID: "sc4_hp_chk_marinate", SOPID: sopChicken, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["sc4_hp_chk_fry"] = &models.SOPStep{
		ID: "sc4_hp_chk_fry", SOPID: sopChicken, SeqNo: 2, DependsOn: []string{"sc4_hp_chk_marinate"},
		EquipmentTypeID: ptrStrF("fryer"), IsIdleStep: true, Duration: 18 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}
	sopRepo.steps["sc4_hp_chk_pack"] = &models.SOPStep{ID: "sc4_hp_chk_pack", SOPID: sopChicken, SeqNo: 3, DependsOn: []string{"sc4_hp_chk_fry"}, Duration: 10 * 60}

	// ── 06:00 AM Initial POs ──────────────────────────────────────────────────
	poBun := &models.ProductionOrder{ID: "po_bun_sc4_hp", NodeID: nodeID, SOPID: sopBun, TargetQty: 500, Status: models.POInProgress, CreatedAt: nowBase}
	poPatty := &models.ProductionOrder{ID: "po_patty_sc4_hp", NodeID: nodeID, SOPID: sopPatty, TargetQty: 600, Status: models.POInProgress, CreatedAt: nowBase}
	poSauce := &models.ProductionOrder{ID: "po_sauce_sc4_hp", NodeID: nodeID, SOPID: sopSauce, TargetQty: 5, Status: models.POInProgress, CreatedAt: nowBase}
	poChicken := &models.ProductionOrder{ID: "po_chicken_sc4_hp", NodeID: nodeID, SOPID: sopChicken, TargetQty: 200, Status: models.POInProgress, CreatedAt: nowBase}

	_ = poRepo.Create(ctx, poBun)
	_ = poRepo.Create(ctx, poPatty)
	_ = poRepo.Create(ctx, poSauce)
	_ = poRepo.Create(ctx, poChicken)

	_, _ = engine.SchedulePO(ctx, poBun.ID)
	_, _ = engine.SchedulePO(ctx, poPatty.ID)
	_, _ = engine.SchedulePO(ctx, poSauce.ID)
	_, _ = engine.SchedulePO(ctx, poChicken.ID)

	allTasks, err := taskRepo.FindByNode(ctx, nodeID, nil)
	if err != nil {
		t.Fatalf("Failed to query tasks: %v", err)
	}

	// Print full task execution timeline
	sort.Slice(allTasks, func(i, j int) bool {
		return allTasks[i].ScheduledStart.Before(allTasks[j].ScheduledStart)
	})

	fmt.Printf("\n=== FULL TIMELINE LOGS (HAPPY PATH SC4) ===\n")
	for _, tk := range allTasks {
		if tk.Status == models.TaskCancelled || tk.ScheduledStart.IsZero() {
			continue
		}
		step, _ := sopRepo.FindStepByID(ctx, tk.SOPStepID)
		desc := tk.SOPStepID
		if step != nil && step.Description != "" {
			desc = step.Description
		}
		startOffset := int(tk.ScheduledStart.Sub(nowBase).Minutes())
		endOffset := int(tk.ScheduledEnd.Sub(nowBase).Minutes())
		clockStart := nowBase.Add(time.Duration(startOffset) * time.Minute).Format("15:04")
		clockEnd := nowBase.Add(time.Duration(endOffset) * time.Minute).Format("15:04")

		machStr := ""
		if tk.MachineID != "" {
			machStr = fmt.Sprintf(" [%s]", tk.MachineID)
		}
		kindStr := string(tk.TaskKind)
		if tk.ParentTaskID != nil {
			kindStr = "FILL-IN"
		}

		fmt.Printf("T+%03d [%s -> %s] ▶️  %-10s: [%s] %s (Qty: %.0f%s)\n",
			startOffset, clockStart, clockEnd, tk.AssignedTo, kindStr, desc, tk.TargetQty, machStr)
	}

	// Calculate completion time per PO
	poCompletionTimes := make(map[string]time.Time)
	for _, tk := range allTasks {
		if tk.Status == models.TaskCancelled {
			continue
		}
		if currentEnd, ok := poCompletionTimes[tk.POID]; !ok || tk.ScheduledEnd.After(currentEnd) {
			poCompletionTimes[tk.POID] = tk.ScheduledEnd
		}
	}

	var latestEnd time.Time
	for _, endTime := range poCompletionTimes {
		if endTime.After(latestEnd) {
			latestEnd = endTime
		}
	}

	var shiftStart time.Time
	for _, tk := range allTasks {
		if shiftStart.IsZero() || (tk.ScheduledStart.Before(shiftStart) && !tk.ScheduledStart.IsZero()) {
			shiftStart = tk.ScheduledStart
		}
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("☀️ SC4 HAPPY PATH BENCHMARK RESULTS ☀️\n")
	fmt.Printf("=======================================================\n")
	for poID, endTime := range poCompletionTimes {
		durMin := int(endTime.Sub(shiftStart).Minutes())
		finishClock := nowBase.Add(time.Duration(durMin) * time.Minute)
		fmt.Printf(" - PO %-18s hoàn thành lúc: %s (Mất %d phút = %dh%dm)\n", poID, finishClock.Format("15:04"), durMin, durMin/60, durMin%60)
	}
	fmt.Printf("-------------------------------------------------------\n")
	totalMin := int(latestEnd.Sub(shiftStart).Minutes())
	finishTotal := nowBase.Add(time.Duration(totalMin) * time.Minute)
	fmt.Printf("🏁 TOÀN BỘ CA HOÀN THÀNH LÚC: %s\n", finishTotal.Format("15:04"))
	fmt.Printf("⏱️ Tổng thời gian sản xuất ca: %d phút (%dh%dm)\n", totalMin, totalMin/60, totalMin%60)
	fmt.Printf("=======================================================\n")
}
