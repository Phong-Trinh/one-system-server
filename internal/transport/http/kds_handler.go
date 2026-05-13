package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type KDSHandler struct {
	allocationUC usecase.AllocationUseCase
}

func newKDSHandler(auc usecase.AllocationUseCase) *KDSHandler {
	return &KDSHandler{allocationUC: auc}
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
func (h *KDSHandler) ConfirmCompletion(c *gin.Context) {
	id := c.Param("id")
	if err := h.allocationUC.ConfirmCompletion(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Batch completed"})
}
