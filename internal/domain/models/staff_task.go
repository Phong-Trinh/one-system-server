package models

import "time"

// ─── StaffTask ────────────────────────────────────────────────────────────────

// TaskStatus represents the lifecycle of a single staff task.
//
// State machine:
//
//	QUEUED  → PENDING     (Dispatcher assign machine + staff cho task)
//	PENDING → ACTIVE      (nhân viên bắt đầu thực hiện — bấm Start trên KDS)
//	ACTIVE  → WAITING     (idle step: nhân viên đã setup máy, máy đang tự chạy)
//	ACTIVE  → DONE        (bước không idle: nhân viên hoàn thành)
//	WAITING → DONE        (idle step: nhân viên lấy sản phẩm ra, bước hoàn thành)
//	PENDING → CANCELLED   (trước khi bắt đầu: máy hỏng, đơn bị hủy, v.v.)
//	QUEUED  → CANCELLED   (PO bị huỷ trước khi Dispatcher assign)
//	ACTIVE  → FAILED      (sau khi bắt đầu: thiết bị lỗi giữa chừng, v.v.)
//	WAITING → FAILED      (idem)
type TaskStatus string

const (
	// TaskQueued — task đã được tạo bởi SchedulingEngine nhưng chưa được Dispatcher assign.
	// ScheduledStart/End ở trạng thái này là estimate, không phải cam kết cứng.
	TaskQueued TaskStatus = "QUEUED"

	// TaskPending — đã được Dispatcher assign (có staff + machine), chờ nhân viên bắt đầu.
	TaskPending TaskStatus = "PENDING"

	// TaskActive — nhân viên đang thực hiện (thao tác trực tiếp).
	TaskActive TaskStatus = "ACTIVE"

	// TaskWaiting — idle step: máy đang tự chạy, nhân viên có thể rời.
	// Scheduler sẽ tìm fill-in task để chèn vào idle window này.
	TaskWaiting TaskStatus = "WAITING"

	// TaskDone — hoàn thành, nhân viên đã confirm.
	TaskDone TaskStatus = "DONE"

	// TaskCancelled — hủy TRƯỚC KHI bắt đầu.
	// TODO (Phase 2): Trigger inventory return nếu nguyên liệu đã được reserved.
	TaskCancelled TaskStatus = "CANCELLED"

	// TaskFailed — thất bại SAU KHI đã bắt đầu.
	// Nguyên liệu đã tiêu thụ → không hoàn trả.
	// TODO (Phase 2): Trigger machine breakdown flow, create replacement task.
	TaskFailed TaskStatus = "FAILED"
)

// TaskKind phân biệt vai trò của task trong kế hoạch sản xuất.
// Cần thiết vì một SOPStep (is_idle_step=true) được split thành 2 tasks
// cùng SOPStepID nhưng khác vai trò.
type TaskKind string

const (
	// TaskKindNormal — bước thông thường (is_idle_step=false), 1 StaffTask duy nhất.
	TaskKindNormal TaskKind = "NORMAL"

	// TaskKindSetup — phần setup của idle step: nhân viên thao tác trực tiếp để khởi động máy.
	// Duration = SOPStep.ActiveTime. Khi DONE → parent task chuyển WAITING.
	TaskKindSetup TaskKind = "SETUP"

	// TaskKindRetrieve — phần lấy sản phẩm ra của idle step.
	// ScheduledStart = SetupTask.ScheduledEnd. Duration = SOPStep.Duration - ActiveTime.
	// Đây là "parent" mà fill-in task sẽ trỏ ParentTaskID vào.
	TaskKindRetrieve TaskKind = "RETRIEVE"

	// TaskKindFillIn — task được chèn vào idle window của một RETRIEVE task.
	// ParentTaskID != nil. IsInterruptible = true (phải dừng khi parent cần attention).
	TaskKindFillIn TaskKind = "FILL_IN"
)

// StaffTask là đơn vị công việc nguyên tử giao cho 1 nhân viên cụ thể
// tại 1 máy cụ thể trong 1 khoảng thời gian cụ thể.
//
// Mỗi SOPStep của một PO được map thành 1 hoặc nhiều StaffTask:
//   - Bước thường (is_idle_step=false): 1 StaffTask
//   - Bước idle (is_idle_step=true): 2 StaffTask (SETUP + RETRIEVE) + 1 WAITING period
//
// Fill-in task (task chèn vào idle window):
//   - ParentTaskID != nil → đây là fill-in task
//   - IsInterruptible = true → được phép dừng giữa chừng khi parent WAITING kết thúc
type StaffTask struct {
	ID        string `json:"id"`
	POID      string `json:"po_id"`       // FK → ProductionOrder

	// OrderItemID FK → OrderItem (item-level tracking, Section 6.5 spec).
	// nil khi OrderItem model chưa được implement (Phase 2).
	// Khi Phase 2 thêm OrderItem: set field này khi Scheduler tạo task.
	OrderItemID *string `json:"order_item_id,omitempty"` // FK → OrderItem, nil = Phase 1

	SOPStepID string `json:"sop_step_id"` // FK → SOPStep (biết làm gì, cần máy gì, mất bao lâu)
	NodeID    string `json:"node_id"`     // FK → Node

	// TaskKind phân biệt vai trò của task trong SOP execution.
	// NORMAL: bước thông thường. SETUP/RETRIEVE: 2 halves của idle step.
	// FILL_IN: task chèn vào idle window (ParentTaskID != nil).
	TaskKind TaskKind `json:"task_kind"`

	// OriginalKind lưu TaskKind gốc khi task được tạo (SETUP hoặc NORMAL).
	// Chỉ có ý nghĩa khi TaskKind = FILL_IN — giúp tính đúng thời gian nhân viên
	// thực tế (FILL-IN từ SETUP chỉ mất ActiveTime, không phải toàn bộ Duration).
	OriginalKind TaskKind `json:"original_kind,omitempty"`

	// AssignedTo là staff_id của nhân viên được phân công.
	// "" = chưa được assign (task đang QUEUED, chờ Dispatcher).
	// Dispatcher set field này; nhân viên không tự chọn task.
	AssignedTo string `json:"assigned_to"` // FK → Staff, "" = unassigned

	// MachineID là máy cụ thể được assign cho task này.
	// "" = step không cần máy (manual step) hoặc task đang QUEUED.
	MachineID string `json:"machine_id"` // FK → Machine, "" = manual step

	TargetQty     float64 `json:"target_qty"`     // Lượng target cần làm của PO (để tính thời gian/capacity)
	RequiredSlots float64 `json:"required_slots"` // Dung lượng yêu cầu trên máy (TargetQty * SOPStep.SlotConsumption)

	Status   TaskStatus `json:"status"`
	Priority int        `json:"priority"` // Thứ tự trong queue (thấp = ưu tiên hơn)

	// IsInterruptible = true nghĩa là fill-in task này có thể bị dừng giữa chừng
	// khi parent task WAITING kết thúc (nhân viên phải quay lại máy).
	// Luôn = true với fill-in tasks. Luôn = false với task chính.
	IsInterruptible bool `json:"is_interruptible"`

	// IsCritical = true nghĩa là task này là prerequisite (trực tiếp hoặc gián tiếp)
	// cho một SETUP task. Cần ưu tiên xếp lịch sớm (Pass 1) để không làm delay máy.
	IsCritical bool `json:"is_critical"`

	// ParentTaskID != nil → đây là fill-in task được chèn vào idle window của parent.
	// Parent task phải đang ở trạng thái WAITING.
	// Scheduler set field này khi tìm được fill-in task phù hợp.
	ParentTaskID *string `json:"parent_task_id,omitempty"` // FK → StaffTask

	// BatchIndex xác định thứ tự mẻ máy khi 1 SOPStep cần nhiều mẻ liên tiếp
	// do TargetQty × SlotConsumption > machine.MaxCapacity.
	// BatchIndex=0 = mẻ đầu tiên. Mặc định = 0 (đủ 1 mẻ hoặc step không dùng máy).
	// SETUP[i] và RETRIEVE[i] có cùng BatchIndex.
	// SETUP[i+1].EarliestStart = RETRIEVE[i].ScheduledEnd.
	BatchIndex int `json:"batch_index"`

	// Scheduling timestamps (set bởi SchedulingEngine)
	// EarliestStart là thời điểm sớm nhất có thể bắt đầu (dựa trên Dependencies DAG).
	EarliestStart time.Time `json:"earliest_start"`
	// Khi Status=QUEUED: đây là estimate. Khi Status=PENDING: đây là kế hoạch chính xác (set bởi Dispatcher).
	ScheduledStart time.Time `json:"scheduled_start"`
	ScheduledEnd   time.Time `json:"scheduled_end"`

	// Execution timestamps (set bởi StaffTaskUseCase khi nhân viên thao tác)
	StartedAt   *time.Time `json:"started_at,omitempty"`   // Khi nhân viên bấm Start
	CompletedAt *time.Time `json:"completed_at,omitempty"` // Khi nhân viên bấm Done

	CreatedAt time.Time `json:"created_at"`
}
