package models

import "time"

// ─── StaffShift ───────────────────────────────────────────────────────────────

// ShiftStatus represents the lifecycle of a single staff working shift.
type ShiftStatus string

const (
	// ShiftActive — nhân viên đang trong ca, sẵn sàng nhận task.
	ShiftActive ShiftStatus = "ACTIVE"

	// ShiftEnded — ca đã kết thúc (tự nhiên hoặc sớm).
	// Tất cả PENDING tasks của nhân viên này sẽ bị unassign.
	ShiftEnded ShiftStatus = "ENDED"

	// TODO (Phase 2): Thêm ShiftScheduled = "SCHEDULED" khi implement shift scheduling trước giờ làm.
)

// StaffShift ghi nhận ca làm việc của một nhân viên tại một node.
// Scheduler dùng model này để biết:
//   - Ai đang available (Status = ACTIVE)
//   - Thời gian ca để tính scheduling horizon
//
// Quan hệ với StaffTask:
//   - Mỗi StaffTask.AssignedTo phải là StaffID của một shift đang ACTIVE
//   - Khi shift ENDED: scheduler tự unassign các PENDING tasks của nhân viên đó
//
// MVP note: Staff là Flexible Runner — không gán cứng vào trạm cụ thể.
// Dispatcher sẽ assign task căn cứ vào availability (freeAt), không phải station.
type StaffShift struct {
	ID         string      `json:"id"`
	StaffID    string      `json:"staff_id"` // FK → Staff
	NodeID     string      `json:"node_id"`  // FK → Node (nơi làm việc)

	ShiftStart time.Time   `json:"shift_start"`
	ShiftEnd   *time.Time  `json:"shift_end,omitempty"`   // Giờ kết thúc dự kiến
	ActualEnd  *time.Time  `json:"actual_end,omitempty"`  // Giờ kết thúc thực tế (nếu end sớm)
	Status     ShiftStatus `json:"status"`

	CreatedAt time.Time `json:"created_at"`
}
