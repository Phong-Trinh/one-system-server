package services

import (
	"context"

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
