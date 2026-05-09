package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type stationTypeHandler struct {
	uc usecase.StationTypeUseCase
}

func newStationTypeHandler(uc usecase.StationTypeUseCase) *stationTypeHandler {
	return &stationTypeHandler{uc: uc}
}

// POST /api/v1/station-types
func (h *stationTypeHandler) Create(c *gin.Context) {
	var req struct {
		ID           string `json:"id"            binding:"required"`
		Name         string `json:"name"          binding:"required"`
		CapacityUnit string `json:"capacity_unit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := h.uc.Create(c.Request.Context(), req.ID, req.Name, req.CapacityUnit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, st)
}

// GET /api/v1/station-types
func (h *stationTypeHandler) List(c *gin.Context) {
	list, err := h.uc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// GET /api/v1/station-types/:id
func (h *stationTypeHandler) GetByID(c *gin.Context) {
	st, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

// PUT /api/v1/station-types/:id
func (h *stationTypeHandler) Update(c *gin.Context) {
	var req struct {
		Name         string `json:"name"          binding:"required"`
		CapacityUnit string `json:"capacity_unit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	st, err := h.uc.Update(c.Request.Context(), c.Param("id"), req.Name, req.CapacityUnit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

// DELETE /api/v1/station-types/:id
func (h *stationTypeHandler) Delete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
