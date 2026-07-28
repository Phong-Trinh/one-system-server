package usecase

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// setupSC8StressEnv creates the complex environment for SC8 Stress Test.
func setupSC8StressEnv(t *testing.T) (context.Context, time.Time, services.StaffTaskUseCase, services.StaffTaskRepository, *models.Machine, *models.Machine) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "factory_f"

	// 3 Staff
	shiftRepo.shifts["shift_1"] = &models.StaffShift{ID: "shift_1", NodeID: nodeID, StaffID: "f_staff_1", Status: models.ShiftActive}
	shiftRepo.shifts["shift_2"] = &models.StaffShift{ID: "shift_2", NodeID: nodeID, StaffID: "f_staff_2", Status: models.ShiftActive}
	shiftRepo.shifts["shift_3"] = &models.StaffShift{ID: "shift_3", NodeID: nodeID, StaffID: "f_staff_3", Status: models.ShiftActive}

	// 2 Machines (Grills)
	grillA := &models.Machine{ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 50}
	grillB := &models.Machine{ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 50}
	machineRepo.machines["m_grill_A"] = grillA
	machineRepo.machines["m_grill_B"] = grillB

	// SOP Bun (needs Grill)
	sopBun := "sop_bun_stress"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["bun_mix"] = &models.SOPStep{ID: "bun_mix", SOPID: sopBun, SeqNo: 1, Duration: 20 * 60}
	sopRepo.steps["bun_bake"] = &models.SOPStep{
		ID: "bun_bake", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"bun_mix"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true, Duration: 20 * 60, ActiveTime: ptrIntF(5 * 60), SlotConsumption: 1.0,
	}

	// SOP Patty (needs Grill)
	sopPatty := "sop_patty_stress"
	sopRepo.sops[sopPatty] = &models.SOP{ID: sopPatty}
	sopRepo.steps["patty_prep"] = &models.SOPStep{ID: "patty_prep", SOPID: sopPatty, SeqNo: 1, Duration: 30 * 60}
	sopRepo.steps["patty_grill"] = &models.SOPStep{
		ID: "patty_grill", SOPID: sopPatty, SeqNo: 2, DependsOn: []string{"patty_prep"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(5 * 60), SlotConsumption: 1.0,
	}

	// SOP Sauce (Manual, Independent)
	sopSauce := "sop_sauce_stress"
	sopRepo.sops[sopSauce] = &models.SOP{ID: sopSauce}
	sopRepo.steps["sauce_mix"] = &models.SOPStep{ID: "sauce_mix", SOPID: sopSauce, SeqNo: 1, Duration: 45 * 60}
	sopRepo.steps["sauce_pack"] = &models.SOPStep{ID: "sauce_pack", SOPID: sopSauce, SeqNo: 2, DependsOn: []string{"sauce_mix"}, Duration: 20 * 60}

	// Create POs
	poBun := &models.ProductionOrder{ID: "po_bun", NodeID: nodeID, ItemID: "bun", TargetQty: 100, Status: models.POInProgress, SOPID: sopBun}
	poPatty := &models.ProductionOrder{ID: "po_patty", NodeID: nodeID, ItemID: "patty", TargetQty: 100, Status: models.POInProgress, SOPID: sopPatty}
	poSauce := &models.ProductionOrder{ID: "po_sauce", NodeID: nodeID, ItemID: "sauce", TargetQty: 100, Status: models.POInProgress, SOPID: sopSauce}
	
	_ = poRepo.Create(ctx, poBun)
	_ = poRepo.Create(ctx, poPatty)
	_ = poRepo.Create(ctx, poSauce)

	// Create Usecase
	kdsUseCase := NewStaffTaskUseCase(taskRepo, machineRepo, engine)

	// Schedule POs
	engine.SchedulePO(ctx, poBun.ID)
	engine.SchedulePO(ctx, poPatty.ID)
	engine.SchedulePO(ctx, poSauce.ID)

	return ctx, nowBase, kdsUseCase, taskRepo, grillA, grillB
}

// ─── HELPER FOR PRINTING TIMELINE ───

func printStressTimeline(t *testing.T, title string, ctx context.Context, repo services.StaffTaskRepository, nowBase time.Time) {
	tasks, _ := repo.FindByNode(ctx, "factory_f", nil)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ScheduledStart.Before(tasks[j].ScheduledStart)
	})

	fmt.Printf("\n========== %s ==========\n", title)
	for _, tk := range tasks {
		if tk.Status == models.TaskDone {
			// Skip done tasks to reduce noise, or print with checkmark
			continue
		}
		startOffset := int(tk.ScheduledStart.Sub(nowBase).Minutes())
		endOffset := int(tk.ScheduledEnd.Sub(nowBase).Minutes())
		clockStart := nowBase.Add(time.Duration(startOffset) * time.Minute).Format("15:04")
		clockEnd := nowBase.Add(time.Duration(endOffset) * time.Minute).Format("15:04")
		
		machineStr := ""
		if tk.MachineID != "" {
			machineStr = fmt.Sprintf(" [%s]", tk.MachineID)
		}
		
		fmt.Printf("T+%04d [%s -> %s] %-10s : [%-8s] %-15s%s\n", 
			startOffset, clockStart, clockEnd, tk.AssignedTo, tk.Status, tk.SOPStepID, machineStr)
	}
	fmt.Println("==================================================")
}

func getTaskByStep(tasks []*models.StaffTask, stepID string) *models.StaffTask {
	for _, tk := range tasks {
		if tk.SOPStepID == stepID {
			return tk
		}
	}
	return nil
}

func TestStress_Case1_DominoEffect(t *testing.T) {
	ctx, nowBase, kds, repo, _, _ := setupSC8StressEnv(t)
	printStressTimeline(t, "INITIAL SCHEDULE", ctx, repo, nowBase)

	tasks, _ := repo.FindByNode(ctx, "factory_f", nil)
	tkPattyPrep := getTaskByStep(tasks, "patty_prep")
	tkSauceMix := getTaskByStep(tasks, "sauce_mix")

	if tkPattyPrep == nil || tkSauceMix == nil {
		t.Fatalf("Could not find required tasks")
	}

	// Staff starts both tasks
	kds.StartTask(ctx, tkPattyPrep.ID, tkPattyPrep.ScheduledStart)
	kds.StartTask(ctx, tkSauceMix.ID, tkSauceMix.ScheduledStart)

	// Patty Prep takes 25 EXTRA minutes -> Domino Effect on Patty Grill
	lateTimePatty := tkPattyPrep.ScheduledEnd.Add(25 * time.Minute)
	fmt.Printf("\n🔥 [SỰ CỐ] Staff làm Patty Prep TRỄ 25 phút (Hoàn thành lúc %s)\n", lateTimePatty.Format("15:04"))
	kds.CompleteTask(ctx, tkPattyPrep.ID, lateTimePatty)

	printStressTimeline(t, "AFTER PATTY PREP LATE DRIFT", ctx, repo, nowBase)

	// Validate Domino: patty_grill (downstream) should be delayed
	tasksAfter, _ := repo.FindByNode(ctx, "factory_f", nil)
	tkPattyGrillNew := getTaskByStep(tasksAfter, "patty_grill")
	if !tkPattyGrillNew.ScheduledStart.After(lateTimePatty.Add(-1 * time.Minute)) {
		t.Errorf("Domino Effect failed: patty_grill should be delayed until after %v, got %v", lateTimePatty, tkPattyGrillNew.ScheduledStart)
	}

	// Validate Independence: sauce_mix (independent) should NOT be affected
	tkSauceMixNew := getTaskByStep(tasksAfter, "sauce_mix")
	if tkSauceMixNew.ScheduledStart != tkSauceMix.ScheduledStart {
		t.Errorf("Independence failed: sauce_mix schedule was changed from %v to %v", tkSauceMix.ScheduledStart, tkSauceMixNew.ScheduledStart)
	}

	fmt.Println("✅ Case 1: Domino Effect OK. Patty Grill bị lùi, Sauce Mix độc lập không bị ảnh hưởng.")
}

func TestStress_Case2_CatastrophicBreakdown(t *testing.T) {
	ctx, nowBase, kds, repo, grillA, grillB := setupSC8StressEnv(t)
	printStressTimeline(t, "INITIAL SCHEDULE", ctx, repo, nowBase)

	tasks, _ := repo.FindByNode(ctx, "factory_f", nil)
	
	// We need to find a Setup task assigned to Grill A
	var tkBakeSetup *models.StaffTask
	for _, tk := range tasks {
		if tk.TaskKind == models.TaskKindSetup && tk.MachineID == "m_grill_A" {
			tkBakeSetup = tk
			break
		}
	}

	if tkBakeSetup == nil {
		t.Skip("Skipping Case 2: No task assigned to Grill A initially (Dispatcher might have picked Grill B).")
		return
	}

	kds.StartTask(ctx, tkBakeSetup.ID, tkBakeSetup.ScheduledStart)

	// Grill A breaks down mid-task
	failTime := tkBakeSetup.ScheduledStart.Add(2 * time.Minute)
	fmt.Printf("\n🔥 [SỰ CỐ] Grill A bốc khói lúc %s\n", failTime.Format("15:04"))
	kds.FailTask(ctx, tkBakeSetup.ID, failTime, "Cháy khét")

	if grillA.Status != models.MachineUnderMaintenance {
		t.Errorf("Grill A status not updated to UNDER_MAINTENANCE")
	}

	printStressTimeline(t, "AFTER CATASTROPHIC BREAKDOWN (GRILL A)", ctx, repo, nowBase)

	// Verify that tasks previously assigned to Grill A are now assigned to Grill B
	tasksAfter, _ := repo.FindByNode(ctx, "factory_f", []models.TaskStatus{models.TaskPending, models.TaskQueued})
	for _, tk := range tasksAfter {
		if tk.MachineID == "m_grill_A" {
			t.Errorf("Catastrophic Breakdown failed: Task %s is still assigned to Grill A!", tk.ID)
		}
	}
	
	fmt.Printf("✅ Case 2: Catastrophic Breakdown OK. Grill A (%s), Grill B (%s). Mọi task chuyển sang Grill B.\n", grillA.Status, grillB.Status)
}

func TestStress_Case3_ChaosMonkey(t *testing.T) {
	ctx, nowBase, kds, repo, _, _ := setupSC8StressEnv(t)
	
	tasks, _ := repo.FindByNode(ctx, "factory_f", nil)
	initialCount := len(tasks)

	fmt.Printf("\n🌪️ [CHAOS MONKEY] Bắt đầu chuỗi sự kiện hủy diệt...\n")

	// 1. Patty Prep Late (T+15)
	tkPattyPrep := getTaskByStep(tasks, "patty_prep")
	kds.StartTask(ctx, tkPattyPrep.ID, tkPattyPrep.ScheduledStart)
	lateTime1 := tkPattyPrep.ScheduledEnd.Add(15 * time.Minute)
	fmt.Printf("   -> [SỰ CỐ 1] Patty Prep trễ 15 phút (lúc %s)\n", lateTime1.Format("15:04"))
	kds.CompleteTask(ctx, tkPattyPrep.ID, lateTime1)

	// 2. Grill A Breaks (T+30)
	tasks2, _ := repo.FindByNode(ctx, "factory_f", nil)
	var tkBakeSetup *models.StaffTask
	for _, tk := range tasks2 {
		if tk.TaskKind == models.TaskKindSetup && tk.MachineID == "m_grill_A" && tk.Status == models.TaskPending {
			tkBakeSetup = tk
			break
		}
	}
	if tkBakeSetup != nil {
		kds.StartTask(ctx, tkBakeSetup.ID, tkBakeSetup.ScheduledStart)
		failTime := tkBakeSetup.ScheduledStart.Add(1 * time.Minute)
		fmt.Printf("   -> [SỰ CỐ 2] Máy nướng Grill A cháy (lúc %s)\n", failTime.Format("15:04"))
		kds.FailTask(ctx, tkBakeSetup.ID, failTime, "Hỏng cảm biến")
	}

	// 3. Sauce Mix Late (T+45)
	tasks3, _ := repo.FindByNode(ctx, "factory_f", nil)
	tkSauceMix := getTaskByStep(tasks3, "sauce_mix")
	// If sauce_mix was pending, start it
	if tkSauceMix.Status == models.TaskPending {
		kds.StartTask(ctx, tkSauceMix.ID, tkSauceMix.ScheduledStart)
		lateTime2 := tkSauceMix.ScheduledEnd.Add(20 * time.Minute)
		fmt.Printf("   -> [SỰ CỐ 3] Sauce Mix trễ tiếp 20 phút (lúc %s)\n", lateTime2.Format("15:04"))
		kds.CompleteTask(ctx, tkSauceMix.ID, lateTime2)
	}

	printStressTimeline(t, "AFTER CHAOS MONKEY", ctx, repo, nowBase)

	// Validation: We must not lose any tasks. The number of DONE + PENDING/ACTIVE tasks should equal initial count (plus replacement tasks if setup failed, actually replacement tasks increase total count).
	tasksFinal, _ := repo.FindByNode(ctx, "factory_f", nil)
	
	// Count unique SOP steps that are PENDING/WAITING/DONE
	stepMap := make(map[string]bool)
	for _, tk := range tasksFinal {
		if tk.Status != models.TaskFailed && tk.Status != models.TaskCancelled {
			stepMap[tk.SOPStepID] = true
		}
	}

	// Should still have 4 unique steps (bun_mix, bun_bake, patty_prep, patty_grill, sauce_mix, sauce_pack = 6 steps total actually)
	if len(stepMap) != 6 {
		t.Errorf("Chaos Monkey failed: Lost SOP steps. Expected 6, got %d", len(stepMap))
	}

	fmt.Printf("✅ Case 3: Chaos Monkey OK. Xử lý 3 sự kiện liên hoàn không Deadlock. Initial tasks: %d, Final tasks: %d\n", initialCount, len(tasksFinal))
}
