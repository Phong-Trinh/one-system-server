# KDS Redesign — MVP: Auto-Scheduling + Idle Fill-In

## Mục tiêu

Hai tính năng cốt lõi phải hoạt động trước khi làm bất kỳ thứ gì khác:

> **1. Scheduling Engine:** Khi PO → IN_PROGRESS, hệ thống tự tạo và phân công `StaffTask` cho từng nhân viên đang có mặt — không cần manager bấm thủ công.

> **2. Idle Time Fill-In:** Khi nhân viên đang chờ máy chạy, hệ thống chèn task phụ phù hợp — và alert đúng lúc để nhân viên quay lại đúng thời điểm.

Defer: Manager view, Shift Handover, Analytics, Assembly tracking.

---

## Nguyên tắc cắt bỏ

| Giữ lại (MVP) | Defer |
|---|---|
| SOPStep V2: `is_idle_step`, `attention_level`, `active_time`, `requires_attention_at` | SOPStep V2: `equipment_profile`, `max_distance_meters` |
| `StaffTask` model đầy đủ | `OrderItem` per-item tracking |
| `StaffShift` (chỉ để scheduler biết ai available) | `ShiftHandover` protocol |
| Scheduling Engine — Phase 1, 2, 3 (DAG + assign + idle fill) | Scheduling Engine — Phase 2.5 (workload balance) |
| Staff KDS screen — 4 states (ACTIVE, WAITING, alert, done) | Manager KDS realtime view |
| Integration test — Happy Path + Idle Fill-In | Failure & Recovery, Reassignment test |

> [!IMPORTANT]
> `ProductionBatch` flow hiện tại **KHÔNG bị động**. Scheduler mới chạy song song — chỉ thêm, không sửa existing code. `ProductionBatch` vẫn là cơ chế chạy máy; `StaffTask` là lớp bổ sung để guide người.

---

## Critical Path

```
A (Models) → B (Repos) → C (Scheduling Engine ← CORE) → D (Staff KDS UI) → E (Test)
```

---

## Block A — Domain Models ✅ DONE

**File chạm:** `internal/domain/models/`

### A.1 — Mở rộng `SOPStep` (V2 Idle Fields Only) ✅

**File:** [`production.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/domain/models/production.go)

Đã implement:

```go
// AttentionLevel mô tả mức độ chú ý cần thiết trong idle time.
// Scheduler dùng field này để quyết định fill-in task nào có thể chèn vào.
type AttentionLevel string

const (
    AttentionFullIdle      AttentionLevel = "FULL_IDLE"      // Tự do hoàn toàn
    AttentionNearbyIdle    AttentionLevel = "NEARBY_IDLE"    // Ở gần máy
    AttentionPeriodicCheck AttentionLevel = "PERIODIC_CHECK" // Check định kỳ
    AttentionActiveWait    AttentionLevel = "ACTIVE_WAIT"    // Không được rời
)

// Thêm vào SOPStep struct:
IsIdleStep           bool           `json:"is_idle_step"`
ActiveTime           *int           `json:"active_time,omitempty"`        // giây setup trực tiếp
AttentionLevel       AttentionLevel `json:"attention_level,omitempty"`    // BẮT BUỘC nếu is_idle_step
CheckIntervalSec     *int           `json:"check_interval_sec,omitempty"` // chỉ khi PERIODIC_CHECK
RequiresAttentionAt  *int           `json:"requires_attention_at,omitempty"` // giây cần quay lại trước khi xong
```

> [!NOTE]
> **Defer:** `MaxDistanceMeters`, `EquipmentProfile`, `SameItemRequired` — không cần cho scheduling core.

### A.2 — `StaffShift` model ✅

**File mới:** [`staff_shift.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/domain/models/staff_shift.go)

Đã tạo — bao gồm `ActualEnd` để track khi ca kết thúc sớm:

```go
type ShiftStatus string
const (
    ShiftActive ShiftStatus = "ACTIVE"
    ShiftEnded  ShiftStatus = "ENDED"
)

type StaffShift struct {
    ID         string      `json:"id"`
    StaffID    string      `json:"staff_id"`
    NodeID     string      `json:"node_id"`
    StationID  *string     `json:"station_id,omitempty"` // FK → EquipmentType
    ShiftStart time.Time   `json:"shift_start"`
    ShiftEnd   *time.Time  `json:"shift_end,omitempty"`
    Status     ShiftStatus `json:"status"`
}
```

> [!NOTE]
> **Defer:** `ShiftHandover`, `BreakPeriods`, `SCHEDULED` status.

### A.3 — `StaffTask` model ✅

**File mới:** [`staff_task.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/domain/models/staff_task.go)

Đã tạo. Fields đầy đủ cho MVP + FK placeholder cho Phase 2:

```go
type StaffTask struct {
    ID           string   `json:"id"`
    POID         string   `json:"po_id"`           // FK → ProductionOrder
    OrderItemID  *string  `json:"order_item_id"`   // FK → OrderItem (nil = Phase 1, Section 6.5)
    SOPStepID    string   `json:"sop_step_id"`
    AssignedTo   string   `json:"assigned_to"`     // "" = unassigned
    MachineID    string   `json:"machine_id"`
    NodeID       string   `json:"node_id"`
    Status       TaskStatus
    Priority     int
    IsInterruptible bool  // fill-in task có thể bị ngắt
    ParentTaskID *string  // FK → StaffTask (fill-in trong idle time)
    ScheduledStart, ScheduledEnd time.Time
    StartedAt, CompletedAt *time.Time
    CreatedAt time.Time
}
```

> **Gap đã xử lý:** `OrderItemID *string` đã được thêm (Section 6.5 spec). Giá trị `nil` trong Phase 1 — sẽ được populate khi Phase 2 thêm `OrderItem` model.

> **Còn defer (Phase 2):** `TaskNote`, `CancelReason`, `FailedReason` — xem Defer Tracker bên dưới.

### A.4 — Cập nhật `sopStepDoc` BSON ✅

**File:** [`sop_repository.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/infrastructure/persistence/mongodb/sop_repository.go)

Đã thêm 5 V2 fields vào `sopStepDoc` struct và cập nhật `sopStepToDoc()` + `docToSOPStep()`.

> **Verify:** `go build ./internal/domain/models/... ./internal/infrastructure/persistence/mongodb/...` — ✅ OK

---

## Block B — Repository Layer ✅ DONE

**File chạm:** `internal/domain/services/`, `internal/infrastructure/persistence/mongodb/`

### B.1 — `StaffShiftRepository` interface ✅

**File:** [`production_repository.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/domain/services/production_repository.go) (append)

Đã thêm. `UpdateStatus` nhận `actualEnd *time.Time` để handle cả trường hợp kết thúc ca sớm.
```go
type StaffShiftRepository interface {
    Create(ctx context.Context, s *models.StaffShift) error
    FindByID(ctx context.Context, id string) (*models.StaffShift, error)
    // Scheduler dùng: tìm staff đang on shift tại node, có station phù hợp
    FindActiveByNode(ctx context.Context, nodeID string) ([]*models.StaffShift, error)
    // Scheduler dùng: tìm task timeline của staff để tính free-at
    FindByStaff(ctx context.Context, staffID string) ([]*models.StaffShift, error)
    UpdateStatus(ctx context.Context, id string, status models.ShiftStatus) error
}
```

### B.2 — `StaffTaskRepository` interface ✅

**File:** [`production_repository.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/domain/services/production_repository.go) (append)

Đã thêm. 7 methods đủ để serve 3 access patterns: SchedulingEngine, Staff KDS polling, fill-in lookup.
```go
type StaffTaskRepository interface {
    Create(ctx context.Context, t *models.StaffTask) error
    FindByID(ctx context.Context, id string) (*models.StaffTask, error)
    FindByPO(ctx context.Context, poID string) ([]*models.StaffTask, error)
    // Scheduler dùng: xem toàn bộ tasks đã schedule của 1 staff → tính free-at
    FindByStaff(ctx context.Context, staffID string, statuses []models.TaskStatus) ([]*models.StaffTask, error)
    FindByNode(ctx context.Context, nodeID string, statuses []models.TaskStatus) ([]*models.StaffTask, error)
    FindActiveByStaff(ctx context.Context, staffID string) (*models.StaffTask, error)
    // Fill-in task cần tìm parent idle tasks đang WAITING
    FindWaitingByStaff(ctx context.Context, staffID string) ([]*models.StaffTask, error)
    Update(ctx context.Context, t *models.StaffTask) error
}
```

### B.3 — MongoDB implementations ✅

**File mới:** [`staff_shift_repository.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/infrastructure/persistence/mongodb/staff_shift_repository.go)  
Indexes: `staff_id`, compound `(node_id, status)` cho SchedulingEngine hot query

**File mới:** [`staff_task_repository.go`](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/internal/infrastructure/persistence/mongodb/staff_task_repository.go)  
Indexes: `po_id`, compound `(assigned_to, status)`, compound `(node_id, status)`, `parent_task_id`

> **Verify:** `go build ./internal/domain/services/... ./internal/infrastructure/persistence/mongodb/...` — ✅ OK

---

## Block C — Scheduling Engine ← CORE

**File chạm:** `internal/usecase/`

> [!IMPORTANT]
> Đây là trái tim của toàn bộ feature. Phần còn lại (UI, tests) phụ thuộc vào đây.

### C.1 — `StaffShiftUseCase` (minimal)

**File mới:** `internal/usecase/staff_shift_usecase.go`

Scheduler cần 3 methods:

```go
type StaffShiftUseCase interface {
    StartShift(ctx, staffID, nodeID string, stationID *string) (*models.StaffShift, error)
    EndShift(ctx, shiftID string) error
    ListActiveShifts(ctx, nodeID string) ([]*models.StaffShift, error)
}
```

### C.2 — `StaffTaskUseCase` (staff actions)

**File mới:** `internal/usecase/staff_task_usecase.go`

Staff-facing actions — những gì nhân viên bấm trên KDS:

```go
type StaffTaskUseCase interface {
    StartTask(ctx, taskID string) error    // PENDING → ACTIVE
    CompleteTask(ctx, taskID string) error // ACTIVE/WAITING → DONE
    // Khi DONE: tự activate fill-in task nếu có, hoặc task kế tiếp trong queue

    GetCurrentTask(ctx, staffID string) (*models.StaffTask, error)
    GetTaskQueue(ctx, staffID string) ([]*models.StaffTask, error)
}
```

### C.3 — `SchedulingEngine` ← **Trọng điểm**

**File mới:** `internal/usecase/scheduling_engine.go`

Engine nhận 1 PO, trả về danh sách `StaffTask` đã được assign và schedule.

#### Interface

```go
type SchedulingEngine interface {
    // SchedulePO là entry point chính:
    // Được gọi khi PO → IN_PROGRESS, hoặc khi StaffShift mới bắt đầu
    SchedulePO(ctx context.Context, poID string) ([]*models.StaffTask, error)

    // RescheduleOnShiftChange: khi có staff mới vào ca hoặc staff rời ca
    RescheduleOnShiftChange(ctx context.Context, nodeID string) error
}
```

#### Thuật toán — Phase 1: Build Dependency DAG

```
Input: PO.SOPID → SOPStep[]

1. Load tất cả SOPStep của SOP
2. Build directed graph: step A → step B nếu B.DependsOn chứa A.ID
3. Topological sort (Kahn's algorithm):
   - Start với steps không có dependency (DependsOn = [])
   - Lần lượt unlock steps khi tất cả dependency đã schedule
4. Output: []SOPStep theo thứ tự an toàn (parallelizable groups)
```

#### Thuật toán — Phase 2: Assign Machine + Staff + Time

```
Với mỗi step trong topo order:

2.1. Tìm Machine:
   - FindIdleByStationType(nodeID, step.EquipmentTypeID)
   - Nếu is_idle_step = false: cần machine hoàn toàn free
   - Tính machine_free_at: max(machine.currentBatch.EstimatedCompletion)
   - Nếu không có machine available: task.Status = PENDING (chờ)

2.2. Tìm Staff:
   - ListActiveShifts(nodeID) → filter by shift.StationID == step.EquipmentTypeID
   - Tính staff_free_at mỗi người: thời điểm task cuối cùng của họ kết thúc
   - Pick staff có staff_free_at sớm nhất (FIFO đơn giản)
   - Không có staff available → task.Status = PENDING, ghi log

2.3. Tính scheduled_start:
   - dep_done_at = max(scheduled_end của tất cả dependency steps)
   - scheduled_start = max(dep_done_at, machine_free_at, staff_free_at)
   - scheduled_end = scheduled_start + step.Duration

2.4. Tạo StaffTask:
   status = PENDING
   machine_id = machine.ID
   assigned_to = staff.ID
   scheduled_start, scheduled_end
```

#### Thuật toán — Phase 3: Idle Time Fill-In

```
Với mỗi step có is_idle_step = true:

3.1. Tách thành 2 tasks:
   - SETUP task (ActiveTime giây): ACTIVE → nhân viên đặt đồ vào máy
   - RETRIEVE task (khi timer xong): nhân viên lấy ra
   - Khoảng giữa: idle window = Duration - ActiveTime - RequiresAttentionAt

3.2. Xác định idle window của staff này:
   - idle_start = SETUP_task.scheduled_end
   - idle_end = scheduled_end - RequiresAttentionAt (thời điểm phải quay lại)
   - available_window = idle_end - idle_start

3.3. Tìm fill-in task dựa theo AttentionLevel:
   - FULL_IDLE: tìm bất kỳ PENDING task nào có:
       * duration < available_window - safety_buffer (30s)
       * assigned_to == "" hoặc assigned_to == cùng staff
   - NEARBY_IDLE: tìm task cùng station (machine_id thuộc cùng equipment_type)
       * duration < available_window - safety_buffer
   - PERIODIC_CHECK: tìm task có:
       * duration < check_interval_sec - safety_buffer
       * is_interruptible = true (quan trọng: fill-in task này phải có thể bị dừng)
   - ACTIVE_WAIT: không fill-in. Log idle_duration cho analytics.

3.4. Nếu tìm được fill-in task:
   - fill_task.assigned_to = staff.ID
   - fill_task.parent_task_id = WAITING_task.ID
   - fill_task.is_interruptible = true
   - fill_task.scheduled_start = idle_start
   - fill_task.scheduled_end = idle_start + fill_task.duration

3.5. Tính alert thresholds (lưu vào task metadata hoặc tính client-side):
   - pre_alert_at = scheduled_end - RequiresAttentionAt - 120s (State 3a: còn 2:00)
   - prep_at     = scheduled_end - RequiresAttentionAt - 45s  (State 3b: còn 0:45)
   - retrieve_at = scheduled_end - RequiresAttentionAt        (State 3c: lấy ra ngay)
```

### C.4 — Tích hợp vào `AllocationUseCase`

**File:** `internal/usecase/allocation_engine.go`

Thêm vào `ConfirmCompletion`: sau khi batch DONE, call `schedulingEngine.SchedulePO()` nếu có dependency steps mới được unlock.

```go
// Sau khi allSOPCompleted check:
if !allSOPCompleted && uc.schedulingEngine != nil {
    _ = uc.schedulingEngine.RescheduleOnShiftChange(ctx, po.NodeID)
}
```

> [!NOTE]
> **Không sửa logic batch hiện tại.** Chỉ thêm scheduling engine call **sau** khi batch hoàn thành.

### C.5 — Trigger `SchedulePO` khi PO → IN_PROGRESS

**File:** `internal/usecase/order_orchestrator.go` hoặc production usecase

Khi PO chuyển sang `IN_PROGRESS`, call `schedulingEngine.SchedulePO(ctx, po.ID)`.

---

## Block D — Staff KDS UI

**File mới:** `web/admin-ui/js/features/factory/staff_kds.js`

### D.1 — 4 màn hình states (theo spec KDS Redesign doc)

**State 1 — ACTIVE (thao tác trực tiếp):**
```
🔴 NGAY BÂY GIỜ
Đặt 5kg khoai vào FRYER_01
[hướng dẫn step-by-step]
⏱ Máy sẽ chạy: 8 phút
[ ✓ ĐÃ ĐẶT VÀO ]
```

**State 2 — WAITING (idle, có fill-in task):**
```
🟡 TRONG KHI CHỜ
⏳ Fryer đang chạy • 7:23 còn lại  ████████░░░
[fill-in task: Pha trà sữa cho đơn #02]
[ ✓ XONG ]
```

**State 3 — Alert sequence (khi `requires_attention_at` đến gần):**
- **3a (T-2:00):** Banner vàng "Chuẩn bị: Lấy khoai ra khỏi FRYER_01"
- **3b (T-0:45):** Banner cam "Kết thúc việc hiện tại và di chuyển về Fryer"
- **3c (T-0:00):** Banner đỏ + nút "ĐÃ LẤY RA"

**State 4 — IDLE (không có task):**
```
✅ Rảnh — chờ nhiệm vụ mới
Không có gì để làm ngay lúc này.
```

### D.2 — State machine logic

```js
// Polling mỗi 3 giây
async function refresh() {
    const task = await api.getCurrentTask(staffId)
    if (!task) { renderIdle(); return }

    if (task.status === 'ACTIVE') {
        renderActive(task)
    } else if (task.status === 'WAITING') {
        const elapsed = (Date.now() - task.started_at) / 1000
        const remaining = task.duration - elapsed
        const parentTask = task // WAITING task có duration từ parent SOPStep

        // Check alert thresholds
        const fillInTask = await api.getFillInTask(staffId, task.id)
        if (fillInTask && fillInTask.status === 'ACTIVE') {
            renderWaitingWithFillIn(parentTask, fillInTask, remaining)
        } else {
            renderWaiting(parentTask, remaining)
        }
        checkAlertThresholds(parentTask, remaining)
    }
}

function checkAlertThresholds(task, remaining) {
    const rat = task.requires_attention_at // giây cần quay lại trước
    if (remaining <= rat)          renderAlert3c(task)
    else if (remaining <= rat + 45)  renderAlert3b(task)
    else if (remaining <= rat + 120) renderAlert3a(task)
}
```

### D.3 — Staff Login + Shift Start

Form đơn giản khi mở Staff KDS:
- Dropdown Staff (load từ `GET /api/staff?node_id=`)
- Dropdown Station (load từ `GET /api/equipment-types`)
- Nút "Bắt đầu ca" → `POST /api/shifts/start`
- Lưu `staff_id`, `shift_id` vào `localStorage`

### D.4 — API wiring trong `api.js`

```js
getCurrentTask(staffId)       → GET /api/kds/v2/staff/:id/current-task
getFillInTask(staffId, parentId) → GET /api/kds/v2/staff/:id/fill-in-task
startTask(taskId)             → POST /api/kds/v2/tasks/:id/start
completeTask(taskId)          → POST /api/kds/v2/tasks/:id/done
startShift(staffId, nodeId, stationId) → POST /api/shifts/start
```

### D.5 — API Endpoints

**File mới:** `internal/transport/http/staff_kds_handler.go`

```
GET  /api/kds/v2/staff/:id/current-task   → GetCurrentTask
GET  /api/kds/v2/staff/:id/fill-in-task   → GetFillInTask (lấy fill-in của WAITING task)
POST /api/kds/v2/tasks/:id/start           → StartTask
POST /api/kds/v2/tasks/:id/done            → CompleteTask
```

**File mới:** `internal/transport/http/staff_shift_handler.go`

```
POST /api/shifts/start         → StartShift
POST /api/shifts/:id/end       → EndShift (trigger RescheduleOnShiftChange)
GET  /api/shifts?node_id=      → ListActiveShifts
```

> [!NOTE]
> **Defer:** `GET /api/kds/v2/manager`, `POST /api/kds/v2/tasks/:id/reassign`, `POST /api/kds/v2/tasks/:id/fail`.

---

## Block E — Integration Test

**File mới:** `test/integration/staff_task_flow_test.go`

### E.1 — Test Scenario 1: Happy Path + Auto-Scheduling

```
Step 1.1 — Setup: Kitchen node, Fryer type, Fryer machine, Staff Minh (Fryer station)
Step 1.2 — SOP: "Chiên khoai" — 1 step, Fryer, duration=600s, NOT idle step
Step 1.3 — Staff Minh bắt đầu ca → StartShift(staffID, nodeID, stationID="ST_MAY_CHIEN")
Step 1.4 — PO tạo và chuyển IN_PROGRESS → SchedulePO(poID)
           Assert: 1 StaffTask được tạo tự động
           Assert: task.AssignedTo = "staff_minh"
           Assert: task.MachineID = "fryer_01"
           Assert: task.Status = PENDING
Step 1.5 — Minh nhận task → StartTask(taskID)
           Assert: task.Status = ACTIVE
Step 1.6 — Minh bấm DONE → CompleteTask(taskID)
           Assert: task.Status = DONE
           Assert: PO.Status = COMPLETED
```

### E.2 — Test Scenario 2: Idle Fill-In

```
Step 2.1 — SOP: "Chiên khoai" — is_idle_step=true, Duration=600s, ActiveTime=60s,
           AttentionLevel=FULL_IDLE, RequiresAttentionAt=120s
           Có thêm 1 PENDING task "Pha trà sữa" chưa assign
Step 2.2 — SchedulePO → tạo 2 tasks: SETUP + RETRIEVE, và fill-in task cho Minh
           Assert: setup_task.scheduled_end = scheduled_start + 60s
           Assert: fill_task.ParentTaskID = WAITING_task.ID
           Assert: fill_task.IsInterruptible = true
           Assert: fill_task.scheduled_end <= retrieve_task.scheduled_start - safety_buffer
Step 2.3 — Minh làm SETUP task → DONE
           Assert: parent task → WAITING (máy đang chạy)
           Assert: fill_task tự động → ACTIVE
Step 2.4 — Alert check:
           Assert: requires_attention_at trigger đúng = retrieve_task.scheduled_start
Step 2.5 — Minh DONE fill-in → complete
Step 2.6 — Timer hết, Minh lấy sản phẩm ra → DONE retrieve task
           Assert: PO → COMPLETED
```

### E.3 — Mock repos cần thêm vào `mocks.go`

- `mockStaffShiftRepo`
- `mockStaffTaskRepo`
- Mở rộng `testContext` với 2 repos mới

---

## Thứ tự thực hiện

```
A.1 (SOPStep V2 fields)
    → A.4 (cập nhật BSON repo)
        → A.2 (StaffShift model)
            → A.3 (StaffTask model)
                → B.1 + B.2 (interfaces)
                    → B.3 (MongoDB impls)
                        → C.1 (ShiftUseCase)
                            → C.2 (TaskUseCase)
                                → C.3 (SchedulingEngine ← CORE, dành nhiều thời gian nhất)
                                    → C.4 + C.5 (Integration với existing flow)
                                        → Block D (UI)
                                            → Block E (Tests)
```

**Ước tính thời gian:**
- Block A (Models): ~3h
- Block B (Repos): ~4h
- Block C (Scheduling Engine): ~10h ← phần phức tạp nhất
- Block D (Staff KDS UI): ~6h
- Block E (Tests): ~4h
- **Tổng: ~27h**

---

## Features bị defer — Tracker

> Phân loại theo phase thực hiện, dựa trên spec gốc [KDS Redesign.md](file:///c:/Users/ADMIN/Documents/OneSystem/one-system-server/business_documents/development/KDS%20Redesign.md).
> Mỗi mục có reference đến section của spec để dễ tra cứu khi implement.

### Phase 2 — Exception Flows + StaffTask đầy đủ

*Khi happy path (Block A–E) đã stable, add các flows này:*

**`StaffTask` model (Section 6.1):**
- [ ] `TaskNote` struct: `{ type NoteType, severity Severity, message string? }`
- [ ] `NoteType` enum: `DEVIATION | QUALITY_ISSUE | EQUIPMENT_ISSUE | OTHER`
- [ ] `Severity` enum: `LOW | MEDIUM | HIGH`
- [ ] Fields trên `StaffTask`: `completion_note`, `cancel_note`, `failure_note TaskNote?`
- [ ] `CancelReason` enum: `MACHINE_UNAVAILABLE | MATERIAL_UNAVAILABLE | ORDER_CANCELLED | OTHER`
- [ ] `FailedReason` enum: `QUALITY_ISSUE | EQUIPMENT_MALFUNCTION | OTHER`
- [ ] Inventory return khi PENDING → CANCELLED (auto-approved, không cần manager confirm)
- [ ] `FailTask(taskID, reason)` use case + API endpoint `POST /api/kds/v2/tasks/:id/fail`
- [ ] `CancelTask(taskID, reason)` use case + inventory return logic

**`StaffShift` (Section 6.2):**
- [ ] `ShiftStatus.SCHEDULED` — để plan ahead trước giờ làm
- [ ] Auto-unassign PENDING tasks khi `EndShift()` sớm (Section 8.3)
- [ ] Scheduler re-assign cho staff khác khi shift end đột xuất

**Scheduling Engine (Section 7.1):**
- [ ] Phase 2.5: Workload Balance Check (soft constraint — tránh assign quá nhiều cho 1 người)
- [ ] Machine conflict — fallback khi không có machine available (Section 8.2)
- [ ] `EDF` priority mode (Earliest Deadline First) — hiện chỉ FIFO (Section 8.2)
- [ ] `ReassignTask(taskID, newStaffID)` + API `POST /api/kds/v2/tasks/:id/reassign`

---

### Phase 3 — OrderItem + Assembly Flow

*Sau khi Phase 2 stable:*

**`OrderItem` model (Section 6.4):**
- [ ] `OrderItem` struct: `{ id, po_id, product_id, sop_id, quantity, status, ready_at, estimated_ready }`
- [ ] `ItemStatus` enum: `PENDING | IN_PROGRESS | READY | ASSEMBLED`
- [ ] `OrderItemRepository` interface + MongoDB impl
- [ ] Khi Phase 3 done: populate `StaffTask.OrderItemID` khi Scheduler tạo task

**Assembly Flow (Section 7.1 Phase 4):**
- [ ] `AssemblyTask` struct: `{ po_id, trigger_condition, ready_items, pending_items }`
- [ ] `trigger_condition`: `ALL_ITEMS_READY | PARTIAL_NOTIFY`
- [ ] Subscribe vào `OrderItem.status → READY` event
- [ ] Assembly endpoint + notification

**Relations (Section 6.5):**
- [ ] `StaffTask.OrderItemID` hiện là `nil` — set giá trị khi `OrderItem` model đã tồn tại

---

### Phase 4 — SOPStep V2 đầy đủ

*Sau Phase 3, khi cần hỗ trợ bếp phức tạp hơn (pizza truyền thống, premium burger):*

**`SOPStep` extended fields (Section 6.3):**
- [ ] `max_distance_meters float?` — dùng với `AttentionLevel = NEARBY_IDLE`
- [ ] `same_item_required bool` — không bin-pack với đơn/item khác
- [ ] `equipment_profile object?` — `{ temperature_celsius float?, mode string? }` để match machine
- [ ] Cập nhật `NEARBY_IDLE` fill-in logic trong Scheduler khi có `max_distance_meters`
- [ ] Cập nhật bin-packing khi có `same_item_required`

---

### Phase 5 — Machine Resilience

*Xử lý thiết bị hỏng giữa chừng:*

**Machine breakdown flow (Section 8.4):**
- [ ] Khi `Machine.status → UNDER_MAINTENANCE`:
  - Tasks đang ACTIVE/WAITING → `FAILED (reason: EQUIPMENT_MALFUNCTION)`
  - Tasks đang PENDING → `CANCELLED (reason: MACHINE_UNAVAILABLE)` + inventory return
- [ ] Scheduler tìm Machine backup cùng `equipment_type` còn IDLE
- [ ] Tạo `StaffTask` mới để re-route (chỉ cho FAILED tasks)
- [ ] Nếu không có backup: PO → PENDING, notify Manager

---

### Phase 6 — Manager KDS + Analytics

*Manager view và dữ liệu phân tích:*

**Manager KDS (Section 7.3):**
- [ ] `GET /api/kds/v2/manager` — overview: staff + machine + order progress
- [ ] Real-time status: Staff ACTIVE/IDLE, Machine BUSY/IDLE/remaining_time, PO % completion

**Analytics (Section 9 Phase 4):**
- [ ] OEE per machine: `Availability × Performance × Quality`
- [ ] Staff performance: `tasks done on time / total tasks per shift`
- [ ] Bottleneck detection: station nào thường xuyên là điểm nghữn
- [ ] Historical SOP timing: so sánh `SOPStep.duration` (estimate) vs actual → gợi ý điều chỉnh

---

### Phase 7 — Shift Handover Protocol

**`ShiftHandover` model (Section 6.6):**
- [ ] `ShiftHandover` struct: `{ id, from_shift_id, to_shift_id, handover_time, in_progress_tasks, machines_in_use, pending_notes, acknowledged_at }`
- [ ] `ShiftHandoverRepository` + MongoDB impl
- [ ] `StaffShiftHandler`: endpoint `POST /api/shifts/:id/handover`
- [ ] Staff KDS: Handover Screen (hiển thị khi ca mới bắt đầu)
  - Hiển thị: máy đang chạy + remaining time, ghi chú từ ca cũ
  - Nút confirm: *"Tôi đã kiểm tra và tiếp nhận"
