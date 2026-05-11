package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

type itemHandler struct {
	uc usecase.ItemUseCase
}

func newItemHandler(uc usecase.ItemUseCase) *itemHandler {
	return &itemHandler{uc: uc}
}

func (h *itemHandler) Create(c *gin.Context) {
	var req struct {
		OrgID    string          `json:"org_id" binding:"required"`
		Name     string          `json:"name" binding:"required"`
		SKU      string          `json:"sku"`
		Type     models.ItemType `json:"type" binding:"required"`
		BaseUnit string          `json:"base_unit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.uc.Create(c.Request.Context(), req.OrgID, req.Name, req.SKU, req.Type, req.BaseUnit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *itemHandler) List(c *gin.Context) {
	orgID := c.Query("org_id")
	var items []*models.Item
	var err error

	if orgID != "" {
		items, err = h.uc.ListByOrg(c.Request.Context(), orgID)
	} else {
		items, err = h.uc.List(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *itemHandler) GetByID(c *gin.Context) {
	item, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *itemHandler) Update(c *gin.Context) {
	var req struct {
		Name     string          `json:"name" binding:"required"`
		SKU      string          `json:"sku"`
		Type     models.ItemType `json:"type" binding:"required"`
		BaseUnit string          `json:"base_unit" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.uc.Update(c.Request.Context(), c.Param("id"), req.Name, req.SKU, req.Type, req.BaseUnit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *itemHandler) Delete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
