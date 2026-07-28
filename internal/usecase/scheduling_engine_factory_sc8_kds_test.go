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

// setupSC8Env creates a simple test environment for KDS execution testing.
func setupSC8Env(t *testing.T) (context.Context, *models.ProductionOrder, time.Time, services.StaffTaskUseCase, services.StaffTaskRepository, *models.Machine) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "factory_f"

	// Staff
	shiftRepo.shifts["shift_1"] = &models.StaffShift{ID: "shift_1", NodeID: nodeID, StaffID: "f_staff_1", Status: models.ShiftActive}

	// Machines
	grillA := &models.Machine{ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 50}
	grillB := &models.Machine{ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill", Status: models.MachineIdle, MaxCapacity: 50}
	machineRepo.machines["m_grill_A"] = grillA
	machineRepo.machines["m_grill_B"] = grillB

	// Simple SOP
	sopBun := "sop_bun_kds"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["step_mix"] = &models.SOPStep{ID: "step_mix", SOPID: sopBun, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["step_bake"] = &models.SOPStep{
		ID: "step_bake", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"step_mix"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), SlotConsumption: 1.0,
	}
	sopRepo.steps["step_pack"] = &models.SOPStep{ID: "step_pack", SOPID: sopBun, SeqNo: 3, DependsOn: []string{"step_bake"}, Duration: 10 * 60}

	poBun := &models.ProductionOrder{
		ID: "po_bun_kds", NodeID: nodeID, ItemID: "item_bun", Status: models.POInProgress, TargetQty: 100, SOPID: sopBun,
	}
	_ = poRepo.Create(ctx, poBun)

	// Create Usecase
	kdsUseCase := NewStaffTaskUseCase(taskRepo, machineRepo, engine)

	// 1. Initial Schedule
	_, err := engine.SchedulePO(ctx, poBun.ID)
	if err != nil {
		t.Fatalf("Failed to schedule PO: %v", err)
	}

	return ctx, poBun, nowBase, kdsUseCase, taskRepo, grillA
}

func getSortedTasks(ctx context.Context, repo services.StaffTaskRepository, nodeID string) []*models.StaffTask {
	tasks, _ := repo.FindByNode(ctx, nodeID, nil)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ScheduledStart.Before(tasks[j].ScheduledStart)
	})
	return tasks
}

func TestKDS_Case1_HappyPath(t *testing.T) {
	ctx, _, _, kds, repo, _ := setupSC8Env(t)
	tasks := getSortedTasks(ctx, repo, "factory_f")

	// Task 1: Mix
	tk1 := tasks[0]
	fmt.Printf("[KDS] Bắt đầu làm task 1 (%s) lúc %s...\n", tk1.SOPStepID, tk1.ScheduledStart.Format("15:04"))
	if err := kds.StartTask(ctx, tk1.ID, tk1.ScheduledStart); err != nil {
		t.Fatalf("StartTask error: %v", err)
	}
	
	// Hoàn thành đúng giờ
	fmt.Printf("[KDS] Hoàn thành task 1 lúc %s...\n", tk1.ScheduledEnd.Format("15:04"))
	if err := kds.CompleteTask(ctx, tk1.ID, tk1.ScheduledEnd); err != nil {
		t.Fatalf("CompleteTask error: %v", err)
	}

	// Kiểm tra lịch các task sau không bị dời
	newTasks := getSortedTasks(ctx, repo, "factory_f")
	if newTasks[1].ScheduledStart != tasks[1].ScheduledStart {
		t.Errorf("Happy Path failed: Downstream tasks should not be rescheduled. Expected %v, got %v", tasks[1].ScheduledStart, newTasks[1].ScheduledStart)
	}
	fmt.Println("✅ Case 1: Happy Path OK. Lịch không thay đổi.")
}

func TestKDS_Case2_LateDrift(t *testing.T) {
	ctx, _, _, kds, repo, _ := setupSC8Env(t)
	tasks := getSortedTasks(ctx, repo, "factory_f")

	// Task 1: Mix
	tk1 := tasks[0]
	tk2OrigStart := tasks[1].ScheduledStart // Save original time
	
	kds.StartTask(ctx, tk1.ID, tk1.ScheduledStart)
	
	// Hoàn thành TRỄ 20 phút
	lateTime := tk1.ScheduledEnd.Add(20 * time.Minute)
	fmt.Printf("[KDS] Hoàn thành task 1 TRỄ lúc %s (Dự kiến %s)...\n", lateTime.Format("15:04"), tk1.ScheduledEnd.Format("15:04"))
	kds.CompleteTask(ctx, tk1.ID, lateTime)

	// Kiểm tra lịch các task sau đã bị dời lùi lại
	newTasks := getSortedTasks(ctx, repo, "factory_f")
	tk2New := newTasks[1]
	
	if !tk2New.ScheduledStart.After(tk2OrigStart) {
		t.Errorf("Late Drift failed: Downstream task was not delayed. Expected after %v, got %v", tk2OrigStart, tk2New.ScheduledStart)
	}
	fmt.Printf("✅ Case 2: Late Drift OK. Task 2 dời từ %s -> %s\n", tk2OrigStart.Format("15:04"), tk2New.ScheduledStart.Format("15:04"))
}

func TestKDS_Case3_FailTask_MachineBreakdown(t *testing.T) {
	ctx, _, _, kds, repo, grillA := setupSC8Env(t)
	tasks := getSortedTasks(ctx, repo, "factory_f")

	// Tìm task Bake Setup
	var tkBake *models.StaffTask
	for _, tk := range tasks {
		if tk.SOPStepID == "step_bake" && tk.TaskKind == models.TaskKindSetup {
			tkBake = tk
			break
		}
	}

	kds.StartTask(ctx, tkBake.ID, tkBake.ScheduledStart)
	
	// Đang nướng thì hỏng máy lúc +5 mins
	failTime := tkBake.ScheduledStart.Add(5 * time.Minute)
	fmt.Printf("[KDS] Báo hỏng máy %s lúc %s...\n", tkBake.MachineID, failTime.Format("15:04"))
	if err := kds.FailTask(ctx, tkBake.ID, failTime, "Cháy khét"); err != nil {
		t.Fatalf("FailTask error: %v", err)
	}

	if grillA.Status != models.MachineUnderMaintenance {
		t.Errorf("Machine status not updated to UNDER_MAINTENANCE")
	}

	// Lấy lại danh sách task xem Dispatcher có re-assign không
	newTasks := getSortedTasks(ctx, repo, "factory_f")
	var tkBakeNew *models.StaffTask
	for _, tk := range newTasks {
		// Tìm task QUEUED/PENDING mới được tạo thay thế (cùng BatchIndex = 0)
		if tk.SOPStepID == "step_bake" && tk.TaskKind == models.TaskKindSetup && tk.Status == models.TaskPending {
			tkBakeNew = tk
			break
		}
	}

	if tkBakeNew == nil {
		t.Fatalf("Case 3 failed: Dispatcher did not re-create/re-assign the failed task")
	}

	fmt.Printf("✅ Case 3: Fail Task OK. Máy hỏng, Dispatcher tự động re-assign sang %s\n", tkBakeNew.MachineID)
}
