package usecase

import (
	"math"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

// TestDispatch_DeadlineUrgency tests that when two POs have the same EarliestStart and MachineUtilizationScore,
// the PO with an earlier (more urgent) DeadlineAt is prioritized.
func TestDispatch_DeadlineUrgency(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, _, dispatcherImpl, _ := setupTestEnv()

	now := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "node_test"

	// Create 2 POs for items with identical manual SOPs (no machine time -> score = 0.0)
	sopManual := "sop_manual"
	sopRepo.sops[sopManual] = &models.SOP{ID: sopManual, Version: 1}
	sopRepo.steps["step_manual"] = &models.SOPStep{
		ID:          "step_manual",
		SOPID:       sopManual,
		SeqNo:       1,
		Duration:    600,
		Description: "Manual Step",
	}

	// PO 1: Deadline in 2 hours (more urgent)
	deadlineUrgent := now.Add(2 * time.Hour)
	poUrgent := &models.ProductionOrder{
		ID:                      "po_urgent",
		NodeID:                  nodeID,
		SOPID:                   sopManual,
		Status:                  models.POInProgress,
		MachineUtilizationScore: 0.0,
		DeadlineAt:              &deadlineUrgent,
		CreatedAt:               now,
	}
	_ = poRepo.Create(ctx, poUrgent)

	// PO 2: Deadline in 5 hours (less urgent)
	deadlineRelaxed := now.Add(5 * time.Hour)
	poRelaxed := &models.ProductionOrder{
		ID:                      "po_relaxed",
		NodeID:                  nodeID,
		SOPID:                   sopManual,
		Status:                  models.POInProgress,
		MachineUtilizationScore: 0.0,
		DeadlineAt:              &deadlineRelaxed,
		CreatedAt:               now,
	}
	_ = poRepo.Create(ctx, poRelaxed)

	// Create QUEUED tasks for both POs with identical EarliestStart
	taskUrgent := &models.StaffTask{
		ID:            "task_urgent",
		POID:          poUrgent.ID,
		SOPStepID:     "step_manual",
		NodeID:        nodeID,
		TaskKind:      models.TaskKindNormal,
		Status:        models.TaskQueued,
		EarliestStart: now,
		CreatedAt:     now,
	}
	_ = taskRepo.Create(ctx, taskUrgent)

	taskRelaxed := &models.StaffTask{
		ID:            "task_relaxed",
		POID:          poRelaxed.ID,
		SOPStepID:     "step_manual",
		NodeID:        nodeID,
		TaskKind:      models.TaskKindNormal,
		Status:        models.TaskQueued,
		EarliestStart: now,
		CreatedAt:     now,
	}
	_ = taskRepo.Create(ctx, taskRelaxed)

	// Create an active shift so staff is available
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID:      "shift_1",
		NodeID:  nodeID,
		StaffID: "staff_1",
		Status:  models.ShiftActive,
	}

	// Execute Dispatch
	err := dispatcherImpl.Dispatch(ctx, nodeID)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	// Verify task_urgent was assigned first (Status = PENDING)
	tUrgent, _ := taskRepo.FindByID(ctx, "task_urgent")
	if tUrgent.Status != models.TaskPending {
		t.Errorf("expected task_urgent to be PENDING, got %s", tUrgent.Status)
	}
	if tUrgent.AssignedTo != "staff_1" {
		t.Errorf("expected task_urgent assigned to staff_1, got %s", tUrgent.AssignedTo)
	}
}

// TestPoDeadlineUrgency_NilDeadline verifies fallback behavior when DeadlineAt is nil.
func TestPoDeadlineUrgency_NilDeadline(t *testing.T) {
	ctx, poRepo, _, _, _, _, _, dispatcherImpl, _ := setupTestEnv()
	now := time.Now()

	poNoDeadline := &models.ProductionOrder{
		ID:         "po_no_dl",
		DeadlineAt: nil,
	}
	_ = poRepo.Create(ctx, poNoDeadline)

	// Cast interface to struct to test private helper method
	d, ok := dispatcherImpl.(*dispatcher)
	if !ok {
		t.Fatalf("failed to cast Dispatcher to *dispatcher")
	}

	cache := make(map[string]int64)
	urgency := d.poDeadlineUrgency(ctx, cache, "po_no_dl", now)

	if urgency != math.MaxInt64 {
		t.Errorf("expected MaxInt64 for nil deadline, got %d", urgency)
	}
}
