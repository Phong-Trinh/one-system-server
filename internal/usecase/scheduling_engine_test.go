package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

func setupTestEnv() (
	context.Context,
	*mockProductionOrderRepo,
	*mockSOPRepo,
	*mockProductionBatchRepo,
	*mockStaffShiftRepo,
	*mockStaffTaskRepo,
	*mockMachineRepo,
	usecase.Dispatcher,
	usecase.SchedulingEngine,
) {
	ctx := context.Background()

	poRepo := newMockProductionOrderRepo()
	sopRepo := newMockSOPRepo()
	batchRepo := newMockProductionBatchRepo()
	shiftRepo := newMockStaffShiftRepo()
	taskRepo := newMockStaffTaskRepo()
	machineRepo := newMockMachineRepo()

	// Setup Dispatcher
	dispatcher := usecase.NewDispatcher(shiftRepo, machineRepo, batchRepo, taskRepo, sopRepo)

	// Setup SchedulingEngine
	engine := usecase.NewSchedulingEngine(poRepo, sopRepo, taskRepo, dispatcher)

	return ctx, poRepo, sopRepo, batchRepo, shiftRepo, taskRepo, machineRepo, dispatcher, engine
}

// ─── HELPER FUNC ─────────────────────────────────────────────────────────────

func ptrInt(i int) *int                                                { return &i }
func ptrAttentionLevel(l models.AttentionLevel) *models.AttentionLevel { return &l }

// ─── T1: Single Step PO (Happy Path) ─────────────────────────────────────────

func TestSchedulingEngine_T1_SingleStepPO(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	_ = taskRepo // avoid unused in case it's only used for one test

	nodeID := "node_1"
	staffID := "staff_minh"
	equipTypeID := "equip_fryer"
	machineID := "machine_fryer_01"
	poID := "po_1"
	sopID := "sop_1"

	// 1. Setup Machine
	machineRepo.machines[machineID] = &models.Machine{
		ID:              machineID,
		NodeID:          nodeID,
		EquipmentTypeID: equipTypeID,
		Status:          models.MachineIdle,
	}

	// 2. Setup Staff Shift
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:        "shift_1",
		NodeID:    nodeID,
		StaffID:   staffID,
		Status:    models.ShiftActive,
		StationID: nil, // Flexible station
	}

	// 3. Setup SOP & Step
	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	step1 := &models.SOPStep{
		ID:              "step_1",
		SOPID:           sopID,
		SeqNo:           1,
		Duration:        300,
		EquipmentTypeID: &equipTypeID,
		IsIdleStep:      false,
	}
	sopRepo.steps[step1.ID] = step1

	// 4. Setup PO (IN_PROGRESS)
	po := &models.ProductionOrder{
		ID:     poID,
		NodeID: nodeID,
		SOPID:  sopID,
		Status: models.POInProgress,
	}
	poRepo.pos[poID] = po

	// 5. Run SchedulePO
	createdTasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}

	// 6. Verify result
	if len(createdTasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(createdTasks))
	}

	task := createdTasks[0]
	if task.TaskKind != models.TaskKindNormal {
		t.Errorf("Expected TaskKindNormal, got %s", task.TaskKind)
	}
	if task.MachineID != machineID {
		t.Errorf("Expected MachineID=%s, got %s", machineID, task.MachineID)
	}
	if task.AssignedTo != staffID {
		t.Errorf("Expected AssignedTo=%s, got %s", staffID, task.AssignedTo)
	}
	if task.Status != models.TaskPending {
		t.Errorf("Expected TaskPending, got %s", task.Status)
	}
	expectedDuration := time.Duration(step1.Duration) * time.Second
	actualDuration := task.ScheduledEnd.Sub(task.ScheduledStart)
	if actualDuration != expectedDuration {
		t.Errorf("Expected duration %v, got %v", expectedDuration, actualDuration)
	}
}

// ─── T2: Idle Step, FULL_IDLE, Fill-In Fit ───────────────────────────────────

func TestSchedulingEngine_T2_IdleStep_FillIn(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcher, engine := setupTestEnv()

	nodeID := "node_1"
	staffID := "staff_minh"
	poID := "po_1"
	sopID := "sop_1"

	// 1. Setup Staff (Flexible)
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:      "shift_1",
		NodeID:  nodeID,
		StaffID: staffID,
		Status:  models.ShiftActive,
	}

	// 2. Setup SOP: Idle step (FULL_IDLE)
	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	idleStep := &models.SOPStep{
		ID:                  "step_idle",
		SOPID:               sopID,
		SeqNo:               1,
		Duration:            600,
		ActiveTime:          ptrInt(60),  // Setup takes 60s
		RequiresAttentionAt: ptrInt(120), // Retrieve takes 120s
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionFullIdle,
	}
	sopRepo.steps[idleStep.ID] = idleStep

	poRepo.pos[poID] = &models.ProductionOrder{
		ID:     poID,
		NodeID: nodeID,
		SOPID:  sopID,
		Status: models.POInProgress,
	}

	// 3. Create a pending fill-in candidate task (QUEUED)
	fillInSOP := "sop_2"
	fillInStep := &models.SOPStep{
		ID:         "step_fillin",
		SOPID:      fillInSOP,
		Duration:   120, // 120s (fits in 420s window)
		IsIdleStep: false,
	}
	sopRepo.steps[fillInStep.ID] = fillInStep

	candidateTask := &models.StaffTask{
		ID:        "task_candidate",
		NodeID:    nodeID,
		POID:      "po_2",
		SOPStepID: fillInStep.ID,
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}
	taskRepo.tasks[candidateTask.ID] = candidateTask

	// 4. Run SchedulePO (creates SETUP + RETRIEVE for idleStep, assigns staff)
	createdTasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}

	if len(createdTasks) != 2 {
		t.Fatalf("Expected 2 tasks (SETUP, RETRIEVE), got %d", len(createdTasks))
	}

	// Because Dispatch is called inside SchedulePO, both SETUP and RETRIEVE are PENDING.
	// But Dispatcher.assignFillInTasks runs inside Dispatch, it might have assigned the fill-in!

	// Wait, let's just trigger dispatcher manually to make sure:
	dispatcher.Dispatch(ctx, nodeID)

	// 5. Verify Fill-In task
	candidateTask = taskRepo.tasks["task_candidate"]
	if candidateTask.TaskKind != models.TaskKindFillIn {
		t.Errorf("Expected TaskKindFillIn, got %s", candidateTask.TaskKind)
	}
	if candidateTask.Status != models.TaskPending {
		t.Errorf("Expected TaskPending, got %s", candidateTask.Status)
	}
	if candidateTask.AssignedTo != staffID {
		t.Errorf("Expected AssignedTo staff_minh, got %s", candidateTask.AssignedTo)
	}
	if candidateTask.ParentTaskID == nil {
		t.Errorf("Expected ParentTaskID to be set")
	} else {
		// Parent should be the RETRIEVE task
		var retrieveTask *models.StaffTask
		for _, t := range createdTasks {
			if t.TaskKind == models.TaskKindRetrieve {
				retrieveTask = t
				break
			}
		}
		if *candidateTask.ParentTaskID != retrieveTask.ID {
			t.Errorf("Expected ParentTaskID=%s, got %s", retrieveTask.ID, *candidateTask.ParentTaskID)
		}
	}
}

// ─── T3: Idle Step, FULL_IDLE, Fill-In Too Long ──────────────────────────────

func TestSchedulingEngine_T3_IdleStep_FillIn_TooLong(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcher, engine := setupTestEnv()

	nodeID := "node_1"
	staffID := "staff_minh"
	poID := "po_1"
	sopID := "sop_1"

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:      "shift_1",
		NodeID:  nodeID,
		StaffID: staffID,
		Status:  models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	idleStep := &models.SOPStep{
		ID:                  "step_idle",
		SOPID:               sopID,
		SeqNo:               1,
		Duration:            600,
		ActiveTime:          ptrInt(60),
		RequiresAttentionAt: ptrInt(120), // available = 600 - 60 - 120 = 420. buf = 30 -> 390
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionFullIdle,
	}
	sopRepo.steps[idleStep.ID] = idleStep
	poRepo.pos[poID] = &models.ProductionOrder{
		ID:     poID,
		NodeID: nodeID,
		SOPID:  sopID,
		Status: models.POInProgress,
	}

	fillInStep := &models.SOPStep{
		ID:         "step_fillin_long",
		Duration:   500, // 500s > 450s window
		IsIdleStep: false,
	}
	sopRepo.steps[fillInStep.ID] = fillInStep

	candidateTask := &models.StaffTask{
		ID:        "task_candidate",
		NodeID:    nodeID,
		SOPStepID: fillInStep.ID,
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}
	taskRepo.tasks[candidateTask.ID] = candidateTask

	engine.SchedulePO(ctx, poID)
	dispatcher.Dispatch(ctx, nodeID)

	candidateTask = taskRepo.tasks["task_candidate"]
	if candidateTask.TaskKind == models.TaskKindFillIn {
		t.Errorf("Candidate should not be assigned as FILL_IN (too long)")
	}
}

// ─── T4: Idle Step, ACTIVE_WAIT ──────────────────────────────────────────────

func TestSchedulingEngine_T4_IdleStep_ActiveWait(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcher, engine := setupTestEnv()

	nodeID := "node_1"
	staffID := "staff_minh"
	poID := "po_1"
	sopID := "sop_1"

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:      "shift_1",
		NodeID:  nodeID,
		StaffID: staffID,
		Status:  models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	idleStep := &models.SOPStep{
		ID:                  "step_idle",
		SOPID:               sopID,
		Duration:            600,
		ActiveTime:          ptrInt(60),
		RequiresAttentionAt: ptrInt(120),
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionActiveWait, // Active wait = no fill in
	}
	sopRepo.steps[idleStep.ID] = idleStep
	poRepo.pos[poID] = &models.ProductionOrder{
		ID:     poID,
		NodeID: nodeID,
		SOPID:  sopID,
		Status: models.POInProgress,
	}

	fillInStep := &models.SOPStep{
		ID:         "step_fillin",
		Duration:   30, // Extremely short, but shouldn't matter
		IsIdleStep: false,
	}
	sopRepo.steps[fillInStep.ID] = fillInStep

	candidateTask := &models.StaffTask{
		ID:        "task_candidate",
		NodeID:    nodeID,
		SOPStepID: fillInStep.ID,
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
	}
	taskRepo.tasks[candidateTask.ID] = candidateTask

	engine.SchedulePO(ctx, poID)
	dispatcher.Dispatch(ctx, nodeID)

	candidateTask = taskRepo.tasks["task_candidate"]
	if candidateTask.TaskKind == models.TaskKindFillIn {
		t.Errorf("Candidate should not be assigned as FILL_IN due to ACTIVE_WAIT")
	}
}

// ─── T5: Idle Step, PERIODIC_CHECK ───────────────────────────────────────────

func TestSchedulingEngine_T5_IdleStep_PeriodicCheck(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcher, engine := setupTestEnv()

	nodeID := "node_1"
	staffID := "staff_minh"
	poID := "po_1"
	sopID := "sop_1"

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:      "shift_1",
		NodeID:  nodeID,
		StaffID: staffID,
		Status:  models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	idleStep := &models.SOPStep{
		ID:                  "step_idle",
		SOPID:               sopID,
		Duration:            600,
		ActiveTime:          ptrInt(60),
		RequiresAttentionAt: ptrInt(120),
		CheckIntervalSec:    ptrInt(90), // Periodic check every 90s (90-30 = 60s window)
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionPeriodicCheck,
	}
	sopRepo.steps[idleStep.ID] = idleStep
	poRepo.pos[poID] = &models.ProductionOrder{
		ID:     poID,
		NodeID: nodeID,
		SOPID:  sopID,
		Status: models.POInProgress,
	}

	// Task A: 50s (fits in 60s window)
	sopRepo.steps["step_a"] = &models.SOPStep{ID: "step_a", Duration: 50}
	taskA := &models.StaffTask{
		ID: "task_a", NodeID: nodeID, SOPStepID: "step_a",
		TaskKind: models.TaskKindNormal, Status: models.TaskQueued, CreatedAt: time.Now(),
	}
	taskRepo.tasks[taskA.ID] = taskA

	// Task B: 70s (does not fit in 60s window)
	sopRepo.steps["step_b"] = &models.SOPStep{ID: "step_b", Duration: 70}
	taskB := &models.StaffTask{
		ID: "task_b", NodeID: nodeID, SOPStepID: "step_b",
		TaskKind: models.TaskKindNormal, Status: models.TaskQueued, CreatedAt: time.Now().Add(time.Second),
	}
	taskRepo.tasks[taskB.ID] = taskB

	engine.SchedulePO(ctx, poID)
	dispatcher.Dispatch(ctx, nodeID)

	taskA = taskRepo.tasks["task_a"]
	taskB = taskRepo.tasks["task_b"]

	if taskA.TaskKind != models.TaskKindFillIn {
		t.Errorf("taskA (50s) should be FILL_IN for PERIODIC_CHECK")
	}
	if taskB.TaskKind == models.TaskKindFillIn {
		t.Errorf("taskB (70s) should NOT be FILL_IN for PERIODIC_CHECK")
	}
}

// ─── T6: Multi-Step Linear SOP ───────────────────────────────────────────────

func TestSchedulingEngine_T6_MultiStep_Linear(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, _, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_6"
	sopID := "sop_6"

	shiftRepo.shifts["shift_fryer"] = &models.StaffShift{
		ID: "shift_fryer", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
		StationID: ptrString("equip_fryer"),
	}
	shiftRepo.shifts["shift_grill"] = &models.StaffShift{
		ID: "shift_grill", NodeID: nodeID, StaffID: "an", Status: models.ShiftActive,
		StationID: ptrString("equip_grill"),
	}
	shiftRepo.shifts["shift_manual"] = &models.StaffShift{
		ID: "shift_manual", NodeID: nodeID, StaffID: "binh", Status: models.ShiftActive,
	}

	machineRepo.machines["m_fryer"] = &models.Machine{
		ID: "m_fryer", NodeID: nodeID, EquipmentTypeID: "equip_fryer", Status: models.MachineIdle,
	}
	machineRepo.machines["m_grill"] = &models.Machine{
		ID: "m_grill", NodeID: nodeID, EquipmentTypeID: "equip_grill", Status: models.MachineIdle,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}

	stepA := &models.SOPStep{ID: "step_a", SOPID: sopID, SeqNo: 1, Duration: 300, EquipmentTypeID: ptrString("equip_fryer")}
	stepB := &models.SOPStep{ID: "step_b", SOPID: sopID, SeqNo: 2, Duration: 200, EquipmentTypeID: ptrString("equip_grill"), DependsOn: []string{"step_a"}}
	stepC := &models.SOPStep{ID: "step_c", SOPID: sopID, SeqNo: 3, Duration: 60, DependsOn: []string{"step_b"}}

	sopRepo.steps[stepA.ID] = stepA
	sopRepo.steps[stepB.ID] = stepB
	sopRepo.steps[stepC.ID] = stepC

	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	createdTasks, _ := engine.SchedulePO(ctx, poID)
	if len(createdTasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(createdTasks))
	}

	// Because we use mocked topologies, let's verify dependency ordering via ScheduledStart/End
	var taskA, taskB, taskC *models.StaffTask
	for _, tk := range createdTasks {
		if tk.SOPStepID == "step_a" {
			taskA = tk
		}
		if tk.SOPStepID == "step_b" {
			taskB = tk
		}
		if tk.SOPStepID == "step_c" {
			taskC = tk
		}
	}

	if taskB.ScheduledStart.Before(taskA.ScheduledEnd) {
		t.Errorf("taskB should start AFTER taskA ends")
	}
	if taskC.ScheduledStart.Before(taskB.ScheduledEnd) {
		t.Errorf("taskC should start AFTER taskB ends")
	}
}

// ─── T7: Multi-Step Parallel SOP ─────────────────────────────────────────────

func TestSchedulingEngine_T7_MultiStep_Parallel(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, _, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_7"
	sopID := "sop_7"

	shiftRepo.shifts["s1"] = &models.StaffShift{ID: "s1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive, StationID: ptrString("fryer")}
	shiftRepo.shifts["s2"] = &models.StaffShift{ID: "s2", NodeID: nodeID, StaffID: "an", Status: models.ShiftActive, StationID: ptrString("fryer")}

	machineRepo.machines["m_f1"] = &models.Machine{ID: "m_f1", NodeID: nodeID, EquipmentTypeID: "fryer", Status: models.MachineIdle}
	machineRepo.machines["m_f2"] = &models.Machine{ID: "m_f2", NodeID: nodeID, EquipmentTypeID: "fryer", Status: models.MachineIdle}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}

	// A and B run in parallel (no deps)
	stepA := &models.SOPStep{ID: "step_a", SOPID: sopID, SeqNo: 1, Duration: 300, EquipmentTypeID: ptrString("fryer")}
	stepB := &models.SOPStep{ID: "step_b", SOPID: sopID, SeqNo: 2, Duration: 200, EquipmentTypeID: ptrString("fryer")}
	// C depends on A and B
	stepC := &models.SOPStep{ID: "step_c", SOPID: sopID, SeqNo: 3, Duration: 60, DependsOn: []string{"step_a", "step_b"}}

	sopRepo.steps[stepA.ID] = stepA
	sopRepo.steps[stepB.ID] = stepB
	sopRepo.steps[stepC.ID] = stepC

	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	createdTasks, _ := engine.SchedulePO(ctx, poID)

	var taskA, taskB, taskC *models.StaffTask
	for _, tk := range createdTasks {
		if tk.SOPStepID == "step_a" {
			taskA = tk
		}
		if tk.SOPStepID == "step_b" {
			taskB = tk
		}
		if tk.SOPStepID == "step_c" {
			taskC = tk
		}
	}

	// A and B should start at the same time
	if !taskA.ScheduledStart.Equal(taskB.ScheduledStart) {
		t.Errorf("taskA and taskB should start at the same time (parallel)")
	}

	// C should start after BOTH A and B end (A ends later since 300 > 200)
	if taskC.ScheduledStart.Before(taskA.ScheduledEnd) {
		t.Errorf("taskC should wait for taskA")
	}
}

// ─── T8: Burger Bun Real Recipe Simulation ───────────────────────────────────

func TestSchedulingEngine_T8_BurgerBunRealRecipe(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, dispatcher, engine := setupTestEnv()

	nodeID := "node_bakery"
	poID := "po_burger_bun"
	sopID := "sop_burger_bun"

	// 1. Setup Staff (1 Baker)
	shiftRepo.shifts["shift_baker"] = &models.StaffShift{
		ID: "shift_baker", NodeID: nodeID, StaffID: "baker_john", Status: models.ShiftActive,
	}

	// 2. Setup Machines (Mixer, Proofer, Oven)
	machineRepo.machines["m_mixer"] = &models.Machine{ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer", Status: models.MachineIdle}
	machineRepo.machines["m_proofer"] = &models.Machine{ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer", Status: models.MachineIdle}
	machineRepo.machines["m_oven"] = &models.Machine{ID: "m_oven", NodeID: nodeID, EquipmentTypeID: "oven", Status: models.MachineIdle}

	// 3. Setup SOP Steps
	sopRepo.sops[sopID] = &models.SOP{ID: sopID}

	steps := []*models.SOPStep{
		{ID: "s1_weigh", SOPID: sopID, SeqNo: 1, Duration: 3 * 60},
		{ID: "s2_mix1", SOPID: sopID, SeqNo: 2, DependsOn: []string{"s1_weigh"}, EquipmentTypeID: ptrString("mixer"),
			IsIdleStep: true, Duration: 10 * 60, ActiveTime: ptrInt(3 * 60), AttentionLevel: models.AttentionFullIdle},
		{ID: "s3_prep_butter", SOPID: sopID, SeqNo: 3, DependsOn: []string{"s1_weigh"}, Duration: 3 * 60}, // Parallel to mix1 idle
		{ID: "s3b_add_butter", SOPID: sopID, SeqNo: 4, DependsOn: []string{"s2_mix1", "s3_prep_butter"}, Duration: 1 * 60},
		{ID: "s4_mix2", SOPID: sopID, SeqNo: 5, DependsOn: []string{"s3b_add_butter"}, EquipmentTypeID: ptrString("mixer"),
			IsIdleStep: true, Duration: 25 * 60, ActiveTime: ptrInt(5 * 60), RequiresAttentionAt: ptrInt(5 * 60), AttentionLevel: models.AttentionFullIdle},
		{ID: "s5_shape", SOPID: sopID, SeqNo: 6, DependsOn: []string{"s4_mix2"}, Duration: 30 * 60},
		{ID: "s6_proof", SOPID: sopID, SeqNo: 7, DependsOn: []string{"s5_shape"}, EquipmentTypeID: ptrString("proofer"),
			IsIdleStep: true, Duration: 120 * 60, ActiveTime: ptrInt(5 * 60), AttentionLevel: models.AttentionFullIdle},
		{ID: "s7_bake", SOPID: sopID, SeqNo: 8, DependsOn: []string{"s6_proof"}, EquipmentTypeID: ptrString("oven"),
			IsIdleStep: true, Duration: 60 * 60, ActiveTime: ptrInt(5 * 60), RequiresAttentionAt: ptrInt(5 * 60),
			AttentionLevel: models.AttentionPeriodicCheck, CheckIntervalSec: ptrInt(15 * 60)},
		{ID: "s8_pack", SOPID: sopID, SeqNo: 9, DependsOn: []string{"s7_bake"}, Duration: 10 * 60},
	}

	stepNames := map[string]string{
		"s1_weigh":       "Cân 3.5kg bột mì số 13 từ vị trí abc bằng cân tiểu ly → cho vào cối 30L ở xyz",
		"s2_mix1":        "Cân & trộn 350g đường + 50g men + 60g muối → vào cối → bật mức 9 x 3 phút",
		"s2_mix1_ret":    "[MÁY ĐÁNH XONG] Nhấn nút dừng máy",
		"s3_prep_butter": "Lấy 16 quả trứng ở def đập ra thau → lấy 600g bơ ở opl → trộn đều",
		"s3b_add_butter": "Cho hỗn hợp bơ trứng vừa trộn vào cối",
		"s4_mix2":        "Cân 1L nước + 400g đá từ tủ đông vre → vào cối → bật mức 6 x 25 phút",
		"s4_mix2_ret":    "[MÁY ĐÁNH XONG] Tắt máy, chuẩn bị lấy bột ra",
		"s5_shape":       "Chia khối 90-93g → vo tròn tạo hình → vào mâm → bọc nilong → xếp lên xe ủ",
		"s6_proof":       "Đưa xe vào tủ ủ → cài đặt nhiệt độ, độ ẩm",
		"s6_proof_ret":   "[Ủ XONG] Kéo xe ủ ra khỏi tủ",
		"s7_bake":        "Phết trứng/dầu → rải mè → cho vào lò (Trên 190°C/Dưới 200°C) → hẹn 15p",
		"s7_bake_ret":    "[LÒ BÁO] Trở mâm / Lấy bánh ra khỏi lò",
		"s8_pack":        "Xếp 4 bánh/bịch size x → lấy bịch ở vị trí y",
		"s_fill1":        "Xé giấy nến lót mâm, 2 tờ/mâm theo chiều ngang",
		"s_fill2":        "Lau sạch bàn + rây bột lên bàn để chuẩn bị tạo hình",
	}

	for _, step := range steps {
		sopRepo.steps[step.ID] = step
	}

	poRepo.pos[poID] = &models.ProductionOrder{ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress}

	// 4. Create Fill-in tasks waiting in the pool
	// "Xé giấy nến lót mâm" (5 phút)
	sopRepo.steps["s_fill1"] = &models.SOPStep{ID: "s_fill1", Duration: 5 * 60}
	taskRepo.tasks["task_fill1"] = &models.StaffTask{
		ID: "task_fill1", POID: "po_other", NodeID: nodeID, SOPStepID: "s_fill1", TaskKind: models.TaskKindNormal, Status: models.TaskQueued, CreatedAt: time.Now(), Priority: 999,
	}
	// "Lau sạch bàn" (2 phút)
	sopRepo.steps["s_fill2"] = &models.SOPStep{ID: "s_fill2", Duration: 2 * 60}
	taskRepo.tasks["task_fill2"] = &models.StaffTask{
		ID: "task_fill2", POID: "po_other", NodeID: nodeID, SOPStepID: "s_fill2", TaskKind: models.TaskKindNormal, Status: models.TaskQueued, CreatedAt: time.Now().Add(time.Second), Priority: 999,
	}

	// 5. Execute Schedule & Dispatch
	engine.SchedulePO(ctx, poID)
	dispatcher.Dispatch(ctx, nodeID)

	// 6. Print Timeline
	allTasks, _ := taskRepo.FindByPO(ctx, poID)
	// Add fill in tasks to print list
	allTasks = append(allTasks, taskRepo.tasks["task_fill1"])
	allTasks = append(allTasks, taskRepo.tasks["task_fill2"])

	// Sort tasks by ScheduledStart
	for i := 0; i < len(allTasks)-1; i++ {
		for j := i + 1; j < len(allTasks); j++ {
			if allTasks[i].ScheduledStart.After(allTasks[j].ScheduledStart) {
				allTasks[i], allTasks[j] = allTasks[j], allTasks[i]
			}
		}
	}

	t.Logf("\n--- KẾT QUẢ SCHEDULE VỎ BÁNH BURGER (T8) ---")
	for _, tk := range allTasks {
		nameKey := tk.SOPStepID
		if tk.TaskKind == models.TaskKindRetrieve {
			nameKey = tk.SOPStepID + "_ret"
		}
		stepName := "Unknown"
		if name, ok := stepNames[nameKey]; ok {
			stepName = name
		} else if name, ok := stepNames[tk.SOPStepID]; ok {
			stepName = name // fallback
		}

		kindStr := string(tk.TaskKind)
		if tk.TaskKind == models.TaskKindFillIn {
			kindStr = "FILL-IN"
		} else if tk.TaskKind == models.TaskKindSetup {
			kindStr = "SETUP  "
		} else if tk.TaskKind == models.TaskKindRetrieve {
			kindStr = "RETRIEV"
		} else {
			kindStr = "NORMAL "
		}

		duration := tk.ScheduledEnd.Sub(tk.ScheduledStart).Minutes()
		startTime := tk.ScheduledStart.Format("15:04:05")
		endTime := tk.ScheduledEnd.Format("15:04:05")

		machineStr := ""
		if tk.MachineID != "" {
			machineStr = " [M: " + tk.MachineID + "]"
		}

		t.Logf("[%s - %s] %s | %-85s (%.0f min)%s",
			startTime, endTime, kindStr, stepName, duration, machineStr)
	}
	t.Logf("--------------------------------------------\n")
}

func ptrString(s string) *string { return &s }

// ─── T9: Machine Busy ─────────────────────────────────────────────────────────

func TestSchedulingEngine_T9_MachineBusy(t *testing.T) {
	ctx, poRepo, sopRepo, batchRepo, shiftRepo, _, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_9"
	sopID := "sop_9"
	equipTypeID := "equip_fryer"
	machineID := "fryer_busy_01"
	batchID := "batch_busy_001"

	estCompletion := time.Now().Add(180 * time.Second)
	batchRepo.batches[batchID] = &models.ProductionBatch{
		ID:                  batchID,
		EstimatedCompletion: &estCompletion,
	}

	ptrBatchID := batchID
	machineRepo.machines[machineID] = &models.Machine{
		ID:              machineID,
		NodeID:          nodeID,
		EquipmentTypeID: equipTypeID,
		Status:          models.MachineBusy,
		CurrentBatchID:  &ptrBatchID,
	}

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	step := &models.SOPStep{
		ID: "step_1", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	sopRepo.steps[step.ID] = step
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.MachineID != machineID {
		t.Errorf("Expected MachineID=%s, got %s", machineID, task.MachineID)
	}
	// ScheduledStart phải >= machineFreeAt (estCompletion)
	if task.ScheduledStart.Before(estCompletion) {
		t.Errorf("ScheduledStart=%v should be >= machineFreeAt=%v", task.ScheduledStart, estCompletion)
	}
	if task.Status != models.TaskPending {
		t.Errorf("Expected TaskPending after Dispatch, got %s", task.Status)
	}
}

// ─── T10: Staff Busy ──────────────────────────────────────────────────────────

func TestSchedulingEngine_T10_StaffBusy(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_10"
	sopID := "sop_10"
	equipTypeID := "equip_fryer"
	machineID := "fryer_01"
	staffID := "minh"

	// Staff Minh đã có 1 task PENDING với ScheduledEnd = now + 240s
	existingTaskEnd := time.Now().Add(240 * time.Second)
	taskRepo.tasks["existing_task"] = &models.StaffTask{
		ID:             "existing_task",
		NodeID:         nodeID,
		AssignedTo:     staffID,
		Status:         models.TaskPending,
		SOPStepID:      "some_step",
		TaskKind:       models.TaskKindNormal,
		ScheduledStart: time.Now(),
		ScheduledEnd:   existingTaskEnd,
	}

	machineRepo.machines[machineID] = &models.Machine{
		ID: machineID, NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: staffID, Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	step := &models.SOPStep{
		ID: "step_10", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	sopRepo.steps[step.ID] = step
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	// ScheduledStart phải >= existingTaskEnd (staff còn bận)
	if task.ScheduledStart.Before(existingTaskEnd) {
		t.Errorf("ScheduledStart=%v should be >= staffFreeAt=%v", task.ScheduledStart, existingTaskEnd)
	}
	if task.AssignedTo != staffID {
		t.Errorf("Expected AssignedTo=%s, got %s", staffID, task.AssignedTo)
	}
}

// ─── T11: Không Có Staff ─────────────────────────────────────────────────────

func TestSchedulingEngine_T11_NoStaff(t *testing.T) {
	ctx, poRepo, sopRepo, _, _, _, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_11"
	sopID := "sop_11"
	equipTypeID := "equip_fryer"

	// Không có active shift nào
	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	step := &models.SOPStep{
		ID: "step_11", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	sopRepo.steps[step.ID] = step
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	// Không được return error — graceful degradation
	if err != nil {
		t.Fatalf("SchedulePO should not error when no staff: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task even with no staff, got %d", len(tasks))
	}
	// Task được tạo nhưng unassigned
	task := tasks[0]
	if task.AssignedTo != "" {
		t.Errorf("Expected AssignedTo=\"\" (unassigned), got %s", task.AssignedTo)
	}
	// Status phải là QUEUED (Dispatcher không assign được → vẫn QUEUED)
	if task.Status != models.TaskQueued {
		t.Errorf("Expected TaskQueued (no staff to assign), got %s", task.Status)
	}
}

// ─── T12: Không Có Machine ───────────────────────────────────────────────────

func TestSchedulingEngine_T12_NoMachine(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, _, _, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_12"
	sopID := "sop_12"
	equipTypeID := "equip_fryer"

	// Không có machine nào cùng type
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	step := &models.SOPStep{
		ID: "step_12", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	sopRepo.steps[step.ID] = step
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO should not error when no machine: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task even with no machine, got %d", len(tasks))
	}
	task := tasks[0]
	if task.MachineID != "" {
		t.Errorf("Expected MachineID=\"\" (no machine), got %s", task.MachineID)
	}
	// Staff vẫn được assign dù không có machine
	if task.AssignedTo != "minh" {
		t.Errorf("Expected AssignedTo=minh, got %s", task.AssignedTo)
	}
}

// ─── T13: Cyclic Dependency → Error ──────────────────────────────────────────

func TestSchedulingEngine_T13_CyclicDependency(t *testing.T) {
	ctx, poRepo, sopRepo, _, _, taskRepo, _, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_13"
	sopID := "sop_13"

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	stepA := &models.SOPStep{ID: "step_a_13", SOPID: sopID, SeqNo: 1, Duration: 60, DependsOn: []string{"step_b_13"}}
	stepB := &models.SOPStep{ID: "step_b_13", SOPID: sopID, SeqNo: 2, Duration: 60, DependsOn: []string{"step_a_13"}}
	sopRepo.steps[stepA.ID] = stepA
	sopRepo.steps[stepB.ID] = stepB

	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	_, err := engine.SchedulePO(ctx, poID)
	if err == nil {
		t.Fatal("Expected ErrCyclicDependency, got nil")
	}
	if !containsError(err, usecase.ErrCyclicDependency) {
		t.Errorf("Expected ErrCyclicDependency, got: %v", err)
	}
	// Không có task nào được tạo
	if len(taskRepo.tasks) != 0 {
		t.Errorf("Expected 0 tasks created, got %d", len(taskRepo.tasks))
	}
}

// ─── T14: Invalid Dependency → Error ─────────────────────────────────────────

func TestSchedulingEngine_T14_InvalidDependency(t *testing.T) {
	ctx, poRepo, sopRepo, _, _, _, _, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_14"
	sopID := "sop_14"

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	stepA := &models.SOPStep{
		ID: "step_a_14", SOPID: sopID, SeqNo: 1, Duration: 60,
		DependsOn: []string{"nonexistent_step"},
	}
	sopRepo.steps[stepA.ID] = stepA

	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	_, err := engine.SchedulePO(ctx, poID)
	if err == nil {
		t.Fatal("Expected ErrInvalidDependency, got nil")
	}
	if !containsError(err, usecase.ErrInvalidDependency) {
		t.Errorf("Expected ErrInvalidDependency, got: %v", err)
	}
}

// ─── T15: Idempotency ────────────────────────────────────────────────────────

func TestSchedulingEngine_T15_Idempotency(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_15"
	sopID := "sop_15"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	sopRepo.steps["step_15"] = &models.SOPStep{
		ID: "step_15", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	// Lần 1
	tasks1, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO lần 1 failed: %v", err)
	}
	countAfterFirst := len(taskRepo.tasks)

	// Lần 2 — phải idempotent
	tasks2, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO lần 2 failed: %v", err)
	}
	countAfterSecond := len(taskRepo.tasks)

	if countAfterSecond != countAfterFirst {
		t.Errorf("Idempotency fail: taskRepo grew from %d to %d after second call", countAfterFirst, countAfterSecond)
	}
	if len(tasks1) != len(tasks2) {
		t.Errorf("Expected same task count both calls: %d vs %d", len(tasks1), len(tasks2))
	}
}

// ─── T16: RescheduleOnShiftChange — Staff Mới Vào Ca ─────────────────────────

func TestSchedulingEngine_T16_RescheduleOnShiftChange_NewStaff(t *testing.T) {
	ctx, _, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}

	// SOPStep cần để Dispatcher load
	sopRepo.steps["step_q"] = &models.SOPStep{
		ID: "step_q", SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}

	// Có sẵn 1 QUEUED task chưa được assign
	taskRepo.tasks["task_queued"] = &models.StaffTask{
		ID:        "task_queued",
		NodeID:    nodeID,
		SOPStepID: "step_q",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}

	// Staff mới bắt đầu ca
	shiftRepo.shifts["shift_new"] = &models.StaffShift{
		ID: "shift_new", NodeID: nodeID, StaffID: "an", Status: models.ShiftActive,
	}

	err := engine.RescheduleOnShiftChange(ctx, nodeID)
	if err != nil {
		t.Fatalf("RescheduleOnShiftChange failed: %v", err)
	}

	task := taskRepo.tasks["task_queued"]
	if task.AssignedTo == "" {
		t.Errorf("Expected task to be assigned to new staff, still unassigned")
	}
	if task.Status != models.TaskPending {
		t.Errorf("Expected task Status=PENDING after dispatch, got %s", task.Status)
	}
}

// ─── T17: RescheduleOnShiftChange — Staff Ra Ca (Phase 1: no auto-reassign) ──

func TestSchedulingEngine_T17_RescheduleOnShiftChange_StaffLeave(t *testing.T) {
	ctx, _, _, _, shiftRepo, taskRepo, _, _, engine := setupTestEnv()

	nodeID := "node_1"

	// Staff Minh ra ca (không có active shift nào)
	// Nhưng đã có task PENDING assign cho Minh
	taskRepo.tasks["task_pending"] = &models.StaffTask{
		ID:         "task_pending",
		NodeID:     nodeID,
		SOPStepID:  "some_step",
		AssignedTo: "minh",
		Status:     models.TaskPending,
		TaskKind:   models.TaskKindNormal,
		CreatedAt:  time.Now(),
	}

	// Không có active shift nào (Minh đã ra ca, chưa có người thay)
	_ = shiftRepo

	err := engine.RescheduleOnShiftChange(ctx, nodeID)
	// Phase 1: không reassign PENDING tasks, không return error
	if err != nil {
		t.Fatalf("RescheduleOnShiftChange should not error: %v", err)
	}

	// PENDING task vẫn giữ nguyên (Phase 2 mới reassign)
	task := taskRepo.tasks["task_pending"]
	if task.AssignedTo != "minh" {
		t.Errorf("Phase 1: PENDING task should not be auto-reassigned, got AssignedTo=%s", task.AssignedTo)
	}
}

// ─── T18: PO Not Found ───────────────────────────────────────────────────────

func TestSchedulingEngine_T18_PONotFound(t *testing.T) {
	ctx, _, _, _, _, _, _, _, engine := setupTestEnv()

	_, err := engine.SchedulePO(ctx, "nonexistent_po_id")
	if err == nil {
		t.Fatal("Expected ErrPONotFound, got nil")
	}
	if !containsError(err, usecase.ErrPONotFound) {
		t.Errorf("Expected ErrPONotFound, got: %v", err)
	}
}

// ─── T19: PO Not In Progress ─────────────────────────────────────────────────

func TestSchedulingEngine_T19_PONotInProgress(t *testing.T) {
	ctx, poRepo, _, _, _, _, _, _, engine := setupTestEnv()

	poRepo.pos["po_pending"] = &models.ProductionOrder{
		ID:     "po_pending",
		NodeID: "node_1",
		SOPID:  "sop_1",
		Status: models.POPending, // Chưa IN_PROGRESS
	}

	_, err := engine.SchedulePO(ctx, "po_pending")
	if err == nil {
		t.Fatal("Expected ErrPONotInProgress, got nil")
	}
	if !containsError(err, usecase.ErrPONotInProgress) {
		t.Errorf("Expected ErrPONotInProgress, got: %v", err)
	}
}

// ─── T20: Idle Step — Không Có Fill-In Task Available ────────────────────────

func TestSchedulingEngine_T20_IdleStep_NoFillInAvailable(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_20"
	sopID := "sop_20"

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	idleStep := &models.SOPStep{
		ID:                  "step_idle_20",
		SOPID:               sopID,
		SeqNo:               1,
		Duration:            600,
		ActiveTime:          ptrInt(60),
		RequiresAttentionAt: ptrInt(120),
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionFullIdle,
	}
	sopRepo.steps[idleStep.ID] = idleStep
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	// Không có fill-in candidate nào trong pool

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	// Chỉ có SETUP + RETRIEVE — không error dù không có fill-in
	setupCount, retrieveCount := 0, 0
	for _, tk := range tasks {
		if tk.SOPStepID == idleStep.ID {
			if tk.TaskKind == models.TaskKindSetup {
				setupCount++
			} else if tk.TaskKind == models.TaskKindRetrieve {
				retrieveCount++
			}
		}
	}
	if setupCount != 1 || retrieveCount != 1 {
		t.Errorf("Expected 1 SETUP + 1 RETRIEVE, got setup=%d retrieve=%d", setupCount, retrieveCount)
	}

	// Không có fill-in task nào
	for _, tk := range taskRepo.tasks {
		if tk.TaskKind == models.TaskKindFillIn {
			t.Errorf("Unexpected FILL_IN task created when no candidates available")
		}
	}
}

// ─── T21: Idle Window Quá Nhỏ → Không Fill-In ───────────────────────────────

func TestSchedulingEngine_T21_IdleWindowTooSmall(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcher, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_21"
	sopID := "sop_21"

	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	// idleWindow = duration(90) - active_time(60) - requires_attention_at(20) = 10s < safetyBuffer(30s)
	idleStep := &models.SOPStep{
		ID:                  "step_idle_21",
		SOPID:               sopID,
		SeqNo:               1,
		Duration:            90,
		ActiveTime:          ptrInt(60),
		RequiresAttentionAt: ptrInt(20),
		IsIdleStep:          true,
		AttentionLevel:      models.AttentionFullIdle,
	}
	sopRepo.steps[idleStep.ID] = idleStep
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	// Candidate task với duration ngắn (5s)
	sopRepo.steps["step_fill_21"] = &models.SOPStep{ID: "step_fill_21", Duration: 5}
	taskRepo.tasks["candidate_21"] = &models.StaffTask{
		ID:        "candidate_21",
		NodeID:    nodeID,
		SOPStepID: "step_fill_21",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}

	engine.SchedulePO(ctx, poID)
	dispatcher.Dispatch(ctx, nodeID)

	// Candidate không được assign làm fill-in vì window quá nhỏ
	candidate := taskRepo.tasks["candidate_21"]
	if candidate.TaskKind == models.TaskKindFillIn {
		t.Errorf("Candidate should NOT be FILL_IN when idle window < safetyBuffer")
	}
}

// ─── T22: Flexible Staff (StationID = nil) ───────────────────────────────────

func TestSchedulingEngine_T22_FlexibleStaff(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, _, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_22"
	sopID := "sop_22"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}

	// Staff flexible: StationID = nil (không gắn station cứng)
	shiftRepo.shifts["shift_flex"] = &models.StaffShift{
		ID:        "shift_flex",
		NodeID:    nodeID,
		StaffID:   "flexible_staff",
		Status:    models.ShiftActive,
		StationID: nil, // flexible
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	sopRepo.steps["step_22"] = &models.SOPStep{
		ID: "step_22", SOPID: sopID, SeqNo: 1, Duration: 300,
		EquipmentTypeID: &equipTypeID,
	}
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}
	// Flexible staff được assign dù không có station cứng
	if tasks[0].AssignedTo != "flexible_staff" {
		t.Errorf("Expected flexible_staff to be assigned, got %s", tasks[0].AssignedTo)
	}
}

// ─── T23: FIFO Staff Pick (Pick Staff Free Sớm Nhất) ─────────────────────────

func TestSchedulingEngine_T23_FIFOStaffPick(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_23"
	sopID := "sop_23"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}

	// Minh: freeAt = now + 300s (bận hơn)
	shiftRepo.shifts["shift_minh"] = &models.StaffShift{
		ID: "shift_minh", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
		StationID: ptrString(equipTypeID),
	}
	taskRepo.tasks["minh_task"] = &models.StaffTask{
		ID: "minh_task", NodeID: nodeID, AssignedTo: "minh",
		Status: models.TaskPending, SOPStepID: "s_x", TaskKind: models.TaskKindNormal,
		ScheduledStart: time.Now(), ScheduledEnd: time.Now().Add(300 * time.Second),
	}

	// An: freeAt = now + 100s (rảnh sớm hơn)
	shiftRepo.shifts["shift_an"] = &models.StaffShift{
		ID: "shift_an", NodeID: nodeID, StaffID: "an", Status: models.ShiftActive,
		StationID: ptrString(equipTypeID),
	}
	taskRepo.tasks["an_task"] = &models.StaffTask{
		ID: "an_task", NodeID: nodeID, AssignedTo: "an",
		Status: models.TaskPending, SOPStepID: "s_y", TaskKind: models.TaskKindNormal,
		ScheduledStart: time.Now(), ScheduledEnd: time.Now().Add(100 * time.Second),
	}

	// Dummy steps for the existing tasks (sopRepo lookup)
	sopRepo.steps["s_x"] = &models.SOPStep{ID: "s_x", Duration: 300}
	sopRepo.steps["s_y"] = &models.SOPStep{ID: "s_y", Duration: 100}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	sopRepo.steps["step_23"] = &models.SOPStep{
		ID: "step_23", SOPID: sopID, SeqNo: 1, Duration: 60,
		EquipmentTypeID: &equipTypeID,
	}
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}
	// An được pick vì freeAt sớm hơn
	if tasks[0].AssignedTo != "an" {
		t.Errorf("Expected FIFO pick 'an' (freeAt=+100s), got %s", tasks[0].AssignedTo)
	}
}

// ─── T24: Manual Step (EquipmentTypeID = nil) ────────────────────────────────

func TestSchedulingEngine_T24_ManualStep(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, _, _, _, engine := setupTestEnv()

	nodeID := "node_1"
	poID := "po_24"
	sopID := "sop_24"

	shiftRepo.shifts["shift_flex"] = &models.StaffShift{
		ID: "shift_flex", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
		StationID: nil,
	}

	sopRepo.sops[sopID] = &models.SOP{ID: sopID}
	// Manual step: EquipmentTypeID = nil
	sopRepo.steps["step_24"] = &models.SOPStep{
		ID: "step_24", SOPID: sopID, SeqNo: 1, Duration: 120,
		EquipmentTypeID: nil, // manual
	}
	poRepo.pos[poID] = &models.ProductionOrder{
		ID: poID, NodeID: nodeID, SOPID: sopID, Status: models.POInProgress,
	}

	tasks, err := engine.SchedulePO(ctx, poID)
	if err != nil {
		t.Fatalf("SchedulePO failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}
	task := tasks[0]
	if task.MachineID != "" {
		t.Errorf("Manual step: expected MachineID=\"\", got %s", task.MachineID)
	}
	if task.AssignedTo == "" {
		t.Errorf("Manual step: staff should still be assigned, got empty")
	}
}

// ─── T25: Dispatcher — QUEUED Task + Staff Rảnh → PENDING ───────────────────

func TestSchedulingEngine_T25_Dispatch_AssignQueuedTask(t *testing.T) {
	ctx, _, sopRepo, _, shiftRepo, taskRepo, machineRepo, dispatcher, _ := setupTestEnv()

	nodeID := "node_1"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}
	sopRepo.steps["step_q"] = &models.SOPStep{
		ID: "step_q", Duration: 300, EquipmentTypeID: &equipTypeID,
	}

	// Pre-existing QUEUED task (tạo trực tiếp, không qua SchedulePO)
	taskRepo.tasks["task_q"] = &models.StaffTask{
		ID:        "task_q",
		NodeID:    nodeID,
		SOPStepID: "step_q",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}

	err := dispatcher.Dispatch(ctx, nodeID)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	task := taskRepo.tasks["task_q"]
	if task.Status != models.TaskPending {
		t.Errorf("Expected TaskPending after Dispatch, got %s", task.Status)
	}
	if task.AssignedTo != "minh" {
		t.Errorf("Expected AssignedTo=minh, got %s", task.AssignedTo)
	}
	if task.MachineID != "fryer_01" {
		t.Errorf("Expected MachineID=fryer_01, got %s", task.MachineID)
	}
}

// ─── T26: Dispatcher — Không Có Staff → Task Giữ QUEUED ─────────────────────

func TestSchedulingEngine_T26_Dispatch_NoStaff(t *testing.T) {
	ctx, _, sopRepo, _, _, taskRepo, machineRepo, dispatcher, _ := setupTestEnv()

	nodeID := "node_1"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}
	sopRepo.steps["step_q"] = &models.SOPStep{
		ID: "step_q", Duration: 300, EquipmentTypeID: &equipTypeID,
	}

	taskRepo.tasks["task_q"] = &models.StaffTask{
		ID:        "task_q",
		NodeID:    nodeID,
		SOPStepID: "step_q",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		CreatedAt: time.Now(),
	}

	// Không có active shift nào
	err := dispatcher.Dispatch(ctx, nodeID)
	// Non-fatal: Dispatch không return error dù không có staff
	if err != nil {
		t.Fatalf("Dispatch should not error when no staff: %v", err)
	}

	task := taskRepo.tasks["task_q"]
	if task.Status != models.TaskQueued {
		t.Errorf("Expected task to remain QUEUED when no staff available, got %s", task.Status)
	}
	if task.AssignedTo != "" {
		t.Errorf("Expected task to remain unassigned, got AssignedTo=%s", task.AssignedTo)
	}
}

// ─── T27: Dispatcher — FIFO (Task Cũ Hơn Được Assign Trước) ─────────────────

func TestSchedulingEngine_T27_Dispatch_FIFO(t *testing.T) {
	ctx, _, sopRepo, _, shiftRepo, taskRepo, machineRepo, dispatcher, _ := setupTestEnv()

	nodeID := "node_1"
	equipTypeID := "equip_fryer"

	machineRepo.machines["fryer_01"] = &models.Machine{
		ID: "fryer_01", NodeID: nodeID, EquipmentTypeID: equipTypeID, Status: models.MachineIdle,
	}
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "minh", Status: models.ShiftActive,
	}
	sopRepo.steps["step_short"] = &models.SOPStep{
		ID: "step_short", Duration: 300, EquipmentTypeID: &equipTypeID,
	}

	now := time.Now()

	// Task A: CreatedAt cũ hơn (T0 - 60s)
	taskRepo.tasks["task_a"] = &models.StaffTask{
		ID:        "task_a",
		NodeID:    nodeID,
		SOPStepID: "step_short",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		Priority:  1,
		CreatedAt: now.Add(-60 * time.Second),
	}
	// Task B: CreatedAt mới hơn (T0 - 10s)
	taskRepo.tasks["task_b"] = &models.StaffTask{
		ID:        "task_b",
		NodeID:    nodeID,
		SOPStepID: "step_short",
		TaskKind:  models.TaskKindNormal,
		Status:    models.TaskQueued,
		Priority:  1,
		CreatedAt: now.Add(-10 * time.Second),
	}

	err := dispatcher.Dispatch(ctx, nodeID)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	taskA := taskRepo.tasks["task_a"]
	taskB := taskRepo.tasks["task_b"]

	// Cả 2 đều được assign (Dispatcher không giới hạn) nhưng A phải được schedule trước B
	if taskA.Status != models.TaskPending {
		t.Errorf("Task A (older) should be PENDING, got %s", taskA.Status)
	}
	if taskB.Status != models.TaskPending {
		t.Errorf("Task B should also be PENDING, got %s", taskB.Status)
	}
	// FIFO: A.ScheduledStart <= B.ScheduledStart
	if taskA.ScheduledStart.After(taskB.ScheduledStart) {
		t.Errorf("FIFO violated: A.ScheduledStart=%v > B.ScheduledStart=%v", taskA.ScheduledStart, taskB.ScheduledStart)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// containsError checks whether err (or its chain) matches target via errors.Is.
func containsError(err, target error) bool {
	return errors.Is(err, target)
}
