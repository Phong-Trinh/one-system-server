package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type PurOHandler struct {
	puroService usecase.PurOUseCase
}

func newPurOHandler(svc usecase.PurOUseCase) *PurOHandler {
	return &PurOHandler{puroService: svc}
}

// Create handles converting a PR to a PO (CapEx flow).
func (h *PurOHandler) Create(c *gin.Context) {
	var req struct {
		PRID               string                  `json:"pr_id"`
		SupplierID         string                  `json:"supplier_id"`
		HQNodeID           string                  `json:"hq_node_id"`
		ConfirmedByStaffID string                  `json:"confirmed_by_staff_id"`
		Lines              []usecase.PurOLineInput `json:"lines"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	purO, err := h.puroService.CreatePRTriggeredPurO(c.Request.Context(), req.PRID, req.SupplierID, req.HQNodeID, req.ConfirmedByStaffID, req.Lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, purO)
}

func (h *PurOHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	po, lines, err := h.puroService.GetPurO(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"po":    po,
		"lines": lines,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PurOHandler) List(c *gin.Context) {
	// Filter by delivery_node_id or org_id (for drafts)
	nodeID := c.Query("delivery_node_id")
	orgID := c.Query("org_id") // to list drafts for HQ

	if nodeID != "" {
		pos, err := h.puroService.ListByDeliveryNode(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, pos)
		return
	}

	if orgID != "" {
		pos, err := h.puroService.ListDrafts(c.Request.Context(), orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, pos)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide delivery_node_id or org_id"})
}

func (h *PurOHandler) MarkShipped(c *gin.Context) {
	id := c.Param("id")
	if err := h.puroService.MarkShipped(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "PO marked as shipped"})
}
