# C.3 — Scheduling Engine: Implementation Plan

## Tổng quan

File: `internal/usecase/scheduling_engine.go`  
Dependencies sẵn có: Block A ✅, Block B ✅ (repos đầy đủ)  
Thời gian ước tính: ~10h

---

## Phần 1 — Kiến trúc tổng thể

### 1.1 Struct & Interface

```go
// SchedulingEngine lo việc planning: build DAG + tạo tasks ở trạng thái QUEUED.
// KHÔNG assign staff/machine — đó là việc của Dispatcher.
type SchedulingEngine interface {
    // SchedulePO: entry point chính — gọi khi PO → IN_PROGRESS.
    // Build DAG từ SOP, tạo StaffTask ở trạng thái QUEUED (chưa assign).
    // Sau khi tạo xong → gọi dispatcher.Dispatch() để assign ngay (sync).
    SchedulePO(ctx context.Context, poID string) ([]*models.StaffTask, error)

    // RescheduleOnShiftChange: gọi khi có shift mới bắt đầu hoặc kết thúc.
    // Trigger Dispatcher chạy lại để assign QUEUED tasks còn trống.
    RescheduleOnShiftChange(ctx context.Context, nodeID string) error
}

// Dispatcher lo việc assignment: kéo QUEUED task phù hợp → assign resource.
type Dispatcher interface {
    // Dispatch: gọi khi có resource rảnh (staff hoàn thành task, PO mới tạo, shift thay đổi).
    // Chạy synchronous trong cùng request — MVP không cần background worker.
    Dispatch(ctx context.Context, nodeID string) error
}

type schedulingEngine struct {
    poRepo        services.ProductionOrderRepository
    sopRepo       services.SOPRepository
    machineRepo   services.MachineRepository
    batchRepo     services.ProductionBatchRepository // tính machine free_at
    shiftRepo     services.StaffShiftRepository
    taskRepo      services.StaffTaskRepository
    dispatcher    Dispatcher
    safetyBuffer  time.Duration   // default 30s
    now           func() time.Time // injectable cho testing
}

type dispatcher struct {
    shiftRepo   services.StaffShiftRepository
    machineRepo services.MachineRepository
    taskRepo    services.StaffTaskRepository
    sopRepo     services.SOPRepository
    now         func() time.Time
}
```

### 1.2 Cấu trúc nội bộ

**SchedulingEngine** — chỉ lo planning:

```
SchedulePO(poID)
  │
  ├── [1] loadPO(poID) → PO + SOP + Steps
  ├── [2] idempotency check: taskRepo.FindByPO(poID) → lọc steps cần schedule
  ├── [3] buildDAG(steps) → topo order [][]SOPStep (các nhóm song song)
  │
  └── for each group in topo order:
        for each step in group:
          ├── [4] calcEstimatedSchedule(depDoneAt) → ước tính start/end (không cam kết)
          ├── [5] buildTask(s)/buildIdleTasks() → StaffTask(Status=QUEUED)
          └── [6] taskRepo.Create(task)
  │
  └── [7] dispatcher.Dispatch(ctx, po.NodeID)  ← gọi sync sau khi tạo xong tasks
```

**Dispatcher** — chỉ lo assignment:

```
Dispatch(nodeID)
  │
  ├── [1] Load QUEUED tasks tại nodeID → []StaffTask
  ├── [2] Load active shifts + machine status tại nodeID
  │
  └── for each free resource:
        ├── [3] candidates = QUEUED tasks phù hợp với resource (equipTypeID match)
        ├── [4] sort candidates theo EDD (due_time của PO) — Phase 2
        │         MVP: FIFO (theo thứ tự CreatedAt)
        ├── [5] pickMachine(nodeID, step) → machine, machineFreeAt
        ├── [6] calcSchedule(depDoneAt, machineFreeAt, staffFreeAt)
        ├── [7] task.Status = PENDING, task.AssignedTo = staffID, task.MachineID = machineID
        └── [8] taskRepo.Update(task)

  └── Fill-in logic (sau khi assign xong NORMAL tasks):
        for each WAITING task với idle window còn trống:
          ├── findFillInTask(idleWindow, staff, nodeID)
          └── assign fill-in nếu tìm được
```

---

## Phần 2 — Chi tiết từng bước implement

### Bước 1 — Load PO + SOP

```go
func (e *schedulingEngine) loadPOContext(ctx, poID) (*poSchedulingCtx, error)
```

```
- poRepo.FindByID(poID) → PO (cần: SOPID, NodeID, Status)
- Guard: PO.Status != IN_PROGRESS → return ErrPONotInProgress
- sopRepo.ListSteps(PO.SOPID) → []SOPStep
- Guard: len(steps) == 0 → return ErrSOPHasNoSteps
```

**Edge case:**
- PO không tồn tại → `ErrPONotFound`
- PO đã được schedule (đã có tasks) → kiểm tra `taskRepo.FindByPO()`, nếu đã có tasks PENDING/ACTIVE → idempotent return (không tạo lại)

---

### Bước 2 — Build Dependency DAG (Kahn's Algorithm)

```go
func buildTopoGroups(steps []*models.SOPStep) ([][]models.SOPStep, error)
```

**Thuật toán Kahn:**

```
inDegree := map[stepID]int
graph    := map[stepID][]stepID  // A → [B, C] = A mở khoá B và C

for each step:
    inDegree[step.ID] = len(step.DependsOn)
    for each dep in step.DependsOn:
        graph[dep] = append(graph[dep], step.ID)

queue := steps với inDegree == 0 (root nodes)
result := [][]SOPStep  // mỗi phần tử là 1 group có thể chạy song song

while queue không rỗng:
    group = queue (snapshot hiện tại)
    result = append(result, group)
    nextQueue = []
    for each step in group:
        for each child in graph[step.ID]:
            inDegree[child]--
            if inDegree[child] == 0:
                nextQueue = append(nextQueue, child)
    queue = nextQueue

if tổng số bước đã xử lý != len(steps):
    return ErrCyclicDependency
```

**Edge cases:**
- Steps không có `DependsOn` → tất cả vào group đầu tiên (chạy song song toàn bộ)
- DependsOn trỏ đến stepID không tồn tại → `ErrInvalidDependency`
- Circular dependency (A → B → A) → `ErrCyclicDependency`
- Single step → 1 group duy nhất

---

### Bước 3 — Load Staff Available

```go
func (e *schedulingEngine) loadStaffContext(ctx, nodeID) (*staffContext, error)
```

```
- shiftRepo.FindActiveByNode(nodeID) → []StaffShift
- Nếu rỗng → return staffCtx rỗng (tasks sẽ được tạo với AssignedTo = "")
- Với mỗi shift:
    taskRepo.FindByStaff(staffID, [PENDING, ACTIVE, WAITING]) → tasks hiện tại
    staffFreeAt[staffID] = max(task.ScheduledEnd) hoặc now() nếu không có task
```

---

### Bước 4 — Pick Machine

```go
func (e *schedulingEngine) pickMachine(ctx, nodeID, step) (*models.Machine, time.Time, error)
```

```
- Nếu step.EquipmentTypeID == nil → machine = nil, freeAt = now() (manual step)
- machineRepo.FindIdleByStationType(nodeID, *step.EquipmentTypeID)
- Nếu có idle machine → pick machine thứ nhất, freeAt = now()
- Nếu KHÔNG có idle machine:
    - Tìm tất cả machines cùng type (FindByNodeAndType)
    - Tính freeAt = min(machine.estimatedFreeAt) ← từ batch đang chạy
    - Dùng machine đó, freeAt = thời điểm nó free sớm nhất
    - Nếu không có machine nào → return nil, zero, ErrNoMachineAvailable
      → task.Status = PENDING, AssignedTo = "", MachineID = ""
```

> **Lưu ý:** MVP dùng `MachineIdle` status từ repo. Phase 2 sẽ dùng timeline từ batches.

---

### Bước 5 — Pick Staff

```go
func (e *schedulingEngine) pickStaff(sc *staffContext, equipTypeID *string) (*models.StaffShift, time.Time)
```

```
- Nếu equipTypeID == nil → pick bất kỳ staff nào free sớm nhất
- Lọc shifts: shift.StationID == equipTypeID (hoặc StationID == nil = flexible)
- Nếu không có staff nào → return nil, zero (unassigned)
- Tìm staff có staffFreeAt sớm nhất (FIFO)
- Trả về shift + freeAt
```

---

### Bước 6 — Calculate Schedule

```go
func calcSchedule(depDoneAt, machineFreeAt, staffFreeAt time.Time, duration int) (start, end time.Time)
```

```
scheduledStart = max(depDoneAt, machineFreeAt, staffFreeAt)
scheduledEnd   = scheduledStart + duration*seconds
```

---

### Bước 7a — Build Normal Task (is_idle_step = false)

```go
func buildNormalTask(po, step, machine, staff, start, end) *models.StaffTask
```

```
StaffTask{
    ID:             uuid(),
    POID:           po.ID,
    SOPStepID:      step.ID,
    NodeID:         po.NodeID,
    AssignedTo:     "",          // Dispatcher sẽ fill sau
    MachineID:      "",          // Dispatcher sẽ fill sau
    Status:         TaskQueued,  // ← QUEUED, chưa assign
    TaskKind:       TaskKindNormal,
    Priority:       step.SeqNo,
    IsInterruptible: false,
    ParentTaskID:   nil,
    ScheduledStart: estimatedStart, // ước tính, Dispatcher sẽ recalc khi assign
    ScheduledEnd:   estimatedEnd,   // ước tính
    CreatedAt:      now(),
}
```

> **Lưu ý:** `ScheduledStart/End` ở đây là **estimate** dùng cho UI "dự kiến".  
> Dispatcher sẽ recalculate và ghi lại giá trị **chính xác** khi assign resource thực tế.

---

### Bước 7b — Build Idle Tasks (is_idle_step = true)

Mỗi idle step → 2 tasks + 1 idle window tracker:

```go
func buildIdleTasks(po, step, machine, staff, start) (setup, waiting *models.StaffTask)
```

```
activeTime = *step.ActiveTime (giây)
totalDuration = step.Duration (giây)
requiresAttentionAt = step.RequiresAttentionAt (giây, mặc định 0)

setupTask:
    ID:             uuid()
    SOPStepID:      step.ID + "_SETUP"  ← cần convention đặt tên
    ScheduledStart: start
    ScheduledEnd:   start + activeTime
    Status:         PENDING
    IsInterruptible: false

waitingTask: (đây là WAITING anchor — máy đang chạy)
    ID:             uuid()
    SOPStepID:      step.ID + "_RETRIEVE"
    ScheduledStart: start + activeTime
    ScheduledEnd:   start + totalDuration
    Status:         PENDING  (sẽ → WAITING khi setupTask DONE)
    IsInterruptible: false
    ParentTaskID:   nil  (đây là parent, không phải fill-in)

return setupTask, waitingTask
```

> **Convention quan trọng:** SOPStepID của SETUP/RETRIEVE được đánh dấu bằng suffix. `StaffTask` model cần thêm field `TaskKind` để phân biệt (SETUP | RETRIEVE | FILL_IN | NORMAL). Xem phần Edge Cases.

**Idle window:**
```
idleStart       = setupTask.ScheduledEnd
idleEnd         = waitingTask.ScheduledEnd - requiresAttentionAt
availableWindow = idleEnd - idleStart
```

---

### Bước 8 — Find & Insert Fill-In Task

```go
func (e *schedulingEngine) findFillInTask(
    ctx context.Context,
    nodeID, staffID string,
    step *models.SOPStep,
    idleStart, idleEnd time.Time,
    waitingTaskID string,
) (*models.StaffTask, error)
```

**Logic theo AttentionLevel:**

```
availableWindow = idleEnd - idleStart - safetyBuffer (30s)
if availableWindow <= 0: return nil  // window quá hẹp

switch step.AttentionLevel:

case FULL_IDLE:
    candidates = taskRepo.FindByNode(nodeID, [PENDING])
    filter:
        task.AssignedTo == "" hoặc task.AssignedTo == staffID
        task.ParentTaskID == nil (chưa được assign làm fill-in)
        step.Duration <= availableWindow
        step.IsInterruptible == true (nếu muốn chèn vào)

case NEARBY_IDLE:
    // Defer MVP: chỉ dùng FULL_IDLE logic, bỏ qua max_distance constraint
    // → treat as FULL_IDLE nhưng filter thêm: step.EquipmentTypeID == same equipment type

case PERIODIC_CHECK:
    if step.CheckIntervalSec == nil: return nil
    windowPerInterval = *step.CheckIntervalSec - safetyBuffer
    if windowPerInterval <= 0: return nil
    candidates = taskRepo.FindByNode(nodeID, [PENDING])
    filter:
        task duration <= windowPerInterval
        task.IsInterruptible == true  // BẮT BUỘC

case ACTIVE_WAIT:
    return nil  // không fill-in

Nếu tìm được candidate:
    candidate.AssignedTo = staffID
    candidate.ParentTaskID = &waitingTaskID
    candidate.IsInterruptible = true
    candidate.ScheduledStart = idleStart
    candidate.ScheduledEnd = idleStart + candidate.Duration (từ SOPStep)
    taskRepo.Update(candidate)
    return candidate

Nếu không có: return nil (nhân viên được nghỉ)
```

---

## Phần 3 — Quyết định thiết kế quan trọng (cần thống nhất trước khi code)

### D1 — SOPStepID cho SETUP/RETRIEVE tasks

**Vấn đề:** `StaffTask.SOPStepID` là FK → SOPStep, nhưng SETUP và RETRIEVE không phải là 2 SOPStep riêng biệt trong DB.

**Giải pháp đề xuất — Thêm `TaskKind` field vào StaffTask:**

```go
type TaskKind string

const (
    TaskKindNormal   TaskKind = "NORMAL"   // Bước thường (is_idle_step=false)
    TaskKindSetup    TaskKind = "SETUP"    // Phần setup của idle step
    TaskKindRetrieve TaskKind = "RETRIEVE" // Phần lấy ra của idle step
    TaskKindFillIn   TaskKind = "FILL_IN"  // Fill-in task trong idle window
)
```

Cả SETUP và RETRIEVE vẫn dùng cùng `SOPStepID` (của bước gốc), nhưng `TaskKind` phân biệt vai trò.  
→ **Cần thêm `TaskKind` vào `StaffTask` model, `staffTaskDoc` BSON, và repo.** (Thay đổi model nhỏ nhưng cần thiết)

### D2 — Machine free_at estimation

**Vấn đề:** `MachineStatus.BUSY` nhưng không có `estimatedCompletion` trực tiếp trên Machine model.  
Machine.CurrentBatchID → cần load batch → `batch.ScheduledEnd`.

**Giải pháp MVP:** Load `ProductionBatch` từ machine.CurrentBatchID để lấy `ScheduledEnd`.  
Cần inject `batchRepo` vào `schedulingEngine`.

### D3 — Idempotency

**Vấn đề:** `SchedulePO` được gọi lại khi shift thay đổi.

**Giải pháp:** Đầu `SchedulePO`, build `scheduledStepIDs` từ `taskRepo.FindByPO(poID)`:

```
existingTasks = taskRepo.FindByPO(poID)

// Phân loại từng SOPStep theo trạng thái tasks tương ứng
doneStepIDs      := {task.SOPStepID | task.Status == DONE}
activeLiveIDs    := {task.SOPStepID | task.Status in [PENDING, ACTIVE, WAITING]}
cancelledOnlyIDs := {task.SOPStepID | ALL tasks của step đó là CANCELLED}
noTaskIDs        := {step.ID | không có task nào tương ứng}

// Chỉ schedule các bước thuộc 2 nhóm cuối
stepsToSchedule = steps có ID ∈ (cancelledOnlyIDs ∪ noTaskIDs)

// Skip hoàn toàn các bước trong doneStepIDs và activeLiveIDs
```

**Lý do bỏ case "tất cả DONE = gián đoạn":** Case này unreachable.
Nếu tất cả tasks DONE → PO.Status đã là `COMPLETED` → guard `ErrPONotInProgress`
đã reject trước khi vào đây. Engine không bao giờ nhìn thấy state này.

**3 trường hợp idempotency thực tế:**
- Step có task PENDING/ACTIVE/WAITING → **bỏ qua** (đang chạy bình thường)
- Step có task DONE → **bỏ qua** (đã hoàn thành)
- Step có task CANCELLED hoặc chưa có task nào → **re-schedule** (bị gián đoạn hoặc mới)

### D4 — Duration của fill-in task

`StaffTask.Duration` không có trong model hiện tại. Fill-in cần biết duration để check fit.  
→ Phải load SOPStep của fill-in candidate để lấy `step.Duration`.  
→ `FindByNode(PENDING)` trả về tasks, nhưng cần join với SOPStep.

**Giải pháp:** Load tất cả PENDING tasks → batch load SOPSteps của chúng → filter.

---

## Phần 4 — Danh sách test cases đầy đủ

### T1 — Happy Path: 1 Step, Non-Idle

**Setup:**
```
Node: KITCHEN_01
Machine: FRYER_01 (type=FRYER, status=IDLE)
Staff: Minh (shift ACTIVE, station=FRYER)
SOP: 1 step — duration=600s, is_idle_step=false, equipment_type=FRYER
PO: status=IN_PROGRESS
```

**Input:** `SchedulePO("po_001")`

**Expected:**
```
tasks = 1 StaffTask
task.SOPStepID      = step.ID
task.AssignedTo     = "staff_minh"
task.MachineID      = "FRYER_01"
task.Status         = PENDING
task.ScheduledStart ≈ now()
task.ScheduledEnd   ≈ now() + 600s
task.TaskKind       = NORMAL
task.IsInterruptible = false
task.ParentTaskID   = nil
```

---

### T2 — Happy Path: 1 Step, Idle (FULL_IDLE), Có Fill-In Task

**Setup:**
```
Node: KITCHEN_01
Machine: FRYER_01 (type=FRYER, status=IDLE)
Staff: Minh (shift ACTIVE, station=FRYER)
SOP: 1 step — duration=600s, is_idle_step=true, active_time=60, 
     attention_level=FULL_IDLE, requires_attention_at=120
Existing pending task: "PHA_TRA_SUA" (duration=120s, assigned_to="", interruptible=true)
PO: status=IN_PROGRESS
Now = T0
```

**Expected:**
```
tasks = 3 StaffTask

setupTask:
    TaskKind         = SETUP
    SOPStepID        = step.ID
    ScheduledStart   = T0
    ScheduledEnd     = T0 + 60s
    Status           = PENDING
    IsInterruptible  = false

waitingTask:
    TaskKind         = RETRIEVE
    SOPStepID        = step.ID
    ScheduledStart   = T0 + 60s
    ScheduledEnd     = T0 + 600s
    Status           = PENDING
    IsInterruptible  = false

--- Idle Window ---
idleStart        = T0 + 60s
idleEnd          = T0 + 600s - 120s = T0 + 480s
availableWindow  = 480s - 60s = 420s (sau khi trừ safetyBuffer 30s → 390s)
"PHA_TRA_SUA" duration = 120s ≤ 390s → FIT ✅

fillInTask (updated PHA_TRA_SUA):
    TaskKind         = FILL_IN
    AssignedTo       = "staff_minh"
    ParentTaskID     = waitingTask.ID
    IsInterruptible  = true
    ScheduledStart   = T0 + 60s
    ScheduledEnd     = T0 + 60s + 120s = T0 + 180s
    --- phải ≤ idleEnd - safetyBuffer = T0 + 480s - 30s = T0 + 450s ✅
```

---

### T3 — Idle Step, FULL_IDLE, Fill-In Task Quá Dài → Không Chèn

**Setup:** Giống T2, nhưng PHA_TRA_SUA duration = 400s

**Expected:**
```
tasks = 2 StaffTask (chỉ SETUP + RETRIEVE)
Không có fill-in task
--- Lý do: 400s > availableWindow (390s) → reject ---
```

---

### T4 — Idle Step, ACTIVE_WAIT → Không Bao Giờ Fill-In

**Setup:**
```
SOP step: is_idle_step=true, attention_level=ACTIVE_WAIT
Có sẵn pending task "PHA_TRA_SUA" duration=30s
```

**Expected:**
```
tasks = 2 StaffTask (SETUP + RETRIEVE)
Không có fill-in task (ACTIVE_WAIT không bao giờ fill-in)
```

---

### T5 — Idle Step, PERIODIC_CHECK

**Setup:**
```
SOP step: is_idle_step=true, attention_level=PERIODIC_CHECK,
          check_interval_sec=90, duration=600s, active_time=60,
          requires_attention_at=120
Pending task A: duration=50s, is_interruptible=true  → FIT
Pending task B: duration=70s, is_interruptible=true  → NOT FIT (70 > 90-30=60)
Pending task C: duration=40s, is_interruptible=false → NOT FIT (must be interruptible)
```

**Expected:**
```
tasks = 3 (SETUP + RETRIEVE + fill-in = task A)
fillIn.SOPStepID = taskA.SOPStepID
fillIn.IsInterruptible = true (enforced bởi engine)
```

---

### T6 — Multi-Step Linear SOP (Sequential Dependencies)

**Setup:**
```
SOP: 3 steps
    Step A: SeqNo=1, DependsOn=[], FRYER, duration=300s
    Step B: SeqNo=2, DependsOn=[A.ID], GRILL, duration=200s
    Step C: SeqNo=3, DependsOn=[B.ID], MANUAL, duration=60s
Machines: FRYER_01 (IDLE), GRILL_01 (IDLE)
Staff: Minh (FRYER), An (GRILL), Binh (MANUAL/flexible)
Now = T0
```

**Expected:**
```
taskA:
    ScheduledStart = T0
    ScheduledEnd   = T0 + 300s

taskB:
    ScheduledStart = T0 + 300s  (dep_done_at = T0+300, machine IDLE = T0, staff IDLE = T0)
    ScheduledEnd   = T0 + 500s

taskC:
    ScheduledStart = T0 + 500s
    ScheduledEnd   = T0 + 560s
```

---

### T7 — Multi-Step Parallel SOP

**Setup:**
```
SOP: 3 steps
    Step A: DependsOn=[], FRYER, duration=300s
    Step B: DependsOn=[], GRILL, duration=200s  ← PARALLEL với A
    Step C: DependsOn=[A.ID, B.ID], MANUAL, duration=60s  ← chờ CẢ A và B
Machines: FRYER_01 (IDLE), GRILL_01 (IDLE)
Staff: Minh (FRYER), An (GRILL)
Now = T0
```

**Expected:**
```
taskA: start=T0, end=T0+300s
taskB: start=T0, end=T0+200s  (CÙNG LÚC với A)
taskC:
    dep_done_at    = max(T0+300, T0+200) = T0+300
    ScheduledStart = T0 + 300s
    ScheduledEnd   = T0 + 360s
```

---

### T8 — Machine Busy (Not Idle)

**Setup:**
```
FRYER_01: status=BUSY, CurrentBatchID="batch_001"
batch_001.ScheduledEnd = T0 + 180s
Step A: equipment_type=FRYER, duration=300s
Staff: Minh (FRYER, free at T0)
```

**Expected:**
```
taskA:
    MachineID      = "FRYER_01"
    machineFreeAt  = T0 + 180s
    ScheduledStart = T0 + 180s  (max(depDone=T0, machFree=T0+180, staffFree=T0))
    ScheduledEnd   = T0 + 480s
```

---

### T9 — Staff Busy (Có Task Đang Scheduled)

**Setup:**
```
Staff Minh đã có 1 task scheduled:
    existingTask.ScheduledEnd = T0 + 240s
Step mới: FRYER, duration=300s
Machine FRYER_01: IDLE
```

**Expected:**
```
taskNew:
    staffFreeAt    = T0 + 240s
    ScheduledStart = T0 + 240s  (max(depDone=T0, machFree=T0, staffFree=T0+240))
    ScheduledEnd   = T0 + 540s
```

---

### T10 — Không Có Staff Available

**Setup:**
```
Node KITCHEN_01: không có active shifts
Step: FRYER, duration=300s
```

**Expected:**
```
task tạo ra nhưng:
    AssignedTo = ""  (unassigned)
    Status     = PENDING
    MachineID  = "FRYER_01" (hoặc "" nếu không có machine)
log warning: "no available staff for step <stepID>, task unassigned"
Không return error (graceful degradation)
```

---

### T11 — Không Có Machine Available

**Setup:**
```
Không có machine cùng type với step
Step: equipment_type=FRYER, nhưng không có FRYER nào tại node
```

**Expected:**
```
task tạo ra:
    AssignedTo = "staff_minh"
    MachineID  = ""
    Status     = PENDING
log warning: "no machine available for step <stepID>"
Không return error (graceful degradation)
```

---

### T12 — Cyclic Dependency → Error

**Setup:**
```
SOP: 2 steps
    Step A: DependsOn=[B.ID]
    Step B: DependsOn=[A.ID]
```

**Expected:**
```
SchedulePO returns ErrCyclicDependency
Không tạo tasks nào
```

---

### T13 — Invalid DependsOn (stepID không tồn tại)

**Setup:**
```
SOP: 1 step, DependsOn=["nonexistent-step-id"]
```

**Expected:**
```
SchedulePO returns ErrInvalidDependency
```

---

### T14 — Idempotency: Gọi SchedulePO 2 Lần

**Setup:**
```
PO: IN_PROGRESS
Lần 1: SchedulePO("po_001") → tạo 1 task
Lần 2: SchedulePO("po_001") → gọi lại
```

**Expected:**
```
Lần 2: return existing tasks, không tạo thêm task mới
len(all tasks by PO) vẫn = 1
```

---

### T15 — RescheduleOnShiftChange: Staff Mới Vào Ca, Có Unassigned Tasks

**Setup:**
```
Đã có task T1 (AssignedTo="", Status=PENDING)
Staff Minh mới bắt đầu ca (station=FRYER)
T1 là step cần FRYER
```

**Input:** `RescheduleOnShiftChange("KITCHEN_01")`

**Expected:**
```
T1.AssignedTo = "staff_minh"
T1.ScheduledStart = now() (recalculated)
T1.ScheduledEnd = now() + step.Duration
taskRepo.Update(T1) được gọi
```

---

### T16 — RescheduleOnShiftChange: Staff Ra Ca, Có PENDING Tasks

**Setup:**
```
Staff Minh: shift → ENDED
Minh có task T1 (Status=PENDING)
Staff An còn đang active (station=FRYER)
```

**Expected:**
```
T1.AssignedTo = "staff_an"  (reassign)
T1 recalculated schedule
Không affect ACTIVE/WAITING tasks (chúng không bị unassign)
```

---

### T17 — PO Không Tồn Tại

**Input:** `SchedulePO("nonexistent-po")`

**Expected:**
```
return nil, ErrPONotFound
```

---

### T18 — PO Chưa IN_PROGRESS (vẫn PENDING)

**Setup:** PO.Status = PENDING

**Expected:**
```
return nil, ErrPONotInProgress
```

---

### T19 — Idle Step, Không Có Fill-In Task Available Nào

**Setup:**
```
is_idle_step=true, attention_level=FULL_IDLE
Không có PENDING tasks nào tại node
```

**Expected:**
```
tasks = 2 (SETUP + RETRIEVE)
Không lỗi — đây là valid state (nhân viên được nghỉ trong idle)
```

---

### T20 — Idle Window Quá Nhỏ (< safetyBuffer)

**Setup:**
```
is_idle_step=true, duration=90s, active_time=60s, requires_attention_at=20s, safetyBuffer=30s
idleWindow = 90 - 60 - 20 = 10s
10s < 30s safetyBuffer → không fit bất kỳ task nào
Có pending task duration=5s
```

**Expected:**
```
tasks = 2 (SETUP + RETRIEVE)
Không fill-in (window không đủ)
```

---

### T21 — Staff Flexible (StationID = nil)

**Setup:**
```
Staff Minh: shift.StationID = nil (flexible)
Step cần FRYER
```

**Expected:**
```
Staff Minh được assign dù không có station cứng
task.AssignedTo = "staff_minh"
```

---

### T22 — Nhiều Staff Cùng Station, Pick Người Free Sớm Nhất

**Setup:**
```
Staff Minh: FRYER, freeAt = T0 + 300s
Staff An:   FRYER, freeAt = T0 + 100s  ← free sớm hơn
Step: FRYER, duration=60s
```

**Expected:**
```
task.AssignedTo = "staff_an"  (FIFO — pick người free sớm nhất)
task.ScheduledStart = T0 + 100s
```

---

### T23 — Manual Step (EquipmentTypeID = nil)

**Setup:**
```
Step: EquipmentTypeID = nil, duration=120s
Staff Minh: flexible
```

**Expected:**
```
task.MachineID = ""
task.AssignedTo = "staff_minh"
task.ScheduledStart = T0
```

---

## Phần 5 — Files cần tạo/sửa

### File mới
- `internal/usecase/scheduling_engine.go` — Engine chính (interface + impl)
- `internal/usecase/scheduling_engine_test.go` — Unit tests (dùng mock repos)

### File sửa (nhỏ)
- `internal/domain/models/staff_task.go` — **Thêm `TaskKind` field** (D1)
- `internal/infrastructure/persistence/mongodb/staff_task_repository.go` — Sync `staffTaskDoc` với `TaskKind`
- `internal/usecase/allocation_engine.go` — Thêm C.4 hook sau batch DONE
- `internal/usecase/order_orchestrator.go` — Thêm C.5 trigger khi PO → IN_PROGRESS

### Dependencies cần inject
```go
func NewSchedulingEngine(
    poRepo      services.ProductionOrderRepository,
    sopRepo     services.SOPRepository,
    machineRepo services.MachineRepository,
    batchRepo   services.ProductionBatchRepository,  // để tính machine free_at
    shiftRepo   services.StaffShiftRepository,
    taskRepo    services.StaffTaskRepository,
) SchedulingEngine
```

---

## Phần 6 — Thứ tự implement (session by session)

| Session | Việc làm | Output |
|---|---|---|
| **S1** (~1h) | Thêm `TaskKind` vào model + repo, build DAG (Kahn) | `buildTopoGroups()` với unit test T12, T13 |
| **S2** (~2h) | `loadPOContext`, `pickMachine`, `pickStaff`, `calcSchedule` | `SchedulePO` chạy được cho T1, T8, T9, T10, T11 |
| **S3** (~2h) | `buildIdleTasks`, idle window logic | T2, T3, T4, T5 pass |
| **S4** (~2h) | `findFillInTask` cho FULL_IDLE + PERIODIC_CHECK | T2, T5 pass end-to-end |
| **S5** (~1h) | `RescheduleOnShiftChange`, idempotency | T14, T15, T16 pass |
| **S6** (~1h) | Wire vào `allocation_engine.go` (C.4) + `order_orchestrator.go` (C.5) | Build passes, C.4/C.5 wired |
| **S7** (~1h) | Mock repos cho integration tests (Block E prep) | `mocks.go` mở rộng |

---

## Phần 7 — Error Types

```go
var (
    ErrPONotFound        = errors.New("production order not found")
    ErrPONotInProgress   = errors.New("production order is not IN_PROGRESS")
    ErrSOPHasNoSteps     = errors.New("SOP has no steps")
    ErrCyclicDependency  = errors.New("cyclic dependency detected in SOP steps")
    ErrInvalidDependency = errors.New("step depends on non-existent step ID")
    ErrNoMachineAvailable = errors.New("no machine available for step") // non-fatal → log only
)
```

> **Quan trọng:** `ErrNoMachineAvailable` và "no staff" **KHÔNG** là fatal errors.  
> Engine vẫn tạo task với empty fields, log warning, và tiếp tục.  
> Chỉ cyclic/invalid dependency mới là hard errors.

---

## Phần 8 — Kiến trúc Pull + Dispatcher: Quyết định & Rationale

> ✅ **Đã quyết định** — Phần này ghi lại các vấn đề phát hiện được và quyết định kiến trúc đã thống nhất.

---

### 8.1 — Ba vấn đề của Push model (đã giải quyết)

**Push model ban đầu:** `SchedulePO()` assign ngay machine + staff tại thời điểm PO vào IN_PROGRESS.

Vấn đề:

#### ❌ Vấn đề 1: Không gom batch (Batch Grouping)

```
Tình huống:
  po_001 → task CHIÊN_GÀ (FRYER, 5 phút) ← đang PENDING, chưa bắt đầu
  po_002 → task CHIÊN_GÀ (FRYER, 5 phút) ← vừa được tạo

Push model: tạo 2 task riêng, chạy lần lượt → 10 phút
Thực tế tối ưu: gom vào 1 batch, chạy chung 1 lần → 5 phút
```

**Giải pháp:** Dispatcher nhìn thấy toàn bộ QUEUED tasks khi FRYER rảnh → tự gom.

#### ❌ Vấn đề 2: Không kiểm soát Staff Overload

```
Tình huống:
  Chỉ có 1 nhân viên Minh đang ca
  10 PO liên tiếp được tạo

Push model: assign tất cả 10 PO cho Minh, stack queue dài vô tận
```

**Giải pháp:** Staff chỉ nhận task khi Dispatcher thấy staff thực sự rảnh (freeAt ≤ now + threshold).

#### ⚠️ Vấn đề 3: Độ trễ đơn hàng (Order Delay) — Có nền nhưng thụ động

Thiếu cơ chế priority boost cho PO urgent.

**Giải pháp:** Dispatcher sort candidates theo EDD (Earliest Due Date) — Phase 2 khi PO có `due_time`.
MVP: FIFO theo `CreatedAt`.

---

### 8.2 — Kiến trúc Pull + Dispatcher: Quyết định cuối cùng

```
[PULL — Đã áp dụng]
PO tạo → SchedulePO() → Tạo tasks ở QUEUED pool (chưa assign)
                                      ↓
              dispatcher.Dispatch() → kéo QUEUED task phù hợp → assign machine + staff → PENDING
```

---

### 8.3 — Các quyết định cụ thể

| # | Câu hỏi | Quyết định | Lý do |
|---|---|---|---|
| **Q1** | Có chuyển Pull không? | ✅ Có — incremental | Không rewrite C.3, chỉ shrink SchedulePO() + thêm Dispatcher |
| **Q2** | `QUEUED` là state riêng hay `PENDING + AssignedTo=""`? | ✅ **State riêng** | Tránh bug khi `FindActiveByStaff()` bắt nhầm unassigned tasks |
| **Q3** | Dispatcher sync hay async? | ✅ **Sync — MVP** | Đơn giản, dễ test, không cần background worker. Gọi cuối mỗi `SchedulePO()`, `CompleteTask()`, `StartShift()` |
| **Q4** | MVP cần EDD không? | ❌ Defer Phase 2 | FIFO (`CreatedAt`) đủ dùng. EDD cần `PO.due_time` — chưa có trong model |

---

### 8.4 — TaskStatus lifecycle mới

```
QUEUED → PENDING (đã assign) → ACTIVE → DONE
  ↑                ↑
  Tạo ở đây        Dispatcher assign tại đây
  (SchedulePO)     (sync, sau khi QUEUED tasks được tạo)

CANCELLED ← từ QUEUED hoặc PENDING (Phase 2)
```

**Lưu ý:** `ScheduledStart/End` trên QUEUED task là **estimate** hiển thị cho UI.  
Dispatcher **recalculate** và ghi lại giá trị chính xác khi assign.

---

### 8.5 — Tác động lên các phần đã plan

```
Block A — Models:
  + Thêm TaskStatus.QUEUED (1 dòng)
  Không đổi gì khác.

Block B — Repos:
  + Thêm FindQueued(ctx, nodeID) vào StaffTaskRepository interface
  + Implement FindQueued trong MongoDB repo (index: node_id + status)
  Không đổi gì khác.

Block C.3 — SchedulingEngine:
  ~ SchedulePO(): bỏ pickStaff() + pickMachine(); tạo tasks ở QUEUED; gọi Dispatcher cuối hàm
  + Dispatcher: interface mới (~20 dòng) + usecase mới (~100 dòng)
  C.1, C.2, C.4, C.5: không đổi

Test cases T1–T23:
  ~ T1, T8, T9: expected task.Status = QUEUED sau SchedulePO(),
    sau đó = PENDING sau Dispatcher.Dispatch()
  + Thêm test cases T24–T26 cho Dispatcher (xem bên dưới)
  Không xoá test case nào.
```

---

### 8.6 — Test cases bổ sung cho Dispatcher

#### T24 — Dispatch: QUEUED Task + Staff Rảnh → Assign

```
Setup:
  QUEUED task T1: equipType=FRYER, nodeID=KITCHEN_01
  Staff Minh: station=FRYER, freeAt=T0
  FRYER_01: IDLE

Input: Dispatch("KITCHEN_01")

Expected:
  T1.AssignedTo  = "staff_minh"
  T1.MachineID   = "FRYER_01"
  T1.Status      = PENDING
  T1.ScheduledStart recalculated = T0
```

#### T25 — Dispatch: Không Có Staff → Task Giữ Nguyên QUEUED

```
Setup:
  QUEUED task T1: equipType=FRYER
  Không có active shift nào tại node

Input: Dispatch("KITCHEN_01")

Expected:
  T1.Status = QUEUED (không thay đổi)
  log: "no available staff for queued task <taskID>"
  Không return error
```

#### T26 — Dispatch: FIFO — Task Cũ Hơn Được Assign Trước

```
Setup:
  QUEUED task A: CreatedAt = T0 - 60s
  QUEUED task B: CreatedAt = T0 - 10s
  Cả 2 đều fit với staff Minh (FRYER, chỉ đủ sức làm 1 task ngay)

Input: Dispatch("KITCHEN_01")

Expected:
  Task A được assign (CreatedAt cũ hơn → FIFO)
  Task B vẫn QUEUED
```


