package models

import "time"

// ─── StaffTask ────────────────────────────────────────────────────────────────

// TaskStatus represents the lifecycle of a single staff task.
//
// State machine:
//
//	PENDING → ACTIVE      (nhân viên bắt đầu thực hiện — bấm Start trên KDS)
//	ACTIVE  → WAITING     (idle step: nhân viên đã setup máy, máy đang tự chạy)
//	ACTIVE  → DONE        (bước không idle: nhân viên hoàn thành)
//	WAITING → DONE        (idle step: nhân viên lấy sản phẩm ra, bước hoàn thành)
//	PENDING → CANCELLED   (trước khi bắt đầu: máy hỏng, đơn bị hủy, v.v.)
//	ACTIVE  → FAILED      (sau khi bắt đầu: thiết bị lỗi giữa chừng, v.v.)
//	WAITING → FAILED      (idem)
type TaskStatus string

const (
	// TaskPending — đã được schedule, chưa đến lượt thực hiện.
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

	// AssignedTo là staff_id của nhân viên được phân công.
	// "" = chưa được assign (ví dụ: không có staff available khi schedule).
	// Scheduler set field này; nhân viên không tự chọn task.
	AssignedTo string `json:"assigned_to"` // FK → Staff, "" = unassigned

	// MachineID là máy cụ thể được assign cho task này.
	// "" = step không cần máy (manual step).
	MachineID string `json:"machine_id"` // FK → Machine, "" = manual step

	Status   TaskStatus `json:"status"`
	Priority int        `json:"priority"` // Thứ tự trong queue (thấp = ưu tiên hơn)

	// IsInterruptible = true nghĩa là fill-in task này có thể bị dừng giữa chừng
	// khi parent task WAITING kết thúc (nhân viên phải quay lại máy).
	// Luôn = true với fill-in tasks. Luôn = false với task chính.
	IsInterruptible bool `json:"is_interruptible"`

	// ParentTaskID != nil → đây là fill-in task được chèn vào idle window của parent.
	// Parent task phải đang ở trạng thái WAITING.
	// Scheduler set field này khi tìm được fill-in task phù hợp.
	ParentTaskID *string `json:"parent_task_id,omitempty"` // FK → StaffTask

	// Scheduling timestamps (set bởi SchedulingEngine)
	ScheduledStart time.Time `json:"scheduled_start"` // Kế hoạch bắt đầu
	ScheduledEnd   time.Time `json:"scheduled_end"`   // Kế hoạch kết thúc

	// Execution timestamps (set bởi StaffTaskUseCase khi nhân viên thao tác)
	StartedAt   *time.Time `json:"started_at,omitempty"`   // Khi nhân viên bấm Start
	CompletedAt *time.Time `json:"completed_at,omitempty"` // Khi nhân viên bấm Done

	CreatedAt time.Time `json:"created_at"`
}
