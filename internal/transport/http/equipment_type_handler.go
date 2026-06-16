package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

type EquipmentTypeHandler struct {
	eqTypeRepo services.EquipmentTypeRepository
}

func newEquipmentTypeHandler(repo services.EquipmentTypeRepository) *EquipmentTypeHandler {
	return &EquipmentTypeHandler{eqTypeRepo: repo}
}

func (h *EquipmentTypeHandler) List(c *gin.Context) {
	eqTypes, err := h.eqTypeRepo.FindAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, eqTypes)
}

func (h *EquipmentTypeHandler) Create(c *gin.Context) {
	var eqType models.EquipmentType
	if err := c.ShouldBindJSON(&eqType); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Default status to DRAFT if not specified
	if eqType.Status == "" {
		eqType.Status = models.EquipmentTypeDraft
	}

	if err := h.eqTypeRepo.Create(c.Request.Context(), &eqType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, eqType)
}

func (h *EquipmentTypeHandler) Update(c *gin.Context) {
	id := c.Param("id")

	eqType, err := h.eqTypeRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Equipment type not found"})
		return
	}
	if eqType == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Equipment type not found"})
		return
	}

	var req struct {
		Name         string `json:"name"`
		CapacityUnit string `json:"capacity_unit"`
		Status       string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if req.Name != "" {
		eqType.Name = req.Name
	}
	if req.CapacityUnit != "" {
		eqType.CapacityUnit = req.CapacityUnit
	}
	if req.Status != "" {
		eqType.Status = models.EquipmentTypeStatus(req.Status)
	}

	if err := h.eqTypeRepo.Update(c.Request.Context(), eqType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, eqType)
}
