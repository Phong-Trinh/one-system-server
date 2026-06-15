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
