package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
	"one-system-server/internal/usecase"
)

type SupplierHandler struct {
	supplierRepo services.SupplierRepository
	purOUC       usecase.PurOUseCase
}

func newSupplierHandler(repo services.SupplierRepository, purOUC usecase.PurOUseCase) *SupplierHandler {
	return &SupplierHandler{supplierRepo: repo, purOUC: purOUC}
}

func (h *SupplierHandler) Create(c *gin.Context) {
	var supplier models.Supplier
	if err := c.ShouldBindJSON(&supplier); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if supplier.ID == "" {
		supplier.ID = uuid.NewString()
	}

	if err := h.supplierRepo.Create(c.Request.Context(), &supplier); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, supplier)
}

func (h *SupplierHandler) List(c *gin.Context) {
	orgID := c.Query("org_id")
	if orgID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_id query param is required"})
		return
	}

	suppliers, err := h.supplierRepo.FindByOrg(c.Request.Context(), orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, suppliers)
}

// SupplierPriceRequest represents the payload for querying historical prices for multiple lines
type SupplierPriceRequest struct {
	SupplierID string `json:"supplier_id"`
	Lines []struct {
		ItemID *string `json:"item_id"`
		EquipmentTypeID *string `json:"equipment_type_id"`
	} `json:"lines"`
}

func (h *SupplierHandler) GetHistoricalPrices(c *gin.Context) {
	var req SupplierPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	prices := make([]float64, len(req.Lines))

	for i, line := range req.Lines {
		price, err := h.purOUC.GetHistoricalPrice(c.Request.Context(), req.SupplierID, line.ItemID, line.EquipmentTypeID)
		if err != nil {
			// Ignore error, just return 0
			price = 0
		}
		prices[i] = price
	}

	c.JSON(http.StatusOK, gin.H{"prices": prices})
}

