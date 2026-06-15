package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

type inventoryHandler struct {
	invSvc     services.InventoryService
	stockRepo  services.NodeStockRepository
	configRepo services.NodeItemConfigRepository
}

func newInventoryHandler(
	invSvc services.InventoryService,
	stockRepo services.NodeStockRepository,
	configRepo services.NodeItemConfigRepository,
) *inventoryHandler {
	return &inventoryHandler{
		invSvc:     invSvc,
		stockRepo:  stockRepo,
		configRepo: configRepo,
	}
}

// GET /api/v1/inventory?node_id=
// Returns all NodeStock records for a given node.
func (h *inventoryHandler) ListStock(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}
	// We need to list all stocks by node — use direct repo access.
	stocks, err := h.stockRepo.ListByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stocks)
}

// POST /api/v1/inventory/init — Initialize or correct stock via stock-take
func (h *inventoryHandler) InitStock(c *gin.Context) {
	var req struct {
		NodeID string  `json:"node_id" binding:"required"`
		ItemID string  `json:"item_id" binding:"required"`
		QtyBU  float64 `json:"qty_bu" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.invSvc.InitStock(c.Request.Context(), req.NodeID, req.ItemID, req.QtyBU); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "stock initialized", "node_id": req.NodeID, "item_id": req.ItemID, "qty_bu": req.QtyBU})
}

// GET /api/v1/node-item-configs?node_id=
// Returns all NodeItemConfig entries for a node.
func (h *inventoryHandler) ListConfigs(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}
	configs, err := h.configRepo.ListByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// PUT /api/v1/node-item-configs — Upsert a NodeItemConfig (set ROP, strategy, etc.)
func (h *inventoryHandler) UpsertConfig(c *gin.Context) {
	var req struct {
		NodeID               string                  `json:"node_id" binding:"required"`
		ItemID               string                  `json:"item_id" binding:"required"`
		SourcingStrategy     models.SourcingStrategy `json:"sourcing_strategy" binding:"required"`
		ProviderNodeID       *string                 `json:"provider_node_id"`
		SupplierID           *string                 `json:"supplier_id"`
		ReorderPoint         float64                 `json:"reorder_point"`
		SafetyStock          float64                 `json:"safety_stock"`
		SupplierLeadTimeDays int                     `json:"supplier_lead_time_days"`
		ConsumptionWindowDays int                    `json:"consumption_window_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := &models.NodeItemConfig{
		NodeID:                req.NodeID,
		ItemID:                req.ItemID,
		SourcingStrategy:      req.SourcingStrategy,
		ProviderNodeID:        req.ProviderNodeID,
		SupplierID:            req.SupplierID,
		ReorderPoint:          req.ReorderPoint,
		SafetyStock:           req.SafetyStock,
		SupplierLeadTimeDays:  req.SupplierLeadTimeDays,
		ConsumptionWindowDays: req.ConsumptionWindowDays,
		UpdatedAt:             time.Now(),
	}
	if cfg.ConsumptionWindowDays == 0 {
		cfg.ConsumptionWindowDays = 30
	}

	if err := h.configRepo.Upsert(c.Request.Context(), cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}
