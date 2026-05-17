package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
	"one-system-server/internal/usecase"
)

type KDSHandler struct {
	allocationUC usecase.AllocationUseCase
	batchRepo    services.ProductionBatchRepository
	sopRepo      services.SOPRepository
	orchestrator *usecase.OrderPoolingOrchestrator
}

func newKDSHandler(
	auc usecase.AllocationUseCase,
	batchRepo services.ProductionBatchRepository,
	sopRepo services.SOPRepository,
	orchestrator *usecase.OrderPoolingOrchestrator,
) *KDSHandler {
	return &KDSHandler{
		allocationUC: auc,
		batchRepo:    batchRepo,
		sopRepo:      sopRepo,
		orchestrator: orchestrator,
	}
}

// ConfirmPlacement handles the "Confirm Placed on Machine" action.
// Staff clicks this when they actually put the item on the equipment.
func (h *KDSHandler) ConfirmPlacement(c *gin.Context) {
	id := c.Param("id")
	if err := h.allocationUC.ConfirmPlacement(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Batch started"})
}

// ConfirmCompletion handles the "Confirm Completed/Taken Off" action.
// Staff clicks this when the timer ends and they remove the item.
// After completion, it triggers the orchestrator to check if pooled orders
// can now be flushed to fill the newly freed machine slot.
func (h *KDSHandler) ConfirmCompletion(c *gin.Context) {
	id := c.Param("id")

	// Get batch before completing to read its nodeID for the flush trigger
	batch, _ := h.batchRepo.FindByID(c.Request.Context(), id)

	if err := h.allocationUC.ConfirmCompletion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Trigger capacity-aware flush: a machine just freed up → check if pool
	// has orders that can now be allocated to it immediately.
	if batch != nil && h.orchestrator != nil {
		h.orchestrator.TriggerFlushForNode(c.Request.Context(), batch.NodeID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Batch completed"})
}

// KDSBatchView is the denormalized response model for the KDS task queue UI.
// It enriches a raw ProductionBatch with human-readable step and timing info.
type KDSBatchView struct {
	ID        string  `json:"id"`
	POID      string  `json:"po_id"`
	SOPStepID string  `json:"sop_step_id"`
	StepName  string  `json:"step_name"`  // SOPStep.Description or ID as fallback
	MachineID string  `json:"machine_id"`
	ItemID    string  `json:"item_id"`    // ID of the target product
	Duration  int     `json:"duration"`   // SOPStep.Duration in seconds
	Status    string  `json:"status"`
	Qty       float64 `json:"qty"`
	SlotsUsed float64 `json:"slots_used"`
	Elapsed   int     `json:"elapsed"`    // Seconds since StartedAt (server-computed, avoids client clock drift)
}

// ListBatches returns all active batches for the KDS task queue display.
// Query params:
//   - node_id (required): filter by kitchen node
//   - status (optional, repeatable): e.g. ?status=QUEUED&status=ALLOCATED&status=IN_PROGRESS
func (h *KDSHandler) ListBatches(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id query param is required"})
		return
	}

	statusStrings := c.QueryArray("status")
	var statuses []models.BatchStatus
	for _, s := range statusStrings {
		statuses = append(statuses, models.BatchStatus(s))
	}

	batches, err := h.batchRepo.FindByNode(c.Request.Context(), nodeID, statuses)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Enrich with step metadata
	views := make([]*KDSBatchView, 0, len(batches))
	for _, b := range batches {
		view := &KDSBatchView{
			ID:        b.ID,
			POID:      b.POID,
			SOPStepID: b.SOPStepID,
			MachineID: b.MachineID,
			ItemID:    b.ItemID,
			Status:    string(b.Status),
			Qty:       b.Qty,
			SlotsUsed: b.SlotsUsed,
		}

		// Lookup step for human-readable name and duration
		if step, err := h.sopRepo.FindStepByID(c.Request.Context(), b.SOPStepID); err == nil && step != nil {
			view.Duration = step.Duration
			if step.Description != "" {
				view.StepName = step.Description
			} else {
				view.StepName = step.ID // Fallback to ID
			}
		}

		// Compute elapsed seconds server-side to avoid client clock drift
		if b.StartedAt != nil {
			view.Elapsed = int(time.Since(*b.StartedAt).Seconds())
		}

		views = append(views, view)
	}

	c.JSON(http.StatusOK, views)
}

// GetPoolStatus returns the current state of the order pool per node.
// Used by the Orchestrator pane in the UI to show countdown timers.
func (h *KDSHandler) GetPoolStatus(c *gin.Context) {
	if h.orchestrator == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, h.orchestrator.PoolStatus())
}
