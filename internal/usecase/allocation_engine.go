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
	// SetFacade injects the supply chain facade to break circular dependencies.
	SetFacade(facade *SupplyChainFacade)
}

// ── Implementation ────────────────────────────────────────────────────────────

type allocationUseCase struct {
	poRepo      services.ProductionOrderRepository
	batchRepo   services.ProductionBatchRepository
	machineRepo services.MachineRepository
	sopRepo     services.SOPRepository
	facade      *SupplyChainFacade
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

func (uc *allocationUseCase) SetFacade(facade *SupplyChainFacade) {
	uc.facade = facade
}

// ── Private Helpers ───────────────────────────────────────────────────────────



// createBatchForStep creates a QUEUED ProductionBatch for the given SOP step.
// Slot consumption is derived from ItemCapacityConfig for the step's equipment type.
// Steps with no equipment type (manual/non-machine) get slotsUsed=0.
func (uc *allocationUseCase) createBatchForStep(ctx context.Context, po *models.ProductionOrder, stepID string) error {
	step, err := uc.sopRepo.FindStepByID(ctx, stepID)
	if err != nil || step == nil {
		return fmt.Errorf("step %q not found", stepID)
	}

	slotsUsed := 0.0
	if step.EquipmentTypeID != nil {
		slotsUsed = po.TargetQty * step.SlotConsumption
	}

	batch := &models.ProductionBatch{
		ID:        uuid.NewString(),
		POID:      po.ID,
		SOPStepID: step.ID,
		NodeID:    po.NodeID,
		ItemID:    po.ItemID,
		Qty:       po.TargetQty,
		SlotsUsed: slotsUsed,
		Status:    models.BatchQueued,
	}

	return uc.batchRepo.Create(ctx, batch)
}

// ── DecomposePO ───────────────────────────────────────────────────────────────

func (uc *allocationUseCase) DecomposePO(ctx context.Context, poID string) error {
	po, err := uc.poRepo.FindByID(ctx, poID)
	if err != nil || po == nil {
		return fmt.Errorf("production order %q not found", poID)
	}

	steps, err := uc.sopRepo.ListSteps(ctx, po.SOPID)
	if err != nil {
		return err
	}

	// Create batches for entry-point steps (no dependencies).
	for _, step := range steps {
		if len(step.DependsOn) == 0 {
			if err := uc.createBatchForStep(ctx, po, step.ID); err != nil {
				return err
			}
		}
	}

	return uc.RunAllocation(ctx, po.NodeID)
}

// ── RunAllocation ─────────────────────────────────────────────────────────────

func (uc *allocationUseCase) RunAllocation(ctx context.Context, nodeID string) error {
	// 1. Get all QUEUED batches for this node.
	queued, err := uc.batchRepo.FindByNode(ctx, nodeID, []models.BatchStatus{models.BatchQueued})
	if err != nil {
		return err
	}
	if len(queued) == 0 {
		return nil
	}

	// 2. Get all machines for this node.
	machines, err := uc.machineRepo.FindByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}

	// 3. For each machine, try to allocate from the queue.
	for _, m := range machines {
		if m.Status == models.MachineUnderMaintenance || m.Status == models.MachineDecommissioned {
			continue // Skip machines that are not available for production.
		}

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

		// If machine already has a batch in progress, lock it until completion (BATCH_SYNC behaviour).
		// Note: In the current model, all machines operate as BATCH_SYNC by default.
		// A future AllocationStrategy field on Machine/EquipmentType can relax this.
		if hasInProgress {
			continue
		}

		// Build a set of active EquipmentType descriptions on this machine for mix-priority pass.
		activeMixDescriptions := make(map[string]bool)
		for _, ab := range activeBatches {
			abStep, _ := uc.sopRepo.FindStepByID(ctx, ab.SOPStepID)
			if abStep != nil && abStep.EquipmentTypeID != nil && abStep.AllowMix {
				activeMixDescriptions[abStep.Description] = true
			}
		}

		// Two-pass allocation: prioritize batches matching currently active mix types (Pass 1),
		// then evaluate remaining batches (Pass 2).
		for pass := 1; pass <= 2; pass++ {
			for i := 0; i < len(queued); i++ {
				b := queued[i]
				if b == nil {
					continue
				}

				// Load step for this batch.
				step, _ := uc.sopRepo.FindStepByID(ctx, b.SOPStepID)
				if step == nil || step.EquipmentTypeID == nil {
					continue // Non-machine step; skip allocation engine.
				}

				// Machine must match the step's equipment type.
				if *step.EquipmentTypeID != m.EquipmentTypeID {
					continue
				}


				// Priority pass: only accept steps matching currently active mix descriptions.
				if step.AllowMix && len(activeMixDescriptions) > 0 {
					isMatchingActive := activeMixDescriptions[step.Description]
					if pass == 1 && !isMatchingActive {
						continue
					}
					if pass == 2 && isMatchingActive {
						continue
					}
				} else if pass == 1 {
					continue
				}

				if currentLoad >= m.MaxCapacity {
					break
				}

				// Determine fit quantity (with splitting if batch exceeds remaining capacity).
				canAllocate := false
				needsSplit := false
				fitQty := b.Qty
				fitSlots := b.SlotsUsed
				slotPerUnit := step.SlotConsumption

				if currentLoad+b.SlotsUsed > m.MaxCapacity {
					if slotPerUnit > 0 {
						available := m.MaxCapacity - currentLoad
						fitQty = float64(int(available / slotPerUnit))
						if fitQty > 0 {
							fitSlots = fitQty * slotPerUnit
							needsSplit = true
						} else {
							continue
						}
					} else {
						continue
					}
				}

				// Mixing / item exclusivity check.
				if currentLoad == 0 {
					canAllocate = true
				} else if b.ItemID == referenceItemID {
					canAllocate = true
				} else if step.AllowMix {
					// Check if the reference item's step also allows mixing.
					if len(activeBatches) > 0 {
						refStep, _ := uc.sopRepo.FindStepByID(ctx, activeBatches[0].SOPStepID)
						if refStep != nil && refStep.AllowMix {
							canAllocate = true
						}
					}
				}

				if canAllocate {
					if needsSplit {
						remainderQty := b.Qty - fitQty
						remainder := &models.ProductionBatch{
							ID:        uuid.NewString(),
							POID:      b.POID,
							SOPStepID: b.SOPStepID,
							NodeID:    b.NodeID,
							ItemID:    b.ItemID,
							Qty:       remainderQty,
							SlotsUsed: remainderQty * slotPerUnit,
							Status:    models.BatchQueued,
						}
						_ = uc.batchRepo.Create(ctx, remainder)

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
						if step.AllowMix {
							activeMixDescriptions[step.Description] = true
						}
						queued[i] = nil
					}
				}
			}
		}
	}

	return nil
}

// ── ConfirmPlacement ──────────────────────────────────────────────────────────

func (uc *allocationUseCase) ConfirmPlacement(ctx context.Context, batchID string) error {
	batch, err := uc.batchRepo.FindByID(ctx, batchID)
	if err != nil || batch == nil {
		return err
	}

	if err := uc.batchRepo.UpdateStatus(ctx, batchID, models.BatchInProgress); err != nil {
		return err
	}

	// Stock deduct for this specific step's ingredients
	if uc.facade != nil {
		step, _ := uc.sopRepo.FindStepByID(ctx, batch.SOPStepID)
		if step != nil && len(step.IngredientBOMLineIDs) > 0 {
			po, _ := uc.poRepo.FindByID(ctx, batch.POID)
			bom, bomLines, _ := uc.facade.GetBOMByItem(ctx, batch.ItemID)
			if po != nil && bom != nil {
				// We need a map of line ID to BOMLine
				lineMap := make(map[string]*models.BOMLine)
				for _, l := range bomLines {
					lineMap[l.ID] = l
				}

				for _, lineID := range step.IngredientBOMLineIDs {
					if line, ok := lineMap[lineID]; ok {
						totalIngQty := line.Qty * batch.Qty // Use batch.Qty (it handles split batches)
						// Use "HQ" as org and hqNodeID context
						if err := uc.facade.StockOutWithROP(ctx, "SNAPBITE_ORG", "HQ", batch.NodeID, line.ItemID, totalIngQty); err != nil {
							fmt.Printf("failed to stock out ingredient %s for PO %s batch %s: %v\n", line.ItemID, po.ID, batch.ID, err)
						}
					}
				}
			}
		}
	}

	return nil
}

// ── ConfirmCompletion ─────────────────────────────────────────────────────────

func (uc *allocationUseCase) ConfirmCompletion(ctx context.Context, batchID string) error {
	batch, err := uc.batchRepo.FindByID(ctx, batchID)
	if err != nil || batch == nil {
		return err
	}

	if err := uc.batchRepo.UpdateStatus(ctx, batchID, models.BatchCompleted); err != nil {
		return err
	}

	po, err := uc.poRepo.FindByID(ctx, batch.POID)
	if err != nil || po == nil {
		return fmt.Errorf("production order %q not found", batch.POID)
	}

	allSteps, err := uc.sopRepo.ListSteps(ctx, po.SOPID)
	if err != nil {
		return err
	}

	existingBatches, err := uc.batchRepo.FindByNode(ctx, po.NodeID, nil)
	if err != nil {
		return err
	}

	isStepCompleted := func(stepID string) bool {
		for _, eb := range existingBatches {
			if eb.POID == po.ID && eb.SOPStepID == stepID && eb.Status == models.BatchCompleted {
				return true
			}
		}
		return false
	}

	// Find and unlock dependent steps whose all dependencies are now met.
	for _, nextStep := range allSteps {
		isDependent := false
		for _, depID := range nextStep.DependsOn {
			if depID == batch.SOPStepID {
				isDependent = true
				break
			}
		}
		if !isDependent {
			continue
		}

		allMet := true
		for _, depID := range nextStep.DependsOn {
			if !isStepCompleted(depID) {
				allMet = false
				break
			}
		}

		if allMet {
			alreadyExists := false
			for _, eb := range existingBatches {
				if eb.POID == po.ID && eb.SOPStepID == nextStep.ID {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				if err := uc.createBatchForStep(ctx, po, nextStep.ID); err != nil {
					fmt.Printf("failed to create batch for step %s: %v\n", nextStep.ID, err)
				}
			}
		}
	}

	// Check if ALL steps in the SOP are completed.
	allSOPCompleted := true
	for _, step := range allSteps {
		if !isStepCompleted(step.ID) {
			allSOPCompleted = false
			break
		}
	}
	if allSOPCompleted {
		if err := uc.poRepo.UpdateStatus(ctx, po.ID, models.POCompleted, nil); err != nil {
			fmt.Printf("failed to mark PO %s completed: %v\n", po.ID, err)
		} else {
			// PO Completed! Stock in the final produced item
			if uc.facade != nil {
				_ = uc.facade.Inventory.StockIn(ctx, po.NodeID, po.ItemID, po.TargetQty)
			}
		}
	}

	return uc.RunAllocation(ctx, po.NodeID)
}
