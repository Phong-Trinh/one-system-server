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

// ── Private Helpers ───────────────────────────────────────────────────────────

func (uc *allocationUseCase) createBatchForStep(ctx context.Context, po *models.ProductionOrder, stepID string) error {
	step, err := uc.sopRepo.FindStepByID(ctx, stepID)
	if err != nil || step == nil {
		return fmt.Errorf("step %q not found", stepID)
	}

	slotsUsed := 0.0
	if step.StationTypeID != "" {
		slotsUsed = po.TargetQty * step.SlotConsumption
	}

	batch := &models.ProductionBatch{
		ID:        uuid.NewString(),
		POID:      po.ID,
		SOPStepID: step.ID,
		NodeID:    po.NodeID,
		Qty:       po.TargetQty,
		SlotsUsed: slotsUsed,
		Status:    models.BatchQueued,
	}

	return uc.batchRepo.Create(ctx, batch)
}

// ── Implementation ────────────────────────────────────────────────────────────

type allocationUseCase struct {
	poRepo      services.ProductionOrderRepository
	batchRepo   services.ProductionBatchRepository
	machineRepo services.MachineRepository
	sopRepo     services.SOPRepository
}

func NewAllocationUseCase(
	poRepo services.ProductionOrderRepository,
	batchRepo services.ProductionBatchRepository,
	machineRepo services.MachineRepository,
	sopRepo services.SOPRepository,
) AllocationUseCase {
	return &allocationUseCase{
		poRepo:      poRepo,
		batchRepo:   batchRepo,
		machineRepo: machineRepo,
		sopRepo:     sopRepo,
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
			if err := uc.createBatchForStep(ctx, po, step.ID); err != nil {
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
		// Calculate current load and check if any batch is already physically cooking
		activeBatches, _ := uc.batchRepo.FindByMachine(ctx, m.ID, []models.BatchStatus{models.BatchAllocated, models.BatchInProgress})

		currentLoad := 0.0
		hasInProgress := false
		referenceItemID := ""

		for _, b := range activeBatches {
			currentLoad += b.SlotsUsed
			if b.Status == models.BatchInProgress {
				hasInProgress = true
			}
			if referenceItemID == "" && b.ItemID != "" {
				referenceItemID = b.ItemID
			}
		}

		// If machine is BATCH_SYNC and already has a batch in progress, it's locked until completion
		if hasInProgress && m.AllocationStrategy == models.StrategySync {
			continue
		}

		if currentLoad >= m.OperationalThreshold {
			continue // Machine is at its human-manageable limit
		}

		// 4. Try to find matching tasks in the queue for this machine
		for i := 0; i < len(queued); i++ {
			b := queued[i]
			if b == nil {
				continue
			}

			// Must match station type
			step, _ := uc.sopRepo.FindStepByID(ctx, b.SOPStepID)
			if step == nil || step.StationTypeID != m.StationTypeID {
				continue
			}

			limit := m.MaxCapacity
			if m.OperationalThreshold > 0 && m.OperationalThreshold < limit {
				limit = m.OperationalThreshold
			}

			if currentLoad >= limit {
				break // Machine full for this run
			}

			// Check capacity and handle splitting
			canAllocate := false
			needsSplit := false
			fitQty := b.Qty
			fitSlots := b.SlotsUsed

			if currentLoad+b.SlotsUsed > limit {
				// Try to split
				if step.SlotConsumption > 0 {
					available := limit - currentLoad
					fitQty = float64(int(available / step.SlotConsumption)) // Greedy floor
					if fitQty > 0 {
						fitSlots = fitQty * step.SlotConsumption
						needsSplit = true
					} else {
						continue // Cannot fit even one unit
					}
				} else {
					continue // Zero consumption but still exceeds limit? Should not happen.
				}
			}

			// Allocation Strategy Logic
			if m.AllocationStrategy == models.StrategyAsync {
				canAllocate = true
			} else {
				// BATCH_SYNC
				if currentLoad == 0 {
					canAllocate = true
				} else if b.ItemID == referenceItemID {
					canAllocate = true
				} else {
					// Check if BOTH the existing steps and the new step allow mixing
					// If any of them has AllowMix = false, they cannot be mixed.

					// We need to fetch the SOPStep for the reference batch
					refBatch, _ := uc.batchRepo.FindByMachine(ctx, m.ID, []models.BatchStatus{models.BatchAllocated, models.BatchInProgress})
					// Note: referenceItemID is tracked, but we need the SOPStep of one of the active batches
					var refStep *models.SOPStep
					if len(refBatch) > 0 {
						refStep, _ = uc.sopRepo.FindStepByID(ctx, refBatch[0].SOPStepID)
					}

					if step.AllowMix && refStep != nil && refStep.AllowMix {
						canAllocate = true
					}
				}
			}

			if canAllocate {
				if needsSplit {
					remainderQty := b.Qty - fitQty
					// Create remainder batch
					remainder := &models.ProductionBatch{
						ID:        uuid.NewString(),
						POID:      b.POID,
						SOPStepID: b.SOPStepID,
						NodeID:    b.NodeID,
						ItemID:    b.ItemID,
						Qty:       remainderQty,
						SlotsUsed: remainderQty * (fitSlots / fitQty),
						Status:    models.BatchQueued,
					}
					_ = uc.batchRepo.Create(ctx, remainder)

					// Update current batch to the portion that fits
					b.Qty = fitQty
					b.SlotsUsed = fitSlots
				}

				b.MachineID = m.ID
				b.Status = models.BatchAllocated
				now := time.Now()
				b.AllocatedAt = &now

				if err := uc.batchRepo.Update(ctx, b); err == nil {
					currentLoad += b.SlotsUsed
					if referenceItemID == "" {
						referenceItemID = b.ItemID
					}
					queued[i] = nil
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

	// 1. Get PO and all its steps to find what's next
	po, err := uc.poRepo.FindByID(ctx, batch.POID)
	if err != nil || po == nil {
		return fmt.Errorf("production order %q not found", batch.POID)
	}

	allSteps, err := uc.sopRepo.ListSteps(ctx, po.SOPID)
	if err != nil {
		return err
	}

	// 2. Get all batches already created for this PO to check completion status
	existingBatches, err := uc.batchRepo.FindByNode(ctx, po.NodeID, nil) // In a real system, filter by POID
	if err != nil {
		return err
	}

	// Helper to check if a specific step is completed
	isStepCompleted := func(stepID string) bool {
		for _, eb := range existingBatches {
			if eb.POID == po.ID && eb.SOPStepID == stepID && eb.Status == models.BatchCompleted {
				return true
			}
		}
		return false
	}

	// 3. Find steps that depend on the step we just finished
	for _, nextStep := range allSteps {
		isDependent := false
		for _, depID := range nextStep.DependsOn {
			if depID == batch.SOPStepID {
				isDependent = true
				break
			}
		}

		if isDependent {
			// 4. Check if ALL dependencies for this nextStep are now satisfied
			allMet := true
			for _, depID := range nextStep.DependsOn {
				if !isStepCompleted(depID) {
					allMet = false
					break
				}
			}

			// 5. If all met, create the next QUEUED batch
			if allMet {
				// Check if we already created a batch for this step to avoid duplicates
				alreadyExists := false
				for _, eb := range existingBatches {
					if eb.POID == po.ID && eb.SOPStepID == nextStep.ID {
						alreadyExists = true
						break
					}
				}

				if !alreadyExists {
					if err := uc.createBatchForStep(ctx, po, nextStep.ID); err != nil {
						// Log error but continue
						fmt.Printf("failed to create batch for step %s: %v\n", nextStep.ID, err)
					}
				}
			}
		}
	}

	// 6. Trigger allocation for the newly queued tasks
	return uc.RunAllocation(ctx, po.NodeID)
}
