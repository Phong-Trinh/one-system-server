package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type staffHandler struct {
	uc usecase.StaffUseCase
}

func newStaffHandler(uc usecase.StaffUseCase) *staffHandler {
	return &staffHandler{uc: uc}
}

// POST /api/v1/staff
func (h *staffHandler) Create(c *gin.Context) {
	var req struct {
		NodeID   string  `json:"node_id"   binding:"required"`
		Name     string  `json:"name"      binding:"required"`
		WageRate float64 `json:"wage_rate" binding:"min=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := h.uc.Create(c.Request.Context(), req.NodeID, req.Name, req.WageRate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, s)
}

// GET /api/v1/staff/:id
func (h *staffHandler) GetByID(c *gin.Context) {
	s, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// GET /api/v1/staff?node_id=
func (h *staffHandler) ListByNode(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id query param is required"})
		return
	}
	staff, err := h.uc.ListByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, staff)
}

// PUT /api/v1/staff/:id
func (h *staffHandler) Update(c *gin.Context) {
	var req struct {
		Name     string  `json:"name"      binding:"required"`
		WageRate float64 `json:"wage_rate" binding:"min=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s, err := h.uc.Update(c.Request.Context(), c.Param("id"), req.Name, req.WageRate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

// DELETE /api/v1/staff/:id
func (h *staffHandler) Delete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
