package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

type productionHandler struct {
	uc usecase.ProductionUseCase
}

func newProductionHandler(uc usecase.ProductionUseCase) *productionHandler {
	return &productionHandler{uc: uc}
}

// POST /api/v1/production/orders
func (h *productionHandler) CreateOrder(c *gin.Context) {
	var req struct {
		BOMID     string  `json:"bom_id" binding:"required"`
		NodeID    string  `json:"node_id" binding:"required"`
		TargetQty float64 `json:"target_qty" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	po, err := h.uc.CreateProductionOrder(c.Request.Context(), req.BOMID, req.NodeID, req.TargetQty)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, po)
}

// GET /api/v1/production/orders/:id
func (h *productionHandler) GetOrder(c *gin.Context) {
	po, err := h.uc.GetProductionOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, po)
}

// GET /api/v1/production/orders?node_id=
func (h *productionHandler) ListOrders(c *gin.Context) {
	nodeID := c.Query("node_id")
	var pos []*models.ProductionOrder
	var err error

	if nodeID != "" {
		pos, err = h.uc.ListProductionOrdersByNode(c.Request.Context(), nodeID)
	} else {
		pos, err = h.uc.ListAllOrders(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pos)
}

// PATCH /api/v1/production/orders/:id/status
func (h *productionHandler) UpdateStatus(c *gin.Context) {
	var req struct {
		Status       models.POStatus `json:"status" binding:"required"`
		ActualOutput *float64        `json:"actual_output"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.uc.UpdatePOStatus(c.Request.Context(), c.Param("id"), req.Status, req.ActualOutput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /api/v1/production/boms
func (h *productionHandler) CreateBOM(c *gin.Context) {
	var req struct {
		OutputItemID string            `json:"output_item_id" binding:"required"`
		Lines        []*models.BOMLine `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bom, err := h.uc.CreateBOM(c.Request.Context(), req.OutputItemID, req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, bom)
}

// POST /api/v1/production/sops
func (h *productionHandler) CreateSOP(c *gin.Context) {
	var req struct {
		BOMID string            `json:"bom_id" binding:"required"`
		Steps []*models.SOPStep `json:"steps" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sop, err := h.uc.CreateSOP(c.Request.Context(), req.BOMID, req.Steps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, sop)
}
// GET /api/v1/production/boms/by-item/:id
func (h *productionHandler) GetFullBOMByItem(c *gin.Context) {
	bom, lines, err := h.uc.GetFullBOMByItem(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if bom == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOM not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bom": bom, "lines": lines})
}

// PUT /api/v1/production/boms/:id
func (h *productionHandler) UpdateBOM(c *gin.Context) {
	var req struct {
		Lines []*models.BOMLine `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.uc.UpdateBOM(c.Request.Context(), c.Param("id"), req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// GET /api/v1/production/sops/by-bom/:id
func (h *productionHandler) GetFullSOPByBOM(c *gin.Context) {
	sop, steps, err := h.uc.GetFullSOPByBOM(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sop == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SOP not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sop": sop, "steps": steps})
}

// PUT /api/v1/production/sops/:id
func (h *productionHandler) UpdateSOP(c *gin.Context) {
	var req struct {
		Steps []*models.SOPStep `json:"steps" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.uc.UpdateSOP(c.Request.Context(), c.Param("id"), req.Steps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
