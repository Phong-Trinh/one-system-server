package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

type orderHandler struct {
	uc     usecase.OrderUseCase
	orgID  string // system default org — pulled from config
	hqNode string // system default HQ node for ROP facade calls
}

func newOrderHandler(uc usecase.OrderUseCase, orgID, hqNode string) *orderHandler {
	return &orderHandler{uc: uc, orgID: orgID, hqNode: hqNode}
}

// POST /api/v1/orders
func (h *orderHandler) Create(c *gin.Context) {
	var req struct {
		NodeID   string `json:"node_id" binding:"required"`
		Source   string `json:"source" binding:"required"` // DIRECT_POS | PLATFORM
		Platform *string `json:"platform"`
		Items    []struct {
			ItemID   string  `json:"item_id" binding:"required"`
			Quantity int     `json:"quantity" binding:"required"`
			Price    float64 `json:"price"`
		} `json:"items" binding:"required"`
		DeadlineAt *time.Time `json:"deadline_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	items := make([]models.OrderItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = models.OrderItem{ItemID: it.ItemID, Quantity: it.Quantity, Price: it.Price}
	}

	order, err := h.uc.CreateOrder(c.Request.Context(), req.NodeID, req.Source, req.Platform, items, req.DeadlineAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, order)
}

// GET /api/v1/orders?node_id=
func (h *orderHandler) List(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id query param required"})
		return
	}
	orders, err := h.uc.ListByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, orders)
}

// GET /api/v1/orders/:id
func (h *orderHandler) GetByID(c *gin.Context) {
	order, err := h.uc.GetOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

// PATCH /api/v1/orders/:id/complete — completes order and fires stock-out + ROP
func (h *orderHandler) Complete(c *gin.Context) {
	if err := h.uc.CompleteOrder(c.Request.Context(), c.Param("id"), h.orgID, h.hqNode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// PATCH /api/v1/orders/:id/cancel
func (h *orderHandler) Cancel(c *gin.Context) {
	if err := h.uc.CancelOrder(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
