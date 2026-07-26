package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	// ErrPONotFound — PO không tồn tại.
	ErrPONotFound = errors.New("production order not found")

	// ErrPONotInProgress — SchedulePO chỉ chạy khi PO đang IN_PROGRESS.
	ErrPONotInProgress = errors.New("production order is not IN_PROGRESS")

	// ErrSOPHasNoSteps — SOP không có bước nào để schedule.
	ErrSOPHasNoSteps = errors.New("SOP has no steps")

	// ErrCyclicDependency — DependsOn tạo thành vòng lặp.
	ErrCyclicDependency = errors.New("cyclic dependency detected in SOP steps")

	// ErrInvalidDependency — DependsOn trỏ đến stepID không tồn tại trong SOP.
	ErrInvalidDependency = errors.New("step depends on non-existent step ID")
)

// ─── Interface ────────────────────────────────────────────────────────────────

// SchedulingEngine lo việc planning: build DAG từ SOP + tạo StaffTask ở trạng
// thái QUEUED. Không assign staff hay machine — đó là việc của Dispatcher.
//
// Được gọi khi:
//   - PO chuyển sang IN_PROGRESS (từ order_orchestrator)
//   - Staff mới bắt đầu ca (trigger Dispatcher, không phải SchedulePO)
type SchedulingEngine interface {
	// SchedulePO: entry point chính.
	// Build DAG, tạo QUEUED tasks, rồi gọi Dispatcher.Dispatch() sync.
	// Idempotent: bỏ qua steps đã có task PENDING/ACTIVE/WAITING/DONE.
	SchedulePO(ctx context.Context, poID string) ([]*models.StaffTask, error)

	// RescheduleOnShiftChange: gọi khi shift thay đổi.
	// Trigger Dispatcher để assign các QUEUED tasks còn trống.
	RescheduleOnShiftChange(ctx context.Context, nodeID string) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type schedulingEngine struct {
	poRepo     services.ProductionOrderRepository
	sopRepo    services.SOPRepository
	taskRepo   services.StaffTaskRepository
	machineRepo services.MachineRepository
	dispatcher Dispatcher
	now        func() time.Time // injectable cho testing
}

// NewSchedulingEngine tạo một SchedulingEngine mới.
// Dispatcher được inject để được gọi sync sau khi QUEUED tasks được tạo.
func NewSchedulingEngine(
	poRepo services.ProductionOrderRepository,
	sopRepo services.SOPRepository,
	taskRepo services.StaffTaskRepository,
	machineRepo services.MachineRepository,
	dispatcher Dispatcher,
) SchedulingEngine {
	return &schedulingEngine{
		poRepo:      poRepo,
		sopRepo:     sopRepo,
		taskRepo:    taskRepo,
		machineRepo: machineRepo,
		dispatcher:  dispatcher,
		now:         time.Now,
	}
}

// ─── SchedulePO ──────────────────────────────────────────────────────────────

func (e *schedulingEngine) SchedulePO(ctx context.Context, poID string) ([]*models.StaffTask, error) {
	// [1] Load PO
	po, err := e.poRepo.FindByID(ctx, poID)
	if err != nil {
		return nil, fmt.Errorf("schedulingEngine.SchedulePO: load PO: %w", err)
	}
	if po == nil {
		return nil, ErrPONotFound
	}
	if po.Status != models.POInProgress {
		return nil, ErrPONotInProgress
	}

	// [2] Load SOP steps
	steps, err := e.sopRepo.ListSteps(ctx, po.SOPID)
	if err != nil {
		return nil, fmt.Errorf("schedulingEngine.SchedulePO: load steps: %w", err)
	}
	if len(steps) == 0 {
		return nil, ErrSOPHasNoSteps
	}

	// [3] Idempotency: tìm steps cần schedule (chưa có task hoặc tất cả tasks là CANCELLED)
	stepsToSchedule, err := e.filterStepsToSchedule(ctx, poID, steps)
	if err != nil {
		return nil, fmt.Errorf("schedulingEngine.SchedulePO: idempotency check: %w", err)
	}
	if len(stepsToSchedule) == 0 {
		// Tất cả steps đã được schedule — trả về existing tasks
		return e.taskRepo.FindByPO(ctx, poID)
	}

	// [4] Build DAG
	topoGroups, err := buildTopoGroups(stepsToSchedule)
	if err != nil {
		return nil, fmt.Errorf("schedulingEngine.SchedulePO: build DAG: %w", err)
	}

	// [5] Tạo QUEUED tasks theo topo order
	// Estimate timeline: mỗi group bắt đầu sau khi group trước kết thúc (đơn giản).
	// Dispatcher sẽ recalculate schedule chính xác khi assign.
	now := e.now()
	var createdTasks []*models.StaffTask
	groupStart := now

	// Build reverse dependencies to find ancestors of IsIdleStep
	revDeps := make(map[string][]string)
	for _, step := range stepsToSchedule {
		for _, dep := range step.DependsOn {
			revDeps[step.ID] = append(revDeps[step.ID], dep)
		}
	}
	criticalStepIDs := make(map[string]bool)
	var queue []string
	for _, step := range stepsToSchedule {
		if step.IsIdleStep {
			queue = append(queue, step.ID)
		}
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, parent := range revDeps[curr] {
			if !criticalStepIDs[parent] {
				criticalStepIDs[parent] = true
				queue = append(queue, parent)
			}
		}
	}

	for _, group := range topoGroups {
		groupMaxEnd := groupStart
		for _, step := range group {
			stepCopy := step // avoid loop variable capture
			isCritical := criticalStepIDs[step.ID]
			tasks := e.buildQueuedTasks(ctx, po, &stepCopy, groupStart, isCritical)
			for _, t := range tasks {
				if err := e.taskRepo.Create(ctx, t); err != nil {
					return nil, fmt.Errorf("schedulingEngine.SchedulePO: create task: %w", err)
				}
				createdTasks = append(createdTasks, t)
				if t.ScheduledEnd.After(groupMaxEnd) {
					groupMaxEnd = t.ScheduledEnd
				}
			}
		}
		// Group tiếp theo bắt đầu sau khi group này kết thúc (estimate)
		groupStart = groupMaxEnd
	}

	log.Printf("schedulingEngine.SchedulePO: po=%s created %d QUEUED tasks", poID, len(createdTasks))

	// [6] Gọi Dispatcher sync để assign ngay
	if e.dispatcher != nil {
		if err := e.dispatcher.Dispatch(ctx, po.NodeID); err != nil {
			// Non-fatal: tasks đã được tạo, Dispatcher sẽ retry ở lần gọi tiếp theo
			log.Printf("schedulingEngine.SchedulePO: dispatcher.Dispatch warning: %v", err)
		}
	}

	return e.taskRepo.FindByPO(ctx, poID)
}

// ─── RescheduleOnShiftChange ─────────────────────────────────────────────────

func (e *schedulingEngine) RescheduleOnShiftChange(ctx context.Context, nodeID string) error {
	// Khi shift thay đổi, chỉ cần gọi Dispatcher để assign các QUEUED tasks còn trống.
	// SchedulePO không cần chạy lại — tasks đã được tạo khi PO → IN_PROGRESS.
	if e.dispatcher == nil {
		return nil
	}
	if err := e.dispatcher.Dispatch(ctx, nodeID); err != nil {
		return fmt.Errorf("schedulingEngine.RescheduleOnShiftChange: %w", err)
	}
	return nil
}

// ─── buildQueuedTasks ────────────────────────────────────────────────────────

// bestMachineCapacity tìm máy có MaxCapacity lớn nhất cho một EquipmentType tại node.
func (e *schedulingEngine) bestMachineCapacity(ctx context.Context, nodeID string, equipTypeID *string) float64 {
	if equipTypeID == nil {
		return 0
	}
	machines, err := e.machineRepo.FindByNodeID(ctx, nodeID)
	if err != nil {
		return 0
	}
	var maxCap float64
	for _, m := range machines {
		if m.EquipmentTypeID == *equipTypeID && m.MaxCapacity > maxCap {
			maxCap = m.MaxCapacity
		}
	}
	return maxCap
}

// buildQueuedTasks tạo 1 hoặc nhiều QUEUED tasks từ một SOPStep.
// - Normal step: 1 task
// - Idle step: 1 hoặc nhiều cặp SETUP+RETRIEVE (tùy theo capacity).
func (e *schedulingEngine) buildQueuedTasks(ctx context.Context, po *models.ProductionOrder, step *models.SOPStep, depDoneAt time.Time, isCritical bool) []*models.StaffTask {
	now := e.now()
	reqSlots := po.TargetQty * step.SlotConsumption

	if !step.IsIdleStep {
		// Normal task
		t := &models.StaffTask{
			ID:              uuid.New().String(),
			POID:            po.ID,
			SOPStepID:       step.ID,
			NodeID:          po.NodeID,
			TaskKind:        models.TaskKindNormal,
			AssignedTo:      "",
			MachineID:       "",
			TargetQty:       po.TargetQty,
			RequiredSlots:   reqSlots,
			Status:          models.TaskQueued,
			Priority:        step.SeqNo,
			IsInterruptible: false,
			IsCritical:      isCritical,
			EarliestStart:   depDoneAt,
			ScheduledStart:  depDoneAt,
			ScheduledEnd:    depDoneAt.Add(time.Duration(step.Duration) * time.Second),
			CreatedAt:       now,
		}
		return []*models.StaffTask{t}
	}

	// Idle step
	maxCap := e.bestMachineCapacity(ctx, po.NodeID, step.EquipmentTypeID)
	
	// Nếu không có MaxCapacity hoặc không dùng slot, fallback 1 batch
	if maxCap <= 0 || step.SlotConsumption <= 0 || reqSlots <= maxCap {
		return e.buildBatchedIdleTasks(po, step, depDoneAt, isCritical, po.TargetQty, 0, 1)
	}

	// Tính số mẻ cần thiết
	batchQty := maxCap / step.SlotConsumption
	if batchQty <= 0 {
		batchQty = 1 // fallback an toàn
	}
	numBatches := int(po.TargetQty / batchQty)
	if float64(numBatches)*batchQty < po.TargetQty {
		numBatches++
	}

	var allTasks []*models.StaffTask
	currentDepDone := depDoneAt

	for i := 0; i < numBatches; i++ {
		qty := batchQty
		if i == numBatches-1 {
			qty = po.TargetQty - float64(i)*batchQty
		}
		batchTasks := e.buildBatchedIdleTasks(po, step, currentDepDone, isCritical, qty, i, numBatches)
		allTasks = append(allTasks, batchTasks...)
		// Batch tiếp theo có thể bắt đầu setup ngay sau khi batch trước setup xong (chờ nhân viên rảnh)
		// Dispatcher sẽ lo việc check xem máy có rảnh hay không (mFreeAt).
		currentDepDone = batchTasks[0].ScheduledEnd 
	}

	return allTasks
}

// buildBatchedIdleTasks sinh 1 cặp SETUP+RETRIEVE cho 1 mẻ máy.
func (e *schedulingEngine) buildBatchedIdleTasks(
	po *models.ProductionOrder,
	step *models.SOPStep,
	depDoneAt time.Time,
	isCritical bool,
	batchQty float64,
	batchIndex int,
	totalBatches int,
) []*models.StaffTask {
	now := e.now()
	reqSlots := batchQty * step.SlotConsumption
	activeTime := 0
	if step.ActiveTime != nil {
		activeTime = *step.ActiveTime
	}

	setupEnd := depDoneAt.Add(time.Duration(activeTime) * time.Second)
	stepEnd := depDoneAt.Add(time.Duration(step.Duration) * time.Second)

	setupTask := &models.StaffTask{
		ID:             uuid.New().String(),
		POID:           po.ID,
		SOPStepID:      step.ID,
		NodeID:         po.NodeID,
		TaskKind:       models.TaskKindSetup,
		AssignedTo:     "",
		MachineID:      "",
		TargetQty:      batchQty,
		RequiredSlots:  reqSlots,
		Status:         models.TaskQueued,
		Priority:       step.SeqNo,
		IsInterruptible: false,
		BatchIndex:     batchIndex,
		EarliestStart:  depDoneAt,
		ScheduledStart: depDoneAt,
		ScheduledEnd:   setupEnd,
		CreatedAt:      now,
	}

	retrieveTask := &models.StaffTask{
		ID:             uuid.New().String(),
		POID:           po.ID,
		SOPStepID:      step.ID,
		NodeID:         po.NodeID,
		TaskKind:       models.TaskKindRetrieve,
		AssignedTo:     "",
		MachineID:      "",
		TargetQty:      batchQty,
		RequiredSlots:  reqSlots,
		Status:         models.TaskQueued,
		Priority:       step.SeqNo,
		IsInterruptible: false,
		ParentTaskID:   nil, // Dispatcher set khi assign fill-in task
		BatchIndex:     batchIndex,
		EarliestStart:  setupEnd,
		ScheduledStart: setupEnd,
		ScheduledEnd:   stepEnd,
		CreatedAt:      now,
	}

	return []*models.StaffTask{setupTask, retrieveTask}
}

// ─── filterStepsToSchedule ───────────────────────────────────────────────────

// filterStepsToSchedule lọc ra các SOPStep cần được schedule.
// Bỏ qua: steps có task đang QUEUED/PENDING/ACTIVE/WAITING hoặc DONE.
// Giữ lại: steps chưa có task nào, hoặc tất cả tasks đều là CANCELLED.
func (e *schedulingEngine) filterStepsToSchedule(ctx context.Context, poID string, steps []*models.SOPStep) ([]*models.SOPStep, error) {
	existingTasks, err := e.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return nil, err
	}

	// Build map: stepID → set of statuses
	type statusSet map[models.TaskStatus]bool
	stepStatuses := make(map[string]statusSet)
	for _, t := range existingTasks {
		if stepStatuses[t.SOPStepID] == nil {
			stepStatuses[t.SOPStepID] = make(statusSet)
		}
		stepStatuses[t.SOPStepID][t.Status] = true
	}

	liveStatuses := map[models.TaskStatus]bool{
		models.TaskQueued:  true,
		models.TaskPending: true,
		models.TaskActive:  true,
		models.TaskWaiting: true,
		models.TaskDone:    true,
	}

	var result []*models.SOPStep
	for _, step := range steps {
		statuses, hasTask := stepStatuses[step.ID]
		if !hasTask {
			result = append(result, step)
			continue
		}
		// Check: có bất kỳ live status nào không?
		hasLive := false
		for s := range statuses {
			if liveStatuses[s] {
				hasLive = true
				break
			}
		}
		if !hasLive {
			// Tất cả tasks đều CANCELLED → re-schedule
			result = append(result, step)
		}
		// Nếu có live status → bỏ qua (đang chạy bình thường hoặc đã DONE)
	}
	return result, nil
}

// ─── buildTopoGroups (Kahn's Algorithm) ─────────────────────────────────────

// buildTopoGroups nhận danh sách SOPStep và trả về các nhóm có thể chạy song song,
// theo thứ tự topo (group[0] là những step không có dependency, v.v.)
//
// Error cases:
//   - ErrCyclicDependency: A→B→A
//   - ErrInvalidDependency: DependsOn trỏ stepID không có trong danh sách
func buildTopoGroups(steps []*models.SOPStep) ([][]models.SOPStep, error) {
	// Build stepID → step map để validate DependsOn
	stepMap := make(map[string]*models.SOPStep, len(steps))
	for _, s := range steps {
		stepMap[s.ID] = s
	}

	// Build inDegree + adjacency graph
	inDegree := make(map[string]int, len(steps))
	graph := make(map[string][]string, len(steps)) // parentID → []childID

	for _, s := range steps {
		if _, exists := inDegree[s.ID]; !exists {
			inDegree[s.ID] = 0
		}
		for _, depID := range s.DependsOn {
			if _, ok := stepMap[depID]; !ok {
				return nil, fmt.Errorf("%w: step %q depends on %q", ErrInvalidDependency, s.ID, depID)
			}
			inDegree[s.ID]++
			graph[depID] = append(graph[depID], s.ID)
		}
	}

	// Kahn's BFS
	var queue []string
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var result [][]models.SOPStep
	processed := 0

	for len(queue) > 0 {
		// Snapshot current queue → 1 group (có thể chạy song song)
		group := make([]models.SOPStep, 0, len(queue))
		nextQueue := []string{}

		for _, id := range queue {
			group = append(group, *stepMap[id])
			processed++
			for _, childID := range graph[id] {
				inDegree[childID]--
				if inDegree[childID] == 0 {
					nextQueue = append(nextQueue, childID)
				}
			}
		}

		result = append(result, group)
		queue = nextQueue
	}

	if processed != len(steps) {
		return nil, ErrCyclicDependency
	}
	return result, nil
}
