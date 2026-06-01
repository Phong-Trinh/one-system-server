package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type InvoiceHandler struct {
	invoiceService usecase.InvoiceUseCase
}

func newInvoiceHandler(svc usecase.InvoiceUseCase) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: svc}
}

func (h *InvoiceHandler) Record(c *gin.Context) {
	var req struct {
		OrgID         string                     `json:"org_id"`
		PurOID        string                     `json:"puro_id"`
		SupplierID    string                     `json:"supplier_id"`
		InvoiceNumber string                     `json:"invoice_number"`
		TotalAmount   float64                    `json:"total_amount"`
		TaxAmount     float64                    `json:"tax_amount"`
		ImageURL      string                     `json:"image_url"`
		Lines         []usecase.InvoiceLineInput `json:"lines"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	inv, err := h.invoiceService.RecordInvoice(c.Request.Context(), req.OrgID, req.PurOID, req.SupplierID, req.InvoiceNumber, req.TotalAmount, req.TaxAmount, req.ImageURL, req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, inv)
}

func (h *InvoiceHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	inv, lines, err := h.invoiceService.GetInvoice(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invoice": inv,
		"lines":   lines,
	})
}

// 3-Way Match
func (h *InvoiceHandler) Match3Way(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		GRID             string `json:"gr_id"`
		MatchedByStaffID string `json:"matched_by_staff_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	asset, err := h.invoiceService.PerformThreeWayMatch(c.Request.Context(), id, req.GRID, req.MatchedByStaffID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "3-Way Match successful",
		"asset":   asset, // may be null for OpEx
	})
}
