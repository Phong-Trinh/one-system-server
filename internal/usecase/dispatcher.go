package usecase

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

const (
	MaxChunkTime         = 60 * time.Minute
	defaultMinUsefulTime = 15 * time.Minute
	// maxFillInPerWindow gioi han so fill-in duoc gap vao 1 idle window.
	// Tranh vong lap vo han khi nhieu task nho phu hop lien tiep nhau.
	maxFillInPerWindow = 5
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
	// SetNow overrides the current time function (useful for testing)
	SetNow(f func() time.Time)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type dispatcher struct {
	shiftRepo   services.StaffShiftRepository
	machineRepo services.MachineRepository
	batchRepo   services.ProductionBatchRepository
	taskRepo    services.StaffTaskRepository
	sopRepo     services.SOPRepository
	poRepo      services.ProductionOrderRepository
	safetyBuf   time.Duration // default 30s — idle window buffer cho fill-in tasks
	now         func() time.Time
}

// NewDispatcher tạo một Dispatcher mới.
//
//   - batchRepo dùng để tính machine free_at (load batch.EstimatedCompletion khi machine BUSY)
//   - sopRepo dùng để load SOPStep.Duration khi tính fill-in task fit
//   - poRepo dùng để lấy MachineUtilizationScore làm tie-breaker khi sort tasks
func NewDispatcher(
	shiftRepo services.StaffShiftRepository,
	machineRepo services.MachineRepository,
	batchRepo services.ProductionBatchRepository,
	taskRepo services.StaffTaskRepository,
	sopRepo services.SOPRepository,
	poRepo services.ProductionOrderRepository,
) Dispatcher {
	return &dispatcher{
		shiftRepo:   shiftRepo,
		machineRepo: machineRepo,
		batchRepo:   batchRepo,
		taskRepo:    taskRepo,
		sopRepo:     sopRepo,
		poRepo:      poRepo,
		safetyBuf:   30 * time.Second,
		now:         time.Now,
	}
}

// SetNow overrides the current time function.
func (d *dispatcher) SetNow(f func() time.Time) {
	d.now = f
}

// poMachineScore tra ve MachineUtilizationScore cua PO chua task nay.
// Cache ket qua trong scoreCache (map[poID]float64) de tranh query N+1.
func (d *dispatcher) poMachineScore(ctx context.Context, scoreCache map[string]float64, poID string) float64 {
	if s, ok := scoreCache[poID]; ok {
		return s
	}
	po, err := d.poRepo.FindByID(ctx, poID)
	if err != nil || po == nil {
		scoreCache[poID] = 0
		return 0
	}
	scoreCache[poID] = po.MachineUtilizationScore
	return po.MachineUtilizationScore
}

// poDeadlineUrgency tra ve so giay con lai den deadline cua PO.
// PO khong co deadline tra ve MaxInt64 (uu tien thap nhat).
// Dung lam Tie-break 4 trong Dispatcher: PO nao co deadline GAP hon (con it thoi gian hon) se duoc uu tien truoc.
func (d *dispatcher) poDeadlineUrgency(ctx context.Context, deadlineCache map[string]int64, poID string, now time.Time) int64 {
	if v, ok := deadlineCache[poID]; ok {
		return v
	}
	po, err := d.poRepo.FindByID(ctx, poID)
	if err != nil || po == nil || po.DeadlineAt == nil {
		deadlineCache[poID] = math.MaxInt64
		return math.MaxInt64
	}
	urgency := int64(po.DeadlineAt.Sub(now).Seconds())
	deadlineCache[poID] = urgency
	return urgency
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
	now := d.now()

	// [2b] Re-queue pending tasks assigned to offline/broken machines that haven't started yet
	offlineMachines := make(map[string]bool)
	for _, m := range machines {
		if m.Status == models.MachineDecommissioned || m.Status == models.MachineUnderMaintenance || m.Status == "OFFLINE" {
			offlineMachines[m.ID] = true
		}
	}
	if len(offlineMachines) > 0 {
		pendingTasks, err := d.taskRepo.FindByNode(ctx, nodeID, []models.TaskStatus{models.TaskPending})
		if err == nil {
			for _, t := range pendingTasks {
				if offlineMachines[t.MachineID] && t.StartedAt == nil {
					t.Status = models.TaskQueued
					t.MachineID = ""
					t.AssignedTo = ""
					_ = d.taskRepo.Update(ctx, t)
				}
			}
		}
	}

	machineTimelines, err := d.calcMachineTimelines(ctx, nodeID, machines)
	if err != nil {
		return fmt.Errorf("dispatcher.Dispatch: calc machine timelines: %w", err)
	}

	for {
		assignedAny := false
		pass1Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
		if err != nil || len(pass1Tasks) == 0 {
			break
		}

		scoreCache := make(map[string]float64)
		deadlineCache := make(map[string]int64)
		sort.Slice(pass1Tasks, func(i, j int) bool {
			ti, tj := pass1Tasks[i], pass1Tasks[j]
			// Tie-break 1: EarliestStart som hon truoc
			if !ti.EarliestStart.Equal(tj.EarliestStart) {
				return ti.EarliestStart.Before(tj.EarliestStart)
			}
			// Tie-break 2: RETRIEVE truoc SETUP de giai phong may som nhat
			if ti.TaskKind == models.TaskKindRetrieve && tj.TaskKind != models.TaskKindRetrieve {
				return true
			}
			if tj.TaskKind == models.TaskKindRetrieve && ti.TaskKind != models.TaskKindRetrieve {
				return false
			}
			// Tie-break 3: PO xai may nhieu hon (MachineUtilizationScore cao hon) -> uu tien kich hoat truoc
			si := d.poMachineScore(ctx, scoreCache, ti.POID)
			sj := d.poMachineScore(ctx, scoreCache, tj.POID)
			if si != sj {
				return si > sj
			}
			// Tie-break 4: PO co deadline GAP hon (so giay den deadline it hon) -> uu tien truoc
			di := d.poDeadlineUrgency(ctx, deadlineCache, ti.POID, now)
			dj := d.poDeadlineUrgency(ctx, deadlineCache, tj.POID, now)
			return di < dj
		})

		for _, task := range pass1Tasks {
			if task.Status != models.TaskQueued {
				continue
			}
			if task.TaskKind != models.TaskKindSetup && task.TaskKind != models.TaskKindRetrieve && !task.IsCritical {
				continue
			}
			if !d.dependenciesAssigned(ctx, task) {
				continue
			}
			if d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineTimelines, now) {
				assignedAny = true
				break // break inner loop to re-fetch and re-sort
			}
		}
		if !assignedAny {
			break
		}
	}

	// [4] Fill-in assignment: RETRIEVE tasks da la PENDING -> idle windows da xac dinh.
	// Candidates (QUEUED) duoc claim vao idle windows neu phu hop.
	if err := d.assignFillInTasks(ctx, nodeID, shifts, staffFreeAt, machines, machineTimelines); err != nil {
		log.Printf("dispatcher.Dispatch: assignFillInTasks warning: %v", err)
	}

	for {
		assignedAny := false
		pass2Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
		if err != nil || len(pass2Tasks) == 0 {
			break
		}

		scoreCache2 := make(map[string]float64)
		deadlineCache2 := make(map[string]int64)
		sort.Slice(pass2Tasks, func(i, j int) bool {
			ti, tj := pass2Tasks[i], pass2Tasks[j]
			// Tie-break 1: EarliestStart som hon truoc
			if !ti.EarliestStart.Equal(tj.EarliestStart) {
				return ti.EarliestStart.Before(tj.EarliestStart)
			}
			// Tie-break 2: PO xai may nhieu hon -> uu tien hon (giai phong may som)
			si := d.poMachineScore(ctx, scoreCache2, ti.POID)
			sj := d.poMachineScore(ctx, scoreCache2, tj.POID)
			if si != sj {
				return si > sj
			}
			// Tie-break 3: PO co deadline GAP hon -> uu tien truoc
			di := d.poDeadlineUrgency(ctx, deadlineCache2, ti.POID, now)
			dj := d.poDeadlineUrgency(ctx, deadlineCache2, tj.POID, now)
			return di < dj
		})

		for _, task := range pass2Tasks {
			if task.Status != models.TaskQueued {
				continue
			}
			if task.TaskKind != models.TaskKindNormal {
				continue
			}
			if !d.dependenciesAssigned(ctx, task) {
				continue
			}
			if d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineTimelines, now) {
				assignedAny = true
				cascadeTime := task.ScheduledEnd
				if task.TaskKind == models.TaskKindSetup {
					rt := d.findRetrieveForSetup(ctx, task.POID, task.SOPStepID, task.BatchIndex)
					if rt != nil && !rt.ScheduledEnd.IsZero() {
						cascadeTime = rt.ScheduledEnd
					}
				}
				d.cascadeEarliestStart(ctx, task.POID, task.SOPStepID, cascadeTime, pass2Tasks)
				break // break inner loop to re-fetch and re-sort
			}
		}
		if !assignedAny {
			break
		}
	}

	return nil
}

// dependenciesAssigned checks if all dependencies of a task have been assigned (Status != TaskQueued).
func (d *dispatcher) dependenciesAssigned(ctx context.Context, task *models.StaffTask) bool {
	step, err := d.sopRepo.FindStepByID(ctx, task.SOPStepID)
	if err != nil || step == nil {
		// Neu task.SOPStepID co _ (e.g. f3_bun_mix_1), fallback lay base ID
		baseID := task.SOPStepID
		if idx := strings.LastIndex(baseID, "_"); idx > 0 {
			baseID = baseID[:idx]
		}
		step, err = d.sopRepo.FindStepByID(ctx, baseID)
		if err != nil || step == nil {
			return true
		}
	}
	if len(step.DependsOn) == 0 {
		return true
	}
	allTasks, err := d.taskRepo.FindByPO(ctx, task.POID)
	if err != nil {
		return true
	}
	for _, depStepID := range step.DependsOn {
		for _, t := range allTasks {
			if (t.SOPStepID == depStepID || strings.HasPrefix(t.SOPStepID, depStepID+"_")) && t.Status == models.TaskQueued {
				return false
			}
		}
	}
	return true
}

// assignSingleTask co gang assign mot QUEUED task cho staff va machine.
// Cap nhat staffFreeAt va machineTimelines in-place khi assign thanh cong.
func (d *dispatcher) assignSingleTask(
	ctx context.Context,
	nodeID string,
	task *models.StaffTask,
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
	machines []*models.Machine,
	machineTimelines map[string]*MachineTimeline,
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

	if task.TaskKind == models.TaskKindNormal && step.IsSplittable {
		taskDuration := time.Duration(step.Duration) * time.Second
		if task.EstimatedDuration != nil {
			taskDuration = time.Duration(*task.EstimatedDuration) * time.Second
		}
		if taskDuration > MaxChunkTime {
			remDuration := taskDuration - MaxChunkTime
			minUsefulTime := defaultMinUsefulTime
			if step.MinUsefulTime != nil {
				minUsefulTime = time.Duration(*step.MinUsefulTime) * time.Second
			}
			if remDuration >= minUsefulTime {
				factor := float64(MaxChunkTime) / float64(taskDuration)
				if factor > 1.0 {
					factor = 1.0
				}
				chunkQty := task.TargetQty * factor
				chunkSlots := task.RequiredSlots * factor
				remQty := task.TargetQty - chunkQty
				remSlots := task.RequiredSlots - chunkSlots

				if task.RootTaskID == nil {
					task.RootTaskID = &task.ID
				}

				remDurationSec := int(remDuration.Seconds())
				remainder := &models.StaffTask{
					ID:                uuid.New().String(),
					POID:              task.POID,
					OrderItemID:       task.OrderItemID,
					SOPStepID:         task.SOPStepID,
					NodeID:            task.NodeID,
					BatchIndex:        task.BatchIndex,
					TaskKind:          task.TaskKind,
					OriginalKind:      task.OriginalKind,
					Status:            models.TaskQueued,
					TargetQty:         remQty,
					RequiredSlots:     remSlots,
					RootTaskID:        task.RootTaskID,
					EstimatedDuration: &remDurationSec,
					Priority:          task.Priority,
					EarliestStart:     task.EarliestStart,
					IsCritical:        task.IsCritical,
					CreatedAt:         time.Now(),
				}

				if err := d.taskRepo.Create(ctx, remainder); err != nil {
					log.Printf("dispatcher.assignSingleTask: create remainder task %s error: %v", remainder.ID, err)
				} else {
					log.Printf("dispatcher.assignSingleTask: split task %s -> chunk (qty=%.1f) + remainder %s (qty=%.1f, dur=%v)",
						task.ID, chunkQty, remainder.ID, remQty, remDuration)
				}

				task.TargetQty = chunkQty
				task.RequiredSlots = chunkSlots
				chunkDurSec := int(MaxChunkTime.Seconds())
				task.EstimatedDuration = &chunkDurSec
			}
		}
	}

	shift, sFreeAt := d.pickStaff(shifts, staffFreeAt, now)
	if shift == nil {
		log.Printf("dispatcher.Dispatch: no available staff for queued task %s (node=%s, equipType=%v)",
			task.ID, nodeID, step.EquipmentTypeID)
		return false
	}

	var machineID string
	var mFreeAt time.Time

	if task.TaskKind == models.TaskKindRetrieve && step.EquipmentTypeID != nil {
		// RETRIEVE phải dùng cùng máy với SETUP tương ứng (cùng SOPStepID + POID).
		// Không được để pickMachine tự chọn máy khác.
		machineID, mFreeAt = d.findSetupMachineForRetrieve(ctx, task.POID, task.SOPStepID, task.BatchIndex, machineTimelines, now)
		if machineID == "" {
			// SETUP chưa được assign -> không thể assign RETRIEVE lúc này. Bỏ qua để loop vòng sau.
			return false
		}
	} else {
		machineFallback := maxTime(task.EarliestStart, sFreeAt)
		machineID, mFreeAt = d.pickMachine(machines, machineTimelines, step, task.RequiredSlots, machineFallback, task.EstimatedDuration)
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
		// RETRIEVE.ScheduledEnd = thời điểm máy hoàn thành toàn bộ chu kỳ.
		// = SETUP.scheduledStart + step.Duration (không phải RETRIEVE.scheduledStart + retrieveDuration).
		// Tìm SETUP tương ứng để lấy scheduledStart chính xác.
		setupStart := d.findSetupScheduledStart(ctx, task.POID, task.SOPStepID, task.BatchIndex, scheduledStart, step)
		scheduledEnd = setupStart.Add(time.Duration(step.Duration) * time.Second)
	} else {
		taskDuration := time.Duration(step.Duration) * time.Second
		if task.EstimatedDuration != nil {
			taskDuration = time.Duration(*task.EstimatedDuration) * time.Second
		}
		scheduledEnd = scheduledStart.Add(taskDuration)
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
		// Máy thực sự bận đến khi toàn bộ step hoàn thành (scheduledStart + Duration),
		// không phải chỉ đến khi nhân viên rời đi (scheduledEnd của SETUP).
		machineCompleteAt := scheduledStart.Add(time.Duration(step.Duration) * time.Second)
		if task.TaskKind == models.TaskKindSetup {
			// Cập nhật RETRIEVE tương ứng để phản ánh thời gian máy thực tế
			d.syncRetrieveAfterSetup(ctx, task.POID, task.SOPStepID, task.BatchIndex, scheduledEnd, machineCompleteAt)
		}

		// Add allocation to timeline
		endUsageTime := scheduledEnd
		if task.TaskKind == models.TaskKindSetup {
			endUsageTime = machineCompleteAt
		}

		timeline, ok := machineTimelines[machineID]
		if ok {
			timeline.AddAllocation(scheduledStart, endUsageTime, task.RequiredSlots)
		}
	}

	log.Printf("dispatcher.Dispatch: assigned task %s -> staff=%s machine=%s start=%s",
		task.ID, shift.StaffID, machineID, scheduledStart.Format(time.RFC3339))
	return true
}

// findSetupMachineForRetrieve tìm machineID của SETUP task tương ứng (cùng POID + SOPStepID).
// Trả về ("", now) nếu không tìm thấy SETUP task đã assign.
func (d *dispatcher) findSetupMachineForRetrieve(
	ctx context.Context,
	poID string,
	sopStepID string,
	batchIndex int,
	machineTimelines map[string]*MachineTimeline,
	now time.Time,
) (machineID string, freeAt time.Time) {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return "", now
	}
	for _, t := range tasks {
		if t.SOPStepID == sopStepID && t.TaskKind == models.TaskKindSetup && t.BatchIndex == batchIndex && t.MachineID != "" {
			if t.Status == models.TaskFailed || t.Status == models.TaskCancelled {
				continue // Bỏ qua setup đã hỏng/hủy
			}
			// RETRIEVE does not consume additional slots (it takes 0 duration on machine).
			// We just need to know when it can start.
			mFree := now
			if tl, ok := machineTimelines[t.MachineID]; ok {
				// RETRIEVE requires the machine to be complete for this batch cycle.
				// FindEarliestWindow for 0 duration with 0 slots.
				mFree = tl.FindEarliestWindow(now, 0, 0)
			}
			return t.MachineID, mFree
		}
	}
	return "", now
}

// findSetupTaskForRetrieve tìm SETUP task đã assign cho cùng POID + SOPStepID.
// Dùng để lấy SETUP.ScheduledEnd = thời điểm nhân viên rời máy (idleStart thực sự).
func (d *dispatcher) findSetupTaskForRetrieve(
	ctx context.Context,
	poID string,
	sopStepID string,
	batchIndex int,
) *models.StaffTask {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return nil
	}
	for _, t := range tasks {
		if t.SOPStepID == sopStepID && t.TaskKind == models.TaskKindSetup && t.BatchIndex == batchIndex && !t.ScheduledEnd.IsZero() {
			return t
		}
	}
	return nil
}

// findRetrieveForSetup tìm RETRIEVE task tương ứng của SETUP task (cùng POID + SOPStepID + BatchIndex).
func (d *dispatcher) findRetrieveForSetup(
	ctx context.Context,
	poID string,
	sopStepID string,
	batchIndex int,
) *models.StaffTask {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return nil
	}
	for _, t := range tasks {
		if t.SOPStepID == sopStepID && t.TaskKind == models.TaskKindRetrieve && t.BatchIndex == batchIndex {
			return t
		}
	}
	return nil
}

// findSetupScheduledStart trả về ScheduledStart của SETUP task đã assign cho cùng POID + SOPStepID.
// Nếu không tìm thấy (SETUP chưa assign), fallback về retrieveScheduledStart (thời gian nhân viên sẵn sàng).
func (d *dispatcher) findSetupScheduledStart(
	ctx context.Context,
	poID string,
	sopStepID string,
	batchIndex int,
	fallback time.Time,
	step *models.SOPStep,
) time.Time {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return fallback
	}
	for _, t := range tasks {
		if t.SOPStepID == sopStepID && t.TaskKind == models.TaskKindSetup && t.BatchIndex == batchIndex && !t.ScheduledStart.IsZero() {
			return t.ScheduledStart
		}
	}
	// SETUP chưa được assign — fallback: ước tính từ RETRIEVE.scheduledStart - ActiveTime
	activeTime := 0
	if step.ActiveTime != nil {
		activeTime = *step.ActiveTime
	}
	return fallback.Add(-time.Duration(activeTime) * time.Second)
}

// syncRetrieveAfterSetup cập nhật RETRIEVE task (cùng POID + SOPStepID) để phản ánh
// thời gian máy thực tế sau khi SETUP được assign chốt thời gian bắt đầu chính xác.
//
//   - setupEnd:          khi nhân viên rời máy (= SETUP.ScheduledEnd = scheduledStart + ActiveTime)
//   - machineCompleteAt: khi máy thực sự xong (= SETUP.scheduledStart + step.Duration)
//
// Ví dụ: SETUP Nướng bắt đầu 07:37, ActiveTime=3min → setupEnd=07:40, Duration=18min → machineCompleteAt=07:55
// → RETRIEVE.EarliestStart=07:40, RETRIEVE.ScheduledEnd=07:55
func (d *dispatcher) syncRetrieveAfterSetup(
	ctx context.Context,
	poID string,
	sopStepID string,
	batchIndex int,
	setupEnd time.Time,
	machineCompleteAt time.Time,
) {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.SOPStepID != sopStepID || t.TaskKind != models.TaskKindRetrieve || t.BatchIndex != batchIndex {
			continue
		}
		// Cập nhật RETRIEVE để khớp với thời gian máy thực tế
		updated := false
		if t.EarliestStart != setupEnd {
			t.EarliestStart = setupEnd
			updated = true
		}
		if t.ScheduledStart != setupEnd {
			t.ScheduledStart = setupEnd
			updated = true
		}
		if t.ScheduledEnd != machineCompleteAt {
			t.ScheduledEnd = machineCompleteAt
			updated = true
		}
		if updated {
			if err := d.taskRepo.Update(ctx, t); err != nil {
				log.Printf("dispatcher.syncRetrieveAfterSetup: update RETRIEVE %s: %v", t.ID, err)
			} else {
				log.Printf("dispatcher.syncRetrieveAfterSetup: RETRIEVE %s (step=%s) synced → start=%s end=%s",
					t.ID, sopStepID, setupEnd.Format("15:04"), machineCompleteAt.Format("15:04"))
			}
		}
		return // Chỉ có 1 RETRIEVE per SETUP
	}
}

// ─── cascadeEarliestStart ────────────────────────────────────────────────────

// cascadeEarliestStart cập nhật EarliestStart (và ScheduledStart) của tất cả
// QUEUED task trong cùng PO mà phụ thuộc vào parentStepID, nếu newEarliestStart
// muộn hơn thời điểm chúng đang hy vọng bắt đầu.
//
// Hàm chạy đệ quy để cascade qua toàn bộ dây chuyền phụ thuộc.
// Ví dụ: SETUP Trộn delay → RETRIEVE Trộn delay → Chia & Cân delay → Tạo hình delay → Nướng delay.
func (d *dispatcher) cascadeEarliestStart(
	ctx context.Context,
	poID string,
	parentStepID string,
	newEarliestStart time.Time,
	allTasks []*models.StaffTask,
) {
	for _, t := range allTasks {
		if t.POID != poID || t.Status != models.TaskQueued {
			continue
		}
		// Load step để kiểm tra DependsOn
		step, err := d.sopRepo.FindStepByID(ctx, t.SOPStepID)
		if err != nil || step == nil {
			continue
		}
		for _, dep := range step.DependsOn {
			if dep != parentStepID {
				continue
			}
			// Task này phụ thuộc vào parentStepID — kiểm tra xem có cần đẩy lùi không
			if !newEarliestStart.After(t.EarliestStart) {
				break
			}
			delay := newEarliestStart.Sub(t.EarliestStart)
			t.EarliestStart = newEarliestStart
			t.ScheduledStart = newEarliestStart
			t.ScheduledEnd = t.ScheduledEnd.Add(delay)
			if err := d.taskRepo.Update(ctx, t); err != nil {
				log.Printf("dispatcher.cascadeEarliestStart: update task %s: %v", t.ID, err)
			}
			log.Printf("dispatcher.cascadeEarliestStart: pushed task %s (step=%s) EarliestStart → %s",
				t.ID, t.SOPStepID, newEarliestStart.Format("15:04"))
			// Tiếp tục cascade cho các task phụ thuộc vào t
			d.cascadeEarliestStart(ctx, poID, t.SOPStepID, t.ScheduledEnd, allTasks)
			break
		}
	}
}

// ─── pickMachine ─────────────────────────────────────────────────────────────

// pickMachine tìm machine phù hợp cho step và trả về machineID + thời điểm nó rảnh.
//
//   - EquipmentTypeID == nil → manual step, không cần machine
//   - Duyệt qua MachineTimeline để tìm cửa sổ thời gian có đủ dung lượng
//   - Nếu không có machine nào cùng type: machineID = "", freeAt = now (non-fatal)
func (d *dispatcher) pickMachine(
	machines []*models.Machine,
	machineTimelines map[string]*MachineTimeline,
	step *models.SOPStep,
	requiredSlots float64,
	fallback time.Time,
	estimatedDuration *int,
) (machineID string, machineFreeAt time.Time) {
	if step.EquipmentTypeID == nil {
		return "", fallback // manual step
	}
	equipType := *step.EquipmentTypeID

	var bestID string
	bestFreeAt := time.Time{} // zero = chưa có candidate

	duration := time.Duration(step.Duration) * time.Second
	if estimatedDuration != nil {
		duration = time.Duration(*estimatedDuration) * time.Second
	}

	for _, m := range machines {
		if m.EquipmentTypeID != equipType {
			continue
		}
		if m.Status == models.MachineDecommissioned || m.Status == models.MachineUnderMaintenance || m.Status == "OFFLINE" {
			continue
		}

		timeline, ok := machineTimelines[m.ID]
		if !ok {
			continue // Should have been initialized
		}

		reqSlots := requiredSlots
		if reqSlots <= 0 {
			reqSlots = 1.0
		}
		mFree := timeline.FindEarliestWindow(fallback, duration, reqSlots)

		// If mFree is zero, it means it cannot fit at all (e.g., requiredSlots > maxCapacity)
		if mFree.IsZero() {
			continue
		}

		if bestFreeAt.IsZero() || mFree.Before(bestFreeAt) {
			bestID = m.ID
			bestFreeAt = mFree
		}
	}

	if bestID == "" {
		log.Printf("dispatcher.pickMachine: no machine available for equipType=%s (non-fatal)", equipType)
		return "", fallback
	}
	return bestID, bestFreeAt
}

// ─── pickStaff ───────────────────────────────────────────────────────────────

// pickStaff tìm staff có thể rảnh sớm nhất (FIFO).
//
//   - Chỉ xét shifts đang ACTIVE
//   - Ưu tiên staff có freeAt sớm nhất
func (d *dispatcher) pickStaff(
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
	now time.Time,
) (shift *models.StaffShift, freeAt time.Time) {
	var bestShift *models.StaffShift
	var bestFreeAt time.Time

	for _, s := range shifts {
		if s.Status != models.ShiftActive {
			continue
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

// ─── calcMachineTimelines ───────────────────────────────────────────────────────

// calcMachineTimelines khởi tạo MachineTimeline cho tất cả máy tại node.
// Cập nhật các usage từ các batch đang chạy (nếu có).
func (d *dispatcher) calcMachineTimelines(ctx context.Context, nodeID string, machines []*models.Machine) (map[string]*MachineTimeline, error) {
	result := make(map[string]*MachineTimeline, len(machines))

	for _, m := range machines {
		maxCap := m.MaxCapacity
		if maxCap <= 0 {
			maxCap = 1.0
		}
		timeline := &MachineTimeline{
			MaxCapacity: maxCap,
			Allocations: make([]MachineAllocation, 0),
		}
		if m.Status == models.MachineBusy && m.CurrentBatchID != nil {
			batch, err := d.batchRepo.FindByID(ctx, *m.CurrentBatchID)
			if err == nil && batch != nil && batch.EstimatedCompletion != nil {
				nowTime := time.Now()
				if d.now != nil {
					nowTime = d.now()
				}
				if batch.EstimatedCompletion.After(nowTime) {
					timeline.AddAllocation(nowTime, *batch.EstimatedCompletion, maxCap)
				}
			}
		}
		result[m.ID] = timeline
	}

	activeTasks, err := d.taskRepo.FindByNode(ctx, nodeID, []models.TaskStatus{models.TaskPending, models.TaskActive, models.TaskWaiting})
	if err != nil {
		log.Printf("calcMachineTimelines: load active tasks: %v", err)
	} else {
		for _, task := range activeTasks {
			if task.MachineID == "" {
				continue
			}
			tl, ok := result[task.MachineID]
			if !ok {
				continue
			}
			step, err := d.sopRepo.FindStepByID(ctx, task.SOPStepID)
			if err != nil || step == nil {
				continue
			}

			start := task.ScheduledStart
			end := task.ScheduledEnd
			if task.OriginalKind == models.TaskKindSetup || task.TaskKind == models.TaskKindSetup {
				end = start.Add(time.Duration(step.Duration) * time.Second)
			} else if task.TaskKind == models.TaskKindRetrieve {
				// Retrieve endUsageTime is just its ScheduledEnd (already setup to the end of the duration)
				end = task.ScheduledEnd
			}

			tl.AddAllocation(start, end, task.RequiredSlots)
		}
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
// tính idle window của chúng, và assign QUEUED candidates phù hợp làm fill-in.
func (d *dispatcher) assignFillInTasks(
	ctx context.Context,
	nodeID string,
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
	machines []*models.Machine,
	machineTimelines map[string]*MachineTimeline,
) error {
	retrieveTasks, err := d.taskRepo.FindByNode(ctx, nodeID,
		[]models.TaskStatus{models.TaskPending, models.TaskWaiting})
	if err != nil {
		return fmt.Errorf("assignFillInTasks: load retrieve tasks: %w", err)
	}

	var retrieveList []*models.StaffTask
	for _, t := range retrieveTasks {
		if t.TaskKind == models.TaskKindRetrieve {
			retrieveList = append(retrieveList, t)
		}
	}
	if len(retrieveList) == 0 {
		return nil
	}

	allTasks, err := d.taskRepo.FindByNode(ctx, nodeID, nil)
	if err != nil {
		return fmt.Errorf("assignFillInTasks: load all tasks: %w", err)
	}

	var fillInPool []*models.StaffTask
	for _, c := range allTasks {
		if c.ParentTaskID == nil && c.Status != models.TaskCancelled {
			if c.TaskKind == models.TaskKindNormal || c.TaskKind == models.TaskKindSetup {
				if c.Status == models.TaskQueued || c.Status == models.TaskPending {
					fillInPool = append(fillInPool, c)
				}
			}
		}
	}

	// Batch load SOPSteps cho candidates
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

	used := make(map[string]bool)

	for _, retrieveTask := range retrieveList {
		if retrieveTask.AssignedTo == "" {
			continue
		}

		parentStep, err := d.sopRepo.FindStepByID(ctx, retrieveTask.SOPStepID)
		if err != nil || parentStep == nil || !parentStep.IsIdleStep {
			continue
		}
		if parentStep.AttentionLevel == models.AttentionActiveWait {
			continue // Không bao giờ fill-in khi ACTIVE_WAIT
		}

		// idleStart = lúc nhân viên RỜI máy (= SETUP.ScheduledEnd), không phải lúc máy xong.
		// RETRIEVE.ScheduledStart hiện tại = machineFreeAt (khi máy hoàn thành), không phải khi NV rời.
		// Phải tìm SETUP tương ứng để lấy thời điểm NV rảnh thực sự.
		idleStart := retrieveTask.ScheduledStart // fallback nếu không tìm được SETUP
		setupSibling := d.findSetupTaskForRetrieve(ctx, retrieveTask.POID, retrieveTask.SOPStepID, retrieveTask.BatchIndex)
		if setupSibling != nil {
			idleStart = setupSibling.ScheduledEnd
		}

		requiresAttentionAt := 0
		if parentStep.RequiresAttentionAt != nil {
			requiresAttentionAt = *parentStep.RequiresAttentionAt
		}
		idleEnd := retrieveTask.ScheduledEnd.Add(-time.Duration(requiresAttentionAt) * time.Second)
		log.Printf("DEBUG assignFillInTasks: retrieve step=%s idleStart=%s idleEnd=%s window=%s",
			retrieveTask.SOPStepID, idleStart.Format("15:04"), idleEnd.Format("15:04"), idleEnd.Sub(idleStart).String())

		// Loop to allow multiple fill-ins in the same window (up to maxFillInPerWindow)
		fillInCount := 0
		for {
			if fillInCount >= maxFillInPerWindow {
				log.Printf("DEBUG assignFillInTasks: reached maxFillInPerWindow (%d) for retrieve %s", maxFillInPerWindow, retrieveTask.ID)
				break
			}
			availableWindow := idleEnd.Sub(idleStart) - d.safetyBuf
			if availableWindow <= 0 {
				break
			}

			var validPool []*models.StaffTask
			for _, c := range fillInPool {
				// LUÔN kiểm tra EarliestStart: không được làm task khi dependency chưa xong.
				// EarliestStart = thời điểm sớm nhất bước trước hoàn thành (theo dependency chain).
				if idleStart.Before(c.EarliestStart) {
					continue
				}
				// Nếu đã là Pending, cũng phải kiểm tra ScheduledStart
				if c.Status == models.TaskPending && idleStart.After(c.ScheduledStart) {
					continue
				}
				validPool = append(validPool, c)
			}

			candidate, candidateStep, candidateMachineID, willSplit := d.findFillInCandidate(
				validPool, candidateSteps, parentStep, availableWindow, retrieveTask.AssignedTo, used, machines, machineTimelines, idleStart)
			if candidate == nil {
				break // No more candidates fit
			}

			// Determine candidate duration
			cDur := candidateStep.Duration
			if candidate.EstimatedDuration != nil {
				cDur = *candidate.EstimatedDuration
			} else if candidate.TaskKind == models.TaskKindSetup && candidateStep.ActiveTime != nil {
				cDur = *candidateStep.ActiveTime
			}
			assignedDuration := time.Duration(cDur) * time.Second

			var remainder *models.StaffTask
			if willSplit {
				assignedDuration = availableWindow

				// Calculate scaling factor
				factor := float64(assignedDuration) / float64(time.Duration(cDur)*time.Second)
				if factor > 1.0 {
					factor = 1.0
				}

				subQty := candidate.TargetQty * factor
				subSlots := candidate.RequiredSlots * factor

				remQty := candidate.TargetQty - subQty
				remSlots := candidate.RequiredSlots - subSlots

				// Initialize RootTaskID for candidate if it doesn't have one
				if candidate.RootTaskID == nil {
					candidate.RootTaskID = &candidate.ID
				}

				remDurationSec := cDur - int(assignedDuration.Seconds())

				// Clone for remainder
				remainder = &models.StaffTask{
					ID:                uuid.New().String(),
					POID:              candidate.POID,
					OrderItemID:       candidate.OrderItemID,
					SOPStepID:         candidate.SOPStepID,
					NodeID:            candidate.NodeID,
					BatchIndex:        candidate.BatchIndex,
					TaskKind:          candidate.TaskKind,
					OriginalKind:      candidate.OriginalKind,
					Status:            models.TaskQueued,
					TargetQty:         remQty,
					RequiredSlots:     remSlots,
					RootTaskID:        candidate.RootTaskID,
					EstimatedDuration: &remDurationSec,
					Priority:          candidate.Priority,
					EarliestStart:     candidate.EarliestStart,
					CreatedAt:         time.Now(),
				}
				// Ghi đè thông số cho mảnh vỡ hiện tại
				candidate.TargetQty = subQty
				candidate.RequiredSlots = subSlots
				estDur := int(assignedDuration.Seconds())
				candidate.EstimatedDuration = &estDur
			}

			// Assign fill-in
			retrieveID := retrieveTask.ID
			candidate.AssignedTo = retrieveTask.AssignedTo
			candidate.ParentTaskID = &retrieveID
			candidate.MachineID = candidateMachineID
			candidate.IsInterruptible = true
			if candidate.OriginalKind == "" {
				candidate.OriginalKind = candidate.TaskKind // Ghi nhớ SETUP hoặc NORMAL trước khi đổi
			}
			candidate.TaskKind = models.TaskKindFillIn
			candidate.Status = models.TaskPending
			candidate.ScheduledStart = idleStart
			candidate.ScheduledEnd = idleStart.Add(assignedDuration)

			if err := d.taskRepo.Update(ctx, candidate); err != nil {
				log.Printf("dispatcher.assignFillInTasks: update fill-in task %s: %v", candidate.ID, err)
				break
			}

			if remainder != nil {
				if err := d.taskRepo.Create(ctx, remainder); err != nil {
					log.Printf("dispatcher.assignFillInTasks: create remainder task %s: %v", remainder.ID, err)
				}
				// remainder sẽ nằm trên DB, đợi loop Dispatch vòng sau pick up.
			}

			if candidateMachineID != "" {
				if tl, ok := machineTimelines[candidateMachineID]; ok {
					endUsageTime := candidate.ScheduledEnd
					if candidate.OriginalKind == models.TaskKindSetup {
						endUsageTime = candidate.ScheduledStart.Add(time.Duration(candidateStep.Duration) * time.Second)

						// Cần update cascade time cho Retrieve task
						d.syncRetrieveAfterSetup(ctx, candidate.POID, candidate.SOPStepID, candidate.BatchIndex, candidate.ScheduledEnd, endUsageTime)

						// Update in memory so subsequent loops see the correct idleEnd
						for _, rt := range retrieveList {
							if rt.POID == candidate.POID && rt.SOPStepID == candidate.SOPStepID && rt.BatchIndex == candidate.BatchIndex {
								rt.EarliestStart = candidate.ScheduledEnd
								rt.ScheduledStart = candidate.ScheduledEnd
								rt.ScheduledEnd = endUsageTime
							}
						}
					}
					tl.AddAllocation(candidate.ScheduledStart, endUsageTime, candidate.RequiredSlots)
				}
			}

			used[candidate.ID] = true
			fillInCount++

			// Advance idleStart for the next candidate
			idleStart = candidate.ScheduledEnd
			log.Printf("dispatcher.assignFillInTasks: fill-in task %s → staff=%s for retrieve=%s (duration=%s, rem_window=%s)",
				candidate.ID, retrieveTask.AssignedTo, retrieveTask.ID, assignedDuration, idleEnd.Sub(idleStart)-d.safetyBuf)
		}
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
	machines []*models.Machine,
	machineTimelines map[string]*MachineTimeline,
	idleStart time.Time,
) (candidate *models.StaffTask, step *models.SOPStep, machineID string, willSplit bool) {
	for _, c := range pool {
		if used[c.ID] {
			log.Printf("DEBUG: findFillInCandidate rejecting %s because already used", c.SOPStepID)
			continue
		}
		// Task phải chưa được assign hoặc assign cho cùng staff
		if c.AssignedTo != "" && c.AssignedTo != staffID {
			log.Printf("DEBUG: findFillInCandidate rejecting %s because AssignedTo=%s != staffID=%s", c.SOPStepID, c.AssignedTo, staffID)
			continue
		}
		cStep, ok := poolSteps[c.SOPStepID]
		if !ok {
			log.Printf("DEBUG: findFillInCandidate rejecting %s because step not found", c.SOPStepID)
			continue
		}
		cDuration := time.Duration(cStep.Duration) * time.Second
		if c.EstimatedDuration != nil {
			cDuration = time.Duration(*c.EstimatedDuration) * time.Second
		} else if c.TaskKind == models.TaskKindSetup && cStep.ActiveTime != nil {
			cDuration = time.Duration(*cStep.ActiveTime) * time.Second
		}

		canSplit := false
		if cStep.IsSplittable {
			minTime := defaultMinUsefulTime
			if cStep.MinUsefulTime != nil {
				minTime = time.Duration(*cStep.MinUsefulTime) * time.Second
			}
			if availableWindow >= minTime && cDuration > availableWindow {
				canSplit = true
			}
		}

		mID := ""
		if cStep.EquipmentTypeID != nil {
			tempID, mFree := d.pickMachine(machines, machineTimelines, cStep, c.RequiredSlots, idleStart, c.EstimatedDuration)
			if tempID == "" || mFree.After(idleStart) {
				continue
			}
			mID = tempID
		}

		switch parentStep.AttentionLevel {
		case models.AttentionFullIdle, models.AttentionNearbyIdle:
			// NEARBY_IDLE: MVP treat như FULL_IDLE (skip max_distance check)
			if cDuration <= availableWindow || canSplit {
				return c, cStep, mID, canSplit
			} else {
				log.Printf("DEBUG: findFillInCandidate rejecting %s because duration %s > window %s", c.SOPStepID, cDuration, availableWindow)
			}

		case models.AttentionPeriodicCheck:
			if parentStep.CheckIntervalSec == nil {
				continue
			}
			windowPerInterval := time.Duration(*parentStep.CheckIntervalSec)*time.Second - d.safetyBuf
			if windowPerInterval <= 0 {
				continue
			}
			if cDuration <= windowPerInterval && cStep.IsIdleStep == false {
				return c, cStep, mID, false // Splitting is generally not safe during Periodic checks
			} else {
				log.Printf("DEBUG: findFillInCandidate rejecting %s because duration/idle constraints for PERIODIC_CHECK", c.SOPStepID)
			}
		}
	}
	return nil, nil, "", false
}
