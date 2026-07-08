package usecase_test

import (
	"context"
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
