package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type machineHandler struct {
	uc usecase.MachineUseCase
}

func newMachineHandler(uc usecase.MachineUseCase) *machineHandler {
	return &machineHandler{uc: uc}
}

// POST /api/v1/machines
func (h *machineHandler) Create(c *gin.Context) {
	var req struct {
		NodeID        string `json:"node_id"         binding:"required"`
		StationTypeID string `json:"station_type_id" binding:"required"`
		MaxSlots      int    `json:"max_slots"       binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.uc.Create(c.Request.Context(), req.NodeID, req.StationTypeID, req.MaxSlots)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, m)
}

// GET /api/v1/machines/:id
func (h *machineHandler) GetByID(c *gin.Context) {
	m, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// GET /api/v1/machines?node_id=
func (h *machineHandler) ListByNode(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id query param is required"})
		return
	}
	machines, err := h.uc.ListByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, machines)
}

// PUT /api/v1/machines/:id
func (h *machineHandler) Update(c *gin.Context) {
	var req struct {
		MaxSlots int `json:"max_slots" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := h.uc.Update(c.Request.Context(), c.Param("id"), req.MaxSlots)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// DELETE /api/v1/machines/:id
func (h *machineHandler) Delete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
