package services

import (
	"context"
	"time"

	"one-system-server/internal/domain/models"
)

// ── BOM ───────────────────────────────────────────────────────────────────────

// BOMRepository defines persistence operations for BOM and its component lines.
type BOMRepository interface {
	Create(ctx context.Context, bom *models.BOM) error
	FindByID(ctx context.Context, id string) (*models.BOM, error)
	// FindByOutputItem returns all BOM versions for a given output item.
	FindByOutputItem(ctx context.Context, outputItemID string) ([]*models.BOM, error)
	FindAll(ctx context.Context) ([]*models.BOM, error)
	Update(ctx context.Context, bom *models.BOM) error
	Delete(ctx context.Context, id string) error

	// BOM Lines
	AddLine(ctx context.Context, line *models.BOMLine) error
	ListLines(ctx context.Context, bomID string) ([]*models.BOMLine, error)
	DeleteLine(ctx context.Context, id string) error
}

// ── SOP ───────────────────────────────────────────────────────────────────────

// SOPRepository defines persistence operations for SOP and its steps.
type SOPRepository interface {
	Create(ctx context.Context, sop *models.SOP) error
	FindByID(ctx context.Context, id string) (*models.SOP, error)
	// FindByBOMID returns the active SOP linked to a BOM.
	FindByBOMID(ctx context.Context, bomID string) (*models.SOP, error)
	Update(ctx context.Context, sop *models.SOP) error
	Delete(ctx context.Context, id string) error

	// SOP Steps
	AddStep(ctx context.Context, step *models.SOPStep) error
	FindStepByID(ctx context.Context, sopStepID string) (*models.SOPStep, error)
	ListSteps(ctx context.Context, sopID string) ([]*models.SOPStep, error)
	DeleteStep(ctx context.Context, sopID string, stepID string) error
	// DeleteStepsBySOPID removes all steps for a given SOP (used before re-seeding or cascade-deleting a SOP).
	DeleteStepsBySOPID(ctx context.Context, sopID string) error
}

// ── Production Order ──────────────────────────────────────────────────────────

// ProductionOrderRepository defines persistence operations for ProductionOrder
// and its associated snapshot + staff assignments.
type ProductionOrderRepository interface {
	Create(ctx context.Context, po *models.ProductionOrder) error
	FindByID(ctx context.Context, id string) (*models.ProductionOrder, error)
	FindByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error)
	FindByStatus(ctx context.Context, status models.POStatus) ([]*models.ProductionOrder, error)
	FindAll(ctx context.Context) ([]*models.ProductionOrder, error)
	UpdateStatus(ctx context.Context, id string, status models.POStatus, actualOutput *float64) error
	Update(ctx context.Context, po *models.ProductionOrder) error
	Delete(ctx context.Context, id string) error
	FindByReferenceOrderIDs(ctx context.Context, orderIDs []string) ([]*models.ProductionOrder, error)

	// BOM Snapshot (1:1 with ProductionOrder — written at PO creation)
	SaveSnapshot(ctx context.Context, snap *models.BOMSnapshot) error
	GetSnapshot(ctx context.Context, poID string) (*models.BOMSnapshot, error)

	// Staff assignments
	AssignStaff(ctx context.Context, assignment *models.POStaffAssignment) error
	ListStaffAssignments(ctx context.Context, poID string) ([]*models.POStaffAssignment, error)
}

// ── Production Batch ──────────────────────────────────────────────────────────

// ProductionBatchRepository defines persistence operations for ProductionBatch.
type ProductionBatchRepository interface {
	Create(ctx context.Context, batch *models.ProductionBatch) error
	FindByID(ctx context.Context, id string) (*models.ProductionBatch, error)
	// FindByNode returns batches for a node, optionally filtered by status.
	FindByNode(ctx context.Context, nodeID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error)
	// FindByMachine returns batches assigned to a machine, optionally filtered by status.
	FindByMachine(ctx context.Context, machineID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error)
	// UpdateStatus atomically updates the status of a batch and relevant timestamps.
	UpdateStatus(ctx context.Context, id string, status models.BatchStatus) error
	Update(ctx context.Context, batch *models.ProductionBatch) error
	Delete(ctx context.Context, id string) error
}

// ── StaffShift ────────────────────────────────────────────────────────

// StaffShiftRepository defines persistence operations for StaffShift.
// Đây là nguồn sự thật để SchedulingEngine biết:
//   - Ai đang có mặt trong ca (status = ACTIVE)
//   - Thời gian scheduling horizon (shift_start → shift_end)
type StaffShiftRepository interface {
	Create(ctx context.Context, s *models.StaffShift) error
	FindByID(ctx context.Context, id string) (*models.StaffShift, error)

	// FindActiveByNode trả về tất cả ca đang ACTIVE tại một node.
	// SchedulingEngine dùng để lấy danh sách staff available.
	FindActiveByNode(ctx context.Context, nodeID string) ([]*models.StaffShift, error)

	// FindByStaff trả về lịch sử ca của một staff (dùng để kiểm tra có active shift không).
	FindByStaff(ctx context.Context, staffID string) ([]*models.StaffShift, error)

	// FindActiveShiftByStaff tìm ca đang ACTIVE của một staff cụ thể.
	// Dùng trong StartShift (để validate không có ca đang chạy) và EndShift (để ENDED đúng ca).
	// Trả về nil nếu không có ca nào đang ACTIVE.
	FindActiveShiftByStaff(ctx context.Context, staffID string) (*models.StaffShift, error)

	// UpdateStatus cập nhật trạng thái ca và actual_end nếu cần.
	// Dùng khi nhân viên kết thúc ca (chuẩn hoặc sớm).
	UpdateStatus(ctx context.Context, id string, status models.ShiftStatus, actualEnd *time.Time) error
}

// ── StaffTask ───────────────────────────────────────────────────────

// StaffTaskRepository defines persistence operations for StaffTask.
// SchedulingEngine viết vào đây khi tạo plan; StaffTaskUseCase cập nhật
// khi nhân viên thực hiện (Start / Done).
type StaffTaskRepository interface {
	Create(ctx context.Context, t *models.StaffTask) error
	FindByID(ctx context.Context, id string) (*models.StaffTask, error)

	// FindByPO trả về tất cả tasks được tạo ra cho một PO cụ thể.
	// Dùng để check PO completion (tất cả tasks DONE chưa?).
	FindByPO(ctx context.Context, poID string) ([]*models.StaffTask, error)

	// FindByStaff trả về tasks của một staff, filter theo status.
	// SchedulingEngine dùng để tính staff_free_at (thời điểm task cuối cùng kết thúc).
	// nil statuses = tất cả status.
	FindByStaff(ctx context.Context, staffID string, statuses []models.TaskStatus) ([]*models.StaffTask, error)

	// FindByNode trả về tasks tại một node, filter theo status.
	// Dùng cho Manager overview và fill-in task lookup.
	FindByNode(ctx context.Context, nodeID string, statuses []models.TaskStatus) ([]*models.StaffTask, error)

	// FindActiveByStaff trả về task hiện tại đang ACTIVE hoặc WAITING của staff.
	// Staff KDS dùng để hiển thị task cần làm ngay.
	// Trả về nil nếu không có task nào đang active.
	FindActiveByStaff(ctx context.Context, staffID string) (*models.StaffTask, error)

	// FindWaitingByStaff trả về các tasks đang WAITING của staff (idle step đang chạy).
	// SchedulingEngine dùng để tìm parent task có idle window có thể chèn fill-in task.
	FindWaitingByStaff(ctx context.Context, staffID string) ([]*models.StaffTask, error)

	// FindQueued trả về tất cả QUEUED tasks tại một node, sắp xếp theo CreatedAt (FIFO).
	// Dispatcher dùng để lấy pool tasks cần được assign khi có resource rảnh.
	FindQueued(ctx context.Context, nodeID string) ([]*models.StaffTask, error)

	// Update ghi lại toàn bộ task (dùng khi cập nhật status, timestamps).
	Update(ctx context.Context, t *models.StaffTask) error
}

// ── StaffTask Usecase (KDS Execution) ──────────────────────────────────────────

// StaffTaskUseCase defines the API boundary for Kitchen Display System (KDS) execution.
type StaffTaskUseCase interface {
	// StartTask is called when a staff member starts a PENDING task.
	StartTask(ctx context.Context, taskID string, actualStart time.Time) error
	
	// CompleteTask is called when a staff member finishes an ACTIVE or WAITING task.
	CompleteTask(ctx context.Context, taskID string, actualEnd time.Time) error
	
	// FailTask is called when a task fails during execution (e.g., machine breakdown).
	FailTask(ctx context.Context, taskID string, failedAt time.Time, reason string) error
}
