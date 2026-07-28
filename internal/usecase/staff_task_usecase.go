package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

type staffTaskUseCase struct {
	taskRepo    services.StaffTaskRepository
	machineRepo services.MachineRepository
	engine      SchedulingEngine
}

func NewStaffTaskUseCase(
	taskRepo services.StaffTaskRepository,
	machineRepo services.MachineRepository,
	engine SchedulingEngine,
) services.StaffTaskUseCase {
	return &staffTaskUseCase{
		taskRepo:    taskRepo,
		machineRepo: machineRepo,
		engine:      engine,
	}
}

func (u *staffTaskUseCase) StartTask(ctx context.Context, taskID string, actualStart time.Time) error {
	task, err := u.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.Status != models.TaskPending {
		return fmt.Errorf("cannot start task in %s status, expected PENDING", task.Status)
	}

	// Update status
	task.Status = models.TaskActive
	task.StartedAt = &actualStart

	if err := u.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return nil
}

func (u *staffTaskUseCase) CompleteTask(ctx context.Context, taskID string, actualEnd time.Time) error {
	task, err := u.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.Status != models.TaskActive && task.Status != models.TaskWaiting {
		return fmt.Errorf("cannot complete task in %s status", task.Status)
	}

	task.Status = models.TaskDone
	task.CompletedAt = &actualEnd

	if err := u.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	// Drift Detection
	drift := actualEnd.Sub(task.ScheduledEnd)
	if drift > 5*time.Minute || drift < -5*time.Minute {
		log.Printf("Drift detected for task %s: %.0fm (Scheduled: %s, Actual: %s)",
			taskID, drift.Minutes(), task.ScheduledEnd.Format("15:04"), actualEnd.Format("15:04"))

		// Trigger Reschedule
		if err := u.engine.ReschedulePendingTasks(ctx, task.NodeID, actualEnd); err != nil {
			log.Printf("ERROR: Failed to reschedule pending tasks: %v", err)
			return err
		}
	}

	return nil
}

func (u *staffTaskUseCase) FailTask(ctx context.Context, taskID string, failedAt time.Time, reason string) error {
	task, err := u.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.Status != models.TaskActive && task.Status != models.TaskWaiting {
		return fmt.Errorf("cannot fail task in %s status", task.Status)
	}

	task.Status = models.TaskFailed
	task.CompletedAt = &failedAt

	if err := u.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	log.Printf("Task %s failed at %s. Reason: %s", taskID, failedAt.Format("15:04"), reason)

	if task.MachineID != "" {
		if mach, err := u.machineRepo.FindByID(ctx, task.MachineID); err == nil {
			mach.Status = models.MachineUnderMaintenance
			_ = u.machineRepo.Update(ctx, mach)
			log.Printf("Machine %s marked as UNDER_MAINTENANCE", task.MachineID)
		}
	}

	// Trigger Reschedule immediately
	if err := u.engine.ReschedulePendingTasks(ctx, task.NodeID, failedAt); err != nil {
		log.Printf("ERROR: Failed to reschedule pending tasks after failure: %v", err)
		return err
	}

	return nil
}
