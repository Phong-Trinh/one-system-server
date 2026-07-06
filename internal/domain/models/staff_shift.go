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
//   - Ai đứng station nào (StationID → EquipmentType)
//   - Thời gian ca để tính scheduling horizon
//
// Quan hệ với StaffTask:
//   - Mỗi StaffTask.AssignedTo phải là StaffID của một shift đang ACTIVE
//   - Khi shift ENDED: scheduler tự unassign các PENDING tasks của nhân viên đó
type StaffShift struct {
	ID         string      `json:"id"`
	StaffID    string      `json:"staff_id"` // FK → Staff
	NodeID     string      `json:"node_id"`  // FK → Node (nơi làm việc)

	// StationID xác định nhân viên này đứng station nào trong ca.
	// Scheduler dùng để filter: chỉ assign step có equipment_type_id == StationID cho nhân viên này.
	// nil = nhân viên linh hoạt (có thể assign bất kỳ station nào).
	// TODO (Phase 2): Nếu nil, scheduler cần strategy khác (e.g. by skill set).
	StationID  *string     `json:"station_id,omitempty"` // FK → EquipmentType

	ShiftStart time.Time   `json:"shift_start"`
	ShiftEnd   *time.Time  `json:"shift_end,omitempty"`   // Giờ kết thúc dự kiến
	ActualEnd  *time.Time  `json:"actual_end,omitempty"`  // Giờ kết thúc thực tế (nếu end sớm)
	Status     ShiftStatus `json:"status"`

	CreatedAt time.Time `json:"created_at"`
}
