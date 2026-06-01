package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

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
