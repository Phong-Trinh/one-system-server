package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ─── Interface ────────────────────────────────────────────────────────────────

// Dispatcher lo việc assignment: kéo QUEUED tasks phù hợp → assign machine + staff → PENDING.
//
// Được gọi synchronous (trong cùng request) sau các sự kiện:
//   - SchedulePO() tạo xong QUEUED tasks (PO → IN_PROGRESS)
//   - CompleteTask() hoàn thành 1 task (resource rảnh)
//   - StartShift() nhân viên mới vào ca
//   - EndShift() nhân viên rời ca → trigger re-assign nếu có staff thay thế
type Dispatcher interface {
	Dispatch(ctx context.Context, nodeID string) error
}

// ─── Implementation ───────────────────────────────────────────────────────────

type dispatcher struct {
	shiftRepo   services.StaffShiftRepository
	machineRepo services.MachineRepository
	batchRepo   services.ProductionBatchRepository
	taskRepo    services.StaffTaskRepository
	sopRepo     services.SOPRepository
	safetyBuf   time.Duration  // default 30s — idle window buffer cho fill-in tasks
	now         func() time.Time
}

// NewDispatcher tạo một Dispatcher mới.
//
//   - batchRepo dùng để tính machine free_at (load batch.EstimatedCompletion khi machine BUSY)
//   - sopRepo dùng để load SOPStep.Duration khi tính fill-in task fit
func NewDispatcher(
	shiftRepo services.StaffShiftRepository,
	machineRepo services.MachineRepository,
	batchRepo services.ProductionBatchRepository,
	taskRepo services.StaffTaskRepository,
	sopRepo services.SOPRepository,
) Dispatcher {
	return &dispatcher{
		shiftRepo:   shiftRepo,
		machineRepo: machineRepo,
		batchRepo:   batchRepo,
		taskRepo:    taskRepo,
		sopRepo:     sopRepo,
		safetyBuf:   30 * time.Second,
		now:         time.Now,
	}
}

// ─── Dispatch ────────────────────────────────────────────────────────────────

// Dispatch lấy tất cả QUEUED tasks tại nodeID và cố gắng assign machine + staff cho mỗi task.
//
// Thuật toán:
//  1. Load QUEUED tasks (FIFO theo CreatedAt — đã sort trong repo)
//  2. Load active shifts + tính staffFreeAt cho mỗi shift
//  3. Load machines tại node
//  4. Với mỗi QUEUED task:
//     a. pickMachine → machine + machineFreeAt
//     b. pickStaff → shift + staffFreeAt (phù hợp equipType)
//     c. Nếu không có staff: giữ nguyên QUEUED, log, continue
//     d. calcSchedule → scheduledStart/End chính xác
//     e. task.Status = PENDING, ghi DB
func (d *dispatcher) Dispatch(ctx context.Context, nodeID string) error {
	// [1] Load active shifts + tinh staffFreeAt
	shifts, err := d.shiftRepo.FindActiveByNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: load shifts: %w", err)
	}
	staffFreeAt, err := d.calcStaffFreeAt(ctx, shifts)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: calc staff free times: %w", err)
	}

	// [2] Load machines tai node
	machines, err := d.machineRepo.FindByNodeID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: load machines: %w", err)
	}
	machineFreeAt, err := d.calcMachineFreeAt(ctx, machines)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: calc machine free times: %w", err)
	}

	now := d.now()

	// [3] Pass 1: Assign SETUP va RETRIEVE tasks truoc.
	// NORMAL tasks (fill-in candidates) duoc bo qua o pass nay.
	pass1Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: load queued tasks (pass 1): %w", err)
	}
	for _, task := range pass1Tasks {
		if task.TaskKind != models.TaskKindSetup && task.TaskKind != models.TaskKindRetrieve {
			continue
		}
		d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineFreeAt, now)
	}

	// [4] Fill-in assignment: RETRIEVE tasks da la PENDING -> idle windows da xac dinh.
	// Candidates (QUEUED) duoc claim vao idle windows neu phu hop.
	if err := d.assignFillInTasks(ctx, nodeID, shifts, staffFreeAt); err != nil {
		log.Printf("dispatcher.Dispatch: assignFillInTasks warning: %v", err)
	}

	// [5] Pass 2: Assign QUEUED NORMAL tasks con lai.
	// Reload de loai bo candidates da assign lam fill-in o buoc tren.
	pass2Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
	if err != nil {
		log.Printf("dispatcher.Dispatch: reload queued tasks (pass 2): %v (continue)", err)
		pass2Tasks = nil
	}
	for _, task := range pass2Tasks {
		if task.TaskKind != models.TaskKindNormal {
			continue
		}
		d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineFreeAt, now)
	}

	return nil
}

// assignSingleTask co gang assign mot QUEUED task cho staff va machine.
// Cap nhat staffFreeAt va machineFreeAt in-place khi assign thanh cong.
func (d *dispatcher) assignSingleTask(
	ctx context.Context,
	nodeID string,
	task *models.StaffTask,
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
	machines []*models.Machine,
	machineFreeAt map[string]time.Time,
	now time.Time,
) bool {
	step, err := d.sopRepo.FindStepByID(ctx, task.SOPStepID)
	if err != nil {
		log.Printf("dispatcher.assignSingleTask: load step %s: %v (skip)", task.SOPStepID, err)
		return false
	}
	if step == nil {
		log.Printf("dispatcher.assignSingleTask: step %s not found (skip task %s)", task.SOPStepID, task.ID)
		return false
	}

	machineID, mFreeAt := d.pickMachine(machines, machineFreeAt, step, now)
	shift, sFreeAt := d.pickStaff(shifts, staffFreeAt, step.EquipmentTypeID, now)
	if shift == nil {
		log.Printf("dispatcher.Dispatch: no available staff for queued task %s (node=%s, equipType=%v)",
			task.ID, nodeID, step.EquipmentTypeID)
		return false
	}

	depDoneAt := task.ScheduledStart
	scheduledStart := maxTime(depDoneAt, mFreeAt, sFreeAt)
	var scheduledEnd time.Time
	if task.TaskKind == models.TaskKindSetup {
		activeTime := 0
		if step.ActiveTime != nil {
			activeTime = *step.ActiveTime
		}
		scheduledEnd = scheduledStart.Add(time.Duration(activeTime) * time.Second)
	} else if task.TaskKind == models.TaskKindRetrieve {
		retrieveDuration := step.Duration
		if step.ActiveTime != nil {
			retrieveDuration -= *step.ActiveTime
		}
		scheduledEnd = scheduledStart.Add(time.Duration(retrieveDuration) * time.Second)
	} else {
		scheduledEnd = scheduledStart.Add(time.Duration(step.Duration) * time.Second)
	}

	task.AssignedTo = shift.StaffID
	task.MachineID = machineID
	task.Status = models.TaskPending
	task.ScheduledStart = scheduledStart
	task.ScheduledEnd = scheduledEnd

	if err := d.taskRepo.Update(ctx, task); err != nil {
		log.Printf("dispatcher.assignSingleTask: update task %s: %v (skip)", task.ID, err)
		return false
	}

	staffFreeAt[shift.StaffID] = scheduledEnd
	if machineID != "" {
		if machineFreeAt[machineID].Before(scheduledEnd) {
			machineFreeAt[machineID] = scheduledEnd
		}
	}

	log.Printf("dispatcher.Dispatch: assigned task %s -> staff=%s machine=%s start=%s",
		task.ID, shift.StaffID, machineID, scheduledStart.Format(time.RFC3339))
	return true
}
// ─── pickMachine ─────────────────────────────────────────────────────────────

// pickMachine tìm machine phù hợp cho step và trả về machineID + thời điểm nó rảnh.
//
//   - EquipmentTypeID == nil → manual step, không cần machine
//   - Ưu tiên machine IDLE (freeAt = now)
//   - Nếu không có IDLE: dùng machine BUSY có freeAt sớm nhất
//   - Nếu không có machine nào cùng type: machineID = "", freeAt = now (non-fatal)
func (d *dispatcher) pickMachine(
	machines []*models.Machine,
	machineFreeAt map[string]time.Time,
	step *models.SOPStep,
	now time.Time,
) (machineID string, freeAt time.Time) {
	if step.EquipmentTypeID == nil {
		return "", now // manual step
	}
	equipType := *step.EquipmentTypeID

	var bestID string
	bestFreeAt := time.Time{} // zero = chưa có candidate

	for _, m := range machines {
		if m.EquipmentTypeID != equipType {
			continue
		}
		if m.Status == models.MachineDecommissioned || m.Status == models.MachineUnderMaintenance {
			continue
		}
		mFree, ok := machineFreeAt[m.ID]
		if !ok {
			mFree = now
		}
		if bestFreeAt.IsZero() || mFree.Before(bestFreeAt) {
			bestID = m.ID
			bestFreeAt = mFree
		}
	}

	if bestID == "" {
		log.Printf("dispatcher.pickMachine: no machine available for equipType=%s (non-fatal)", equipType)
		return "", now
	}
	return bestID, bestFreeAt
}

// ─── pickStaff ───────────────────────────────────────────────────────────────

// pickStaff tìm staff phù hợp với equipTypeID và có thể rảnh sớm nhất (FIFO).
//
//   - equipTypeID == nil → bất kỳ staff flexible nào (StationID == nil)
//   - Chỉ xét shifts đang ACTIVE
//   - Ưu tiên staff có freeAt sớm nhất
func (d *dispatcher) pickStaff(
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
	equipTypeID *string,
	now time.Time,
) (shift *models.StaffShift, freeAt time.Time) {
	var bestShift *models.StaffShift
	bestFreeAt := time.Time{}

	for _, s := range shifts {
		if s.Status != models.ShiftActive {
			continue
		}
		// Station match:
		//   - equipTypeID nil → chỉ nhận flexible staff (StationID nil)
		//   - equipTypeID non-nil → nhận station match hoặc flexible
		if equipTypeID == nil {
			if s.StationID != nil {
				continue // flexible task chỉ cho flexible staff
			}
		} else {
			if s.StationID != nil && *s.StationID != *equipTypeID {
				continue // station không khớp
			}
			// s.StationID == nil → flexible, nhận mọi equipType
		}

		sFree, ok := staffFreeAt[s.StaffID]
		if !ok {
			sFree = now
		}
		if bestFreeAt.IsZero() || sFree.Before(bestFreeAt) {
			bestShift = s
			bestFreeAt = sFree
		}
	}

	if bestShift == nil {
		return nil, time.Time{}
	}
	return bestShift, bestFreeAt
}

// ─── calcStaffFreeAt ─────────────────────────────────────────────────────────

// calcStaffFreeAt tính thời điểm mỗi staff rảnh dựa trên tasks đang live.
func (d *dispatcher) calcStaffFreeAt(ctx context.Context, shifts []*models.StaffShift) (map[string]time.Time, error) {
	now := d.now()
	result := make(map[string]time.Time, len(shifts))
	for _, s := range shifts {
		tasks, err := d.taskRepo.FindByStaff(ctx, s.StaffID,
			[]models.TaskStatus{models.TaskQueued, models.TaskPending, models.TaskActive, models.TaskWaiting})
		if err != nil {
			return nil, fmt.Errorf("calcStaffFreeAt for staff %s: %w", s.StaffID, err)
		}
		freeAt := now
		for _, t := range tasks {
			if t.ScheduledEnd.After(freeAt) {
				freeAt = t.ScheduledEnd
			}
		}
		result[s.StaffID] = freeAt
	}
	return result, nil
}

// ─── calcMachineFreeAt ───────────────────────────────────────────────────────

// calcMachineFreeAt tính thời điểm mỗi machine rảnh.
// Với machine IDLE: freeAt = now.
// Với machine BUSY: freeAt = batch.EstimatedCompletion (load từ CurrentBatchID).
func (d *dispatcher) calcMachineFreeAt(ctx context.Context, machines []*models.Machine) (map[string]time.Time, error) {
	now := d.now()
	result := make(map[string]time.Time, len(machines))
	for _, m := range machines {
		if m.Status != models.MachineBusy || m.CurrentBatchID == nil {
			result[m.ID] = now
			continue
		}
		// Load batch để lấy EstimatedCompletion
		batch, err := d.batchRepo.FindByID(ctx, *m.CurrentBatchID)
		if err != nil {
			log.Printf("calcMachineFreeAt: load batch %s: %v (use now)", *m.CurrentBatchID, err)
			result[m.ID] = now
			continue
		}
		if batch == nil || batch.EstimatedCompletion == nil {
			result[m.ID] = now
			continue
		}
		result[m.ID] = *batch.EstimatedCompletion
	}
	return result, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func maxTime(times ...time.Time) time.Time {
	var max time.Time
	for _, t := range times {
		if t.After(max) {
			max = t
		}
	}
	return max
}

// ─── assignFillInTasks ───────────────────────────────────────────────────────

// assignFillInTasks tìm RETRIEVE tasks (PENDING hoặc WAITING) tại node,
// tính idle window của chúng, và assign QUEUED candidate phù hợp làm fill-in.
//
// Fill-in task được assign cho cùng staff với RETRIEVE task.
// Candidate phải là QUEUED + TaskKindNormal + ParentTaskID == nil (chưa là fill-in).
func (d *dispatcher) assignFillInTasks(
	ctx context.Context,
	nodeID string,
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
) error {
	// Load RETRIEVE tasks (PENDING hoặc WAITING) tại node
	retrieveTasks, err := d.taskRepo.FindByNode(ctx, nodeID,
		[]models.TaskStatus{models.TaskPending, models.TaskWaiting})
	if err != nil {
		return fmt.Errorf("assignFillInTasks: load retrieve tasks: %w", err)
	}

	// Filter chỉ lấy RETRIEVE kind
	var retrieveList []*models.StaffTask
	for _, t := range retrieveTasks {
		if t.TaskKind == models.TaskKindRetrieve {
			retrieveList = append(retrieveList, t)
		}
	}
	if len(retrieveList) == 0 {
		return nil
	}

	// Build set: retrieveTaskID → đã có fill-in chưa
	// Load tất cả tasks tại node để check FILL_IN existing
	allTasks, err := d.taskRepo.FindByNode(ctx, nodeID, nil)
	if err != nil {
		return fmt.Errorf("assignFillInTasks: load all tasks: %w", err)
	}
	hasFillin := make(map[string]bool)
	for _, t := range allTasks {
		if t.TaskKind == models.TaskKindFillIn && t.ParentTaskID != nil &&
			t.Status != models.TaskCancelled {
			hasFillin[*t.ParentTaskID] = true
		}
	}

	// Candidates: QUEUED tasks, hoặc PENDING tasks (đã được normal loop assign vào tương lai)
	var fillInPool []*models.StaffTask
	for _, c := range allTasks {
		if c.ParentTaskID == nil && c.TaskKind == models.TaskKindNormal && c.Status != models.TaskCancelled {
			if c.Status == models.TaskQueued || c.Status == models.TaskPending {
				fillInPool = append(fillInPool, c)
			}
		}
	}


	// Batch load SOPSteps cho candidates (D4: cần duration để check fit)
	candidateSteps := make(map[string]*models.SOPStep, len(fillInPool))
	for _, c := range fillInPool {
		if _, loaded := candidateSteps[c.SOPStepID]; loaded {
			continue
		}
		step, err := d.sopRepo.FindStepByID(ctx, c.SOPStepID)
		if err == nil && step != nil {
			candidateSteps[c.SOPStepID] = step
		}
	}

	used := make(map[string]bool) // taskID đã được assign làm fill-in trong vòng này

	for _, retrieveTask := range retrieveList {
		if hasFillin[retrieveTask.ID] || retrieveTask.AssignedTo == "" {
			continue
		}

		// Load SOPStep của RETRIEVE task
		parentStep, err := d.sopRepo.FindStepByID(ctx, retrieveTask.SOPStepID)
		if err != nil || parentStep == nil || !parentStep.IsIdleStep {
			continue
		}
		if parentStep.AttentionLevel == models.AttentionActiveWait {
			continue // Không bao giờ fill-in khi ACTIVE_WAIT
		}

		// Tính idle window
		idleStart := retrieveTask.ScheduledStart // = setupTask.ScheduledEnd
		requiresAttentionAt := 0
		if parentStep.RequiresAttentionAt != nil {
			requiresAttentionAt = *parentStep.RequiresAttentionAt
		}
		idleEnd := retrieveTask.ScheduledEnd.Add(-time.Duration(requiresAttentionAt) * time.Second)
		availableWindow := idleEnd.Sub(idleStart) - d.safetyBuf
		if availableWindow <= 0 {
			continue // Window quá hẹp
		}

		// Tìm candidate phù hợp
		var validPool []*models.StaffTask
		for _, c := range fillInPool {
			if c.Status == models.TaskPending {
				// Kéo task vào khoảng trống (idleStart)
				// Đảm bảo không kéo task về trước thời điểm EarliestStart (khi các dependencies chưa hoàn thành)
				if idleStart.Before(c.EarliestStart) {
					continue
				}
				if idleStart.After(c.ScheduledStart) {
					continue // Không đẩy task vào tương lai
				}
			}
			validPool = append(validPool, c)
		}

		candidate, candidateStep := d.findFillInCandidate(
			validPool, candidateSteps, parentStep, availableWindow, retrieveTask.AssignedTo, used)
		if candidate == nil {
			continue
		}

		// Assign fill-in
		retrieveID := retrieveTask.ID
		candidate.AssignedTo = retrieveTask.AssignedTo
		candidate.ParentTaskID = &retrieveID
		candidate.IsInterruptible = true
		candidate.TaskKind = models.TaskKindFillIn
		candidate.Status = models.TaskPending
		candidate.ScheduledStart = idleStart
		candidate.ScheduledEnd = idleStart.Add(time.Duration(candidateStep.Duration) * time.Second)

		if err := d.taskRepo.Update(ctx, candidate); err != nil {
			log.Printf("dispatcher.assignFillInTasks: update fill-in task %s: %v", candidate.ID, err)
			continue
		}

		used[candidate.ID] = true
		hasFillin[retrieveTask.ID] = true
		log.Printf("dispatcher.assignFillInTasks: fill-in task %s → staff=%s for retrieve=%s (window=%s)",
			candidate.ID, retrieveTask.AssignedTo, retrieveTask.ID, availableWindow)
	}
	return nil
}

// ─── findFillInCandidate ──────────────────────────────────────────────────────

// findFillInCandidate chọn candidate phù hợp từ pool dựa trên AttentionLevel.
//
//	FULL_IDLE:       bất kỳ NORMAL task nào fit trong availableWindow
//	NEARBY_IDLE:     task cùng equipment_type_id với parent step (MVP: same as FULL_IDLE)
//	PERIODIC_CHECK:  task có duration < checkInterval - safetyBuf, phải IsInterruptible
//	ACTIVE_WAIT:     không fill-in (đã filter ở trên)
func (d *dispatcher) findFillInCandidate(
	pool []*models.StaffTask,
	poolSteps map[string]*models.SOPStep,
	parentStep *models.SOPStep,
	availableWindow time.Duration,
	staffID string,
	used map[string]bool,
) (candidate *models.StaffTask, step *models.SOPStep) {
	for _, c := range pool {
		if used[c.ID] {
			continue
		}
		// Task phải chưa được assign hoặc assign cho cùng staff
		if c.AssignedTo != "" && c.AssignedTo != staffID {
			continue
		}
		cStep, ok := poolSteps[c.SOPStepID]
		if !ok {
			continue
		}
		cDuration := time.Duration(cStep.Duration) * time.Second

		switch parentStep.AttentionLevel {
		case models.AttentionFullIdle, models.AttentionNearbyIdle:
			// NEARBY_IDLE: MVP treat như FULL_IDLE (skip max_distance check)
			if cDuration <= availableWindow {
				return c, cStep
			}

		case models.AttentionPeriodicCheck:
			if parentStep.CheckIntervalSec == nil {
				continue
			}
			windowPerInterval := time.Duration(*parentStep.CheckIntervalSec)*time.Second - d.safetyBuf
			if windowPerInterval <= 0 {
				continue
			}
			// Candidate phải fit vào 1 check interval VÀ phải là interruptible
			if cDuration <= windowPerInterval && cStep.IsIdleStep == false {
				// Interruptible: chỉ nhận nếu bản thân step không phải idle
				// (idle steps không interruptible theo design)
				return c, cStep
			}
		}
	}
	return nil, nil
}
