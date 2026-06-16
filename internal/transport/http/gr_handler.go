package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type GRHandler struct {
	grService usecase.GRUseCase
}

func newGRHandler(svc usecase.GRUseCase) *GRHandler {
	return &GRHandler{grService: svc}
}

// Confirm receives a Purchase Order (HQ or Store).
func (h *GRHandler) ConfirmPurO(c *gin.Context) {
	var req struct {
		PurOID          string                  `json:"puro_id"`
		ReceivingNodeID string                  `json:"receiving_node_id"`
		StaffID         string                  `json:"staff_id"`
		Notes           string                  `json:"notes"`
		DeliveryNoteURL string                  `json:"delivery_note_url"`
		Lines           []usecase.GRLineInput   `json:"lines"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	gr, err := h.grService.ConfirmPurOGoodsReceipt(c.Request.Context(), req.PurOID, req.ReceivingNodeID, req.StaffID, req.Notes, req.DeliveryNoteURL, req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gr)
}

func (h *GRHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	gr, lines, err := h.grService.GetGR(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gr":    gr,
		"lines": lines,
	})
}

// List fetching GoodsReceipts optionally filtered by puro_id
func (h *GRHandler) List(c *gin.Context) {
	purOID := c.Query("puro_id")
	if purOID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "puro_id query param is required"})
		return
	}

	grs, err := h.grService.GetGRsByPurOID(c.Request.Context(), purOID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, grs)
}
