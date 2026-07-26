package usecase

import (
	"context"
	"fmt"
	"log"
	"sort"
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

	for {
		assignedAny := false
		pass1Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
		if err != nil || len(pass1Tasks) == 0 {
			break
		}

		sort.Slice(pass1Tasks, func(i, j int) bool {
			return pass1Tasks[i].EarliestStart.Before(pass1Tasks[j].EarliestStart)
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
			if d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineFreeAt, now) {
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
	if err := d.assignFillInTasks(ctx, nodeID, shifts, staffFreeAt); err != nil {
		log.Printf("dispatcher.Dispatch: assignFillInTasks warning: %v", err)
	}

	for {
		assignedAny := false
		pass2Tasks, err := d.taskRepo.FindQueued(ctx, nodeID)
		if err != nil || len(pass2Tasks) == 0 {
			break
		}

		sort.Slice(pass2Tasks, func(i, j int) bool {
			return pass2Tasks[i].EarliestStart.Before(pass2Tasks[j].EarliestStart)
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
			if d.assignSingleTask(ctx, nodeID, task, shifts, staffFreeAt, machines, machineFreeAt, now) {
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
		return true
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
			if t.SOPStepID == depStepID && t.Status == models.TaskQueued {
				return false
			}
		}
	}
	return true
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

	var machineID string
	var mFreeAt time.Time

	if task.TaskKind == models.TaskKindRetrieve {
		// RETRIEVE phải dùng cùng máy với SETUP tương ứng (cùng SOPStepID + POID).
		// Không được để pickMachine tự chọn máy khác.
		machineID, mFreeAt = d.findSetupMachineForRetrieve(ctx, task.POID, task.SOPStepID, task.BatchIndex, machineFreeAt, now)
		if machineID == "" {
			// SETUP chưa được assign -> không thể assign RETRIEVE lúc này. Bỏ qua để loop vòng sau.
			return false
		}
	} else {
		machineID, mFreeAt = d.pickMachine(machines, machineFreeAt, step, now)
	}

	shift, sFreeAt := d.pickStaff(shifts, staffFreeAt, now)
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
		// RETRIEVE.ScheduledEnd = thời điểm máy hoàn thành toàn bộ chu kỳ.
		// = SETUP.scheduledStart + step.Duration (không phải RETRIEVE.scheduledStart + retrieveDuration).
		// Tìm SETUP tương ứng để lấy scheduledStart chính xác.
		setupStart := d.findSetupScheduledStart(ctx, task.POID, task.SOPStepID, task.BatchIndex, scheduledStart, step)
		scheduledEnd = setupStart.Add(time.Duration(step.Duration) * time.Second)
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
		// Máy thực sự bận đến khi toàn bộ step hoàn thành (scheduledStart + Duration),
		// không phải chỉ đến khi nhân viên rời đi (scheduledEnd của SETUP).
		machineCompleteAt := scheduledEnd.Add(time.Duration(step.Duration) * time.Second)
		if task.TaskKind == models.TaskKindSetup {
			// Cập nhật RETRIEVE tương ứng để phản ánh thời gian máy thực tế
			d.syncRetrieveAfterSetup(ctx, task.POID, task.SOPStepID, task.BatchIndex, scheduledEnd, machineCompleteAt)
			// Machine busy cho đến khi cycle hoàn tất
			if machineFreeAt[machineID].Before(machineCompleteAt) {
				machineFreeAt[machineID] = machineCompleteAt
			}
		} else {
			if machineFreeAt[machineID].Before(scheduledEnd) {
				machineFreeAt[machineID] = scheduledEnd
			}
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
	machineFreeAt map[string]time.Time,
	now time.Time,
) (machineID string, freeAt time.Time) {
	tasks, err := d.taskRepo.FindByPO(ctx, poID)
	if err != nil {
		return "", now
	}
	for _, t := range tasks {
		if t.SOPStepID == sopStepID && t.TaskKind == models.TaskKindSetup && t.BatchIndex == batchIndex && t.MachineID != "" {
			mFree, ok := machineFreeAt[t.MachineID]
			if !ok {
				mFree = now
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
// tính idle window của chúng, và assign QUEUED candidates phù hợp làm fill-in.
func (d *dispatcher) assignFillInTasks(
	ctx context.Context,
	nodeID string,
	shifts []*models.StaffShift,
	staffFreeAt map[string]time.Time,
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

		// Loop to allow multiple fill-ins in the same window
		for {
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

			candidate, candidateStep := d.findFillInCandidate(
				validPool, candidateSteps, parentStep, availableWindow, retrieveTask.AssignedTo, used)
			if candidate == nil {
				break // No more candidates fit
			}

			// Determine candidate duration
			cDur := candidateStep.Duration
			if candidate.TaskKind == models.TaskKindSetup && candidateStep.ActiveTime != nil {
				cDur = *candidateStep.ActiveTime
			}
			assignedDuration := time.Duration(cDur) * time.Second

			// Assign fill-in
			retrieveID := retrieveTask.ID
			candidate.AssignedTo = retrieveTask.AssignedTo
			candidate.ParentTaskID = &retrieveID
			candidate.IsInterruptible = true
			candidate.OriginalKind = candidate.TaskKind // Ghi nhớ SETUP hoặc NORMAL trước khi đổi
			candidate.TaskKind = models.TaskKindFillIn
			candidate.Status = models.TaskPending
			candidate.ScheduledStart = idleStart
			candidate.ScheduledEnd = idleStart.Add(assignedDuration)

			if err := d.taskRepo.Update(ctx, candidate); err != nil {
				log.Printf("dispatcher.assignFillInTasks: update fill-in task %s: %v", candidate.ID, err)
				break
			}

			used[candidate.ID] = true
			
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
) (candidate *models.StaffTask, step *models.SOPStep) {
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
		if c.TaskKind == models.TaskKindSetup && cStep.ActiveTime != nil {
			cDuration = time.Duration(*cStep.ActiveTime) * time.Second
		}

		switch parentStep.AttentionLevel {
		case models.AttentionFullIdle, models.AttentionNearbyIdle:
			// NEARBY_IDLE: MVP treat như FULL_IDLE (skip max_distance check)
			if cDuration <= availableWindow {
				return c, cStep
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
				return c, cStep
			} else {
				log.Printf("DEBUG: findFillInCandidate rejecting %s because duration/idle constraints for PERIODIC_CHECK", c.SOPStepID)
			}
		}
	}
	return nil, nil
}
