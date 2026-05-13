package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type AllocationUseCase interface {
	// DecomposePO breaks a ProductionOrder into its initial ready SOP steps.
	DecomposePO(ctx context.Context, poID string) error
	// RunAllocation matches QUEUED batches to available machine slots.
	RunAllocation(ctx context.Context, nodeID string) error
	// ConfirmPlacement moves a batch from ALLOCATED to IN_PROGRESS.
	ConfirmPlacement(ctx context.Context, batchID string) error
	// ConfirmCompletion moves a batch to COMPLETED and triggers dependencies.
	ConfirmCompletion(ctx context.Context, batchID string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type allocationUseCase struct {
	poRepo         services.ProductionOrderRepository
	batchRepo      services.ProductionBatchRepository
	machineRepo    services.MachineRepository
	sopRepo        services.SOPRepository
	itemConfigRepo services.ConfigRepository // Assuming ItemCapacityConfig is here
}

func NewAllocationUseCase(
	poRepo services.ProductionOrderRepository,
	batchRepo services.ProductionBatchRepository,
	machineRepo services.MachineRepository,
	sopRepo services.SOPRepository,
	itemConfigRepo services.ConfigRepository,
) AllocationUseCase {
	return &allocationUseCase{
		poRepo:         poRepo,
		batchRepo:      batchRepo,
		machineRepo:    machineRepo,
		sopRepo:        sopRepo,
		itemConfigRepo: itemConfigRepo,
	}
}

func (uc *allocationUseCase) DecomposePO(ctx context.Context, poID string) error {
	po, err := uc.poRepo.FindByID(ctx, poID)
	if err != nil || po == nil {
		return fmt.Errorf("production order %q not found", poID)
	}

	steps, err := uc.sopRepo.ListSteps(ctx, po.SOPID)
	if err != nil {
		return err
	}

	// Find steps with no dependencies (entry points)
	for _, step := range steps {
		if len(step.DependsOn) == 0 {
			// Create a QUEUED batch for this step
			// Note: In a real system, we might need to handle qty splitting here
			// if the PO qty is massive, but for now we create one task per step.
			batch := &models.ProductionBatch{
				ID:        uuid.NewString(),
				POID:      po.ID,
				SOPStepID: step.ID,
				Status:    models.BatchQueued,
				// ItemID and Qty would be derived from the PO/SOP context
				// This is a simplified version
			}
			if err := uc.batchRepo.Create(ctx, batch); err != nil {
				return err
			}
		}
	}

	return uc.RunAllocation(ctx, po.NodeID)
}

func (uc *allocationUseCase) RunAllocation(ctx context.Context, nodeID string) error {
	// 1. Get all QUEUED batches for this node
	queued, err := uc.batchRepo.FindByNode(ctx, nodeID, []models.BatchStatus{models.BatchQueued})
	if err != nil {
		return err
	}
	if len(queued) == 0 {
		return nil
	}

	// 2. Get all machines for this node
	machines, err := uc.machineRepo.FindByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	// 3. For each machine, try to allocate from the queue
	for _, m := range machines {
		if m.Status == models.MachineBusy && m.AllocationStrategy == models.StrategySync {
			continue // Fully locked
		}

		// Calculate current load
		activeBatches, _ := uc.batchRepo.FindByMachine(ctx, m.ID, []models.BatchStatus{models.BatchAllocated, models.BatchInProgress})
		currentLoad := 0.0
		for _, b := range activeBatches {
			currentLoad += b.SlotsUsed
		}

		if currentLoad >= m.OperationalThreshold {
			continue // Machine is at its human-manageable limit
		}

		// 4. Try to find matching tasks in the queue for this machine
		// For SLOT_ASYNC, we can pick tasks one by one up to threshold.
		// For BATCH_SYNC, we only allocate if the machine is IDLE.

		for i := 0; i < len(queued); i++ {
			b := queued[i]
			if b == nil {
				continue
			}

			// Must match station type
			if b.MachineID != "" || m.StationTypeID != "" {
				// We need to fetch the SOPStep to know the required StationType
				step, _ := uc.sopRepo.FindStepByID(ctx, b.SOPStepID)
				if step == nil || step.StationTypeID != m.StationTypeID {
					continue
				}
			}

			// Check capacity
			if currentLoad+b.SlotsUsed > m.MaxCapacity || currentLoad+b.SlotsUsed > m.OperationalThreshold {
				continue
			}

			// Mixing Rules Logic:
			// If machine is SLOT_ASYNC, we can generally mix (independent slots).
			// If machine is BATCH_SYNC, we follow the "One-item-type-per-cycle" rule unless AllowMix=true.

			// For now, let's implement the "Same Item Type" batching as priority
			canAllocate := false
			if m.AllocationStrategy == models.StrategyAsync {
				canAllocate = true
			} else {
				// BATCH_SYNC: Only if machine is currently idle
				if currentLoad == 0 {
					canAllocate = true
				}
				// Or if we are building a consolidated batch (not implemented yet in this loop)
			}

			if canAllocate {
				b.MachineID = m.ID
				b.Status = models.BatchAllocated
				now := time.Now()
				b.AllocatedAt = &now

				if err := uc.batchRepo.Update(ctx, b); err == nil {
					currentLoad += b.SlotsUsed
					queued[i] = nil // Mark as assigned in this run
				}
			}
		}
	}

	return nil
}

func (uc *allocationUseCase) ConfirmPlacement(ctx context.Context, batchID string) error {
	// now := time.Now()
	return uc.batchRepo.UpdateStatus(ctx, batchID, models.BatchInProgress)
	// Implementation should also set StartedAt = now in the actual repo/db update
}

func (uc *allocationUseCase) ConfirmCompletion(ctx context.Context, batchID string) error {
	batch, err := uc.batchRepo.FindByID(ctx, batchID)
	if err != nil || batch == nil {
		return err
	}

	if err := uc.batchRepo.UpdateStatus(ctx, batchID, models.BatchCompleted); err != nil {
		return err
	}

	// Trigger next steps logic here...
	// 1. Check if all sibling steps for this PO are done
	// 2. Find steps that depend on this one
	// 3. If all their dependencies are met, create QUEUED batches for them

	po, _ := uc.poRepo.FindByID(ctx, batch.POID)
	return uc.RunAllocation(ctx, po.NodeID)
}
