package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

// ErrStaffAlreadyOnShift — nhân viên đang trong ca khác còn ACTIVE.
var ErrStaffAlreadyOnShift = fmt.Errorf("staff already has an active shift")

// ErrNoActiveShift — không tìm thấy ca ACTIVE để kết thúc.
var ErrNoActiveShift = fmt.Errorf("no active shift found for staff")

// ─── Interface ────────────────────────────────────────────────────────────────

// StaffShiftUseCase quản lý vòng đời ca làm việc của nhân viên.
//
// MVP model: Nhân viên là Flexible Runner — StartShift không cần chỉ định station.
// Dispatcher tự pick task theo freeAt (người rảnh nhất trước).
type StaffShiftUseCase interface {
	// StartShift bắt đầu ca làm việc mới cho một nhân viên tại một node.
	//
	// Validation:
	//   - Nếu staff đã có ca ACTIVE: trả về ErrStaffAlreadyOnShift.
	//
	// Side effect:
	//   - Trigger RescheduleOnShiftChange để Dispatcher tự assign QUEUED tasks còn lại.
	StartShift(ctx context.Context, staffID, nodeID string) (*models.StaffShift, error)

	// EndShift kết thúc ca làm việc hiện tại của một nhân viên.
	//
	// Validation:
	//   - Nếu staff không có ca ACTIVE: trả về ErrNoActiveShift.
	//
	// Side effects:
	//   1. Đổi shift status → ENDED, set actual_end = now.
	//   2. Unassign tất cả PENDING tasks của nhân viên này → QUEUED (AssignedTo = "").
	//   3. Trigger RescheduleOnShiftChange để Dispatcher chia lại tasks cho staff còn lại.
	EndShift(ctx context.Context, staffID string) error

	// ListActiveShifts trả về tất cả ca đang ACTIVE tại một node.
	// Dùng cho manager overview.
	ListActiveShifts(ctx context.Context, nodeID string) ([]*models.StaffShift, error)
}

// ─── Implementation ───────────────────────────────────────────────────────────

type staffShiftUseCase struct {
	shiftRepo services.StaffShiftRepository
	taskRepo  services.StaffTaskRepository
	scheduler SchedulingEngine
	now       func() time.Time
}

// NewStaffShiftUseCase tạo một StaffShiftUseCase mới.
func NewStaffShiftUseCase(
	shiftRepo services.StaffShiftRepository,
	taskRepo services.StaffTaskRepository,
	scheduler SchedulingEngine,
) StaffShiftUseCase {
	return &staffShiftUseCase{
		shiftRepo: shiftRepo,
		taskRepo:  taskRepo,
		scheduler: scheduler,
		now:       time.Now,
	}
}

// ─── StartShift ───────────────────────────────────────────────────────────────

func (u *staffShiftUseCase) StartShift(ctx context.Context, staffID, nodeID string) (*models.StaffShift, error) {
	// [1] Validate: không có ca ACTIVE trùng
	existing, err := u.shiftRepo.FindActiveShiftByStaff(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("staffShiftUseCase.StartShift: check existing shift: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("staffShiftUseCase.StartShift (staffID=%s): %w", staffID, ErrStaffAlreadyOnShift)
	}

	// [2] Tạo shift mới (không có StationID — MVP Flexible Runner model)
	now := u.now()
	shift := &models.StaffShift{
		ID:         uuid.NewString(),
		StaffID:    staffID,
		NodeID:     nodeID,
		ShiftStart: now,
		Status:     models.ShiftActive,
		CreatedAt:  now,
	}
	if err := u.shiftRepo.Create(ctx, shift); err != nil {
		return nil, fmt.Errorf("staffShiftUseCase.StartShift: create shift: %w", err)
	}

	log.Printf("staffShiftUseCase.StartShift: staff=%s node=%s shift=%s", staffID, nodeID, shift.ID)

	// [3] Trigger rescheduling — Dispatcher sẽ pick QUEUED tasks và assign cho staff mới này
	if err := u.scheduler.RescheduleOnShiftChange(ctx, nodeID); err != nil {
		// Non-fatal: shift đã được tạo. Log và tiếp tục.
		log.Printf("staffShiftUseCase.StartShift: RescheduleOnShiftChange warning: %v", err)
	}

	return shift, nil
}

// ─── EndShift ─────────────────────────────────────────────────────────────────

func (u *staffShiftUseCase) EndShift(ctx context.Context, staffID string) error {
	// [1] Tìm ca ACTIVE của staff
	shift, err := u.shiftRepo.FindActiveShiftByStaff(ctx, staffID)
	if err != nil {
		return fmt.Errorf("staffShiftUseCase.EndShift: find active shift: %w", err)
	}
	if shift == nil {
		return fmt.Errorf("staffShiftUseCase.EndShift (staffID=%s): %w", staffID, ErrNoActiveShift)
	}

	nodeID := shift.NodeID
	now := u.now()

	// [2] ENDED shift
	if err := u.shiftRepo.UpdateStatus(ctx, shift.ID, models.ShiftEnded, &now); err != nil {
		return fmt.Errorf("staffShiftUseCase.EndShift: update shift status: %w", err)
	}

	log.Printf("staffShiftUseCase.EndShift: staff=%s shift=%s ended", staffID, shift.ID)

	// [3] Unassign tất cả PENDING tasks của nhân viên → QUEUED
	pendingTasks, err := u.taskRepo.FindByStaff(ctx, staffID, []models.TaskStatus{models.TaskPending})
	if err != nil {
		// Non-fatal: shift đã ENDED. Log và tiếp tục rescheduling.
		log.Printf("staffShiftUseCase.EndShift: load pending tasks warning: %v", err)
	}

	for _, task := range pendingTasks {
		task.AssignedTo = ""
		task.Status = models.TaskQueued
		if err := u.taskRepo.Update(ctx, task); err != nil {
			log.Printf("staffShiftUseCase.EndShift: unassign task %s warning: %v", task.ID, err)
		}
	}
	log.Printf("staffShiftUseCase.EndShift: unassigned %d PENDING tasks for staff=%s", len(pendingTasks), staffID)

	// [4] Trigger rescheduling — Dispatcher phân phối lại tasks cho staff còn lại
	if err := u.scheduler.RescheduleOnShiftChange(ctx, nodeID); err != nil {
		log.Printf("staffShiftUseCase.EndShift: RescheduleOnShiftChange warning: %v", err)
	}

	return nil
}

// ─── ListActiveShifts ─────────────────────────────────────────────────────────

func (u *staffShiftUseCase) ListActiveShifts(ctx context.Context, nodeID string) ([]*models.StaffShift, error) {
	shifts, err := u.shiftRepo.FindActiveByNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("staffShiftUseCase.ListActiveShifts: %w", err)
	}
	return shifts, nil
}
