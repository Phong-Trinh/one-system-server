package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

type itoHandler struct {
	uc usecase.ITOUseCase
}

func newITOHandler(uc usecase.ITOUseCase) *itoHandler {
	return &itoHandler{uc: uc}
}

// POST /api/v1/itos — Create a manual ITO (Store/Factory initiates)
func (h *itoHandler) Create(c *gin.Context) {
	var req struct {
		OrgID            string `json:"org_id" binding:"required"`
		RequesterNodeID  string `json:"requester_node_id" binding:"required"`
		ProviderNodeID   string `json:"provider_node_id" binding:"required"`
		StaffID          string `json:"staff_id" binding:"required"`
		Lines            []struct {
			ItemID     string  `json:"item_id" binding:"required"`
			QtyOrdered float64 `json:"qty_ordered" binding:"required"`
			PkgUnit    string  `json:"pkg_unit" binding:"required"`
			Conversion float64 `json:"conversion" binding:"required"`
		} `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	lines := make([]usecase.ITOLineInput, len(req.Lines))
	for i, l := range req.Lines {
		lines[i] = usecase.ITOLineInput{
			ItemID:     l.ItemID,
			QtyOrdered: l.QtyOrdered,
			PkgUnit:    l.PkgUnit,
			Conversion: l.Conversion,
		}
	}

	ito, err := h.uc.CreateManualITO(c.Request.Context(), req.OrgID, req.RequesterNodeID, req.ProviderNodeID, req.StaffID, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, ito)
}

// GET /api/v1/itos?node_id=
func (h *itoHandler) List(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id query param required"})
		return
	}
	itos, err := h.uc.ListITOsByNode(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, itos)
}

// GET /api/v1/itos/:id
func (h *itoHandler) GetByID(c *gin.Context) {
	ito, lines, err := h.uc.GetITO(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ito": ito, "lines": lines})
}

// PATCH /api/v1/itos/:id/approve
func (h *itoHandler) Approve(c *gin.Context) {
	if err := h.uc.ApproveManualITO(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// PATCH /api/v1/itos/:id/reject
func (h *itoHandler) Reject(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.uc.RejectManualITO(c.Request.Context(), c.Param("id"), req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// POST /api/v1/itos/:id/goods-issue — Provider dispatches goods
func (h *itoHandler) GoodsIssue(c *gin.Context) {
	var req struct {
		DriverName   string `json:"driver_name"`
		DriverPhone  string `json:"driver_phone"`
		VehiclePlate string `json:"vehicle_plate"`
		MediaURL     string `json:"media_url"`
		ShippingFee  float64 `json:"shipping_fee"`
		Lines        []struct {
			ItemID    string  `json:"item_id" binding:"required"`
			QtyIssued float64 `json:"qty_issued" binding:"required"`
		} `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	giLines := make([]usecase.GILineInput, len(req.Lines))
	for i, l := range req.Lines {
		giLines[i] = usecase.GILineInput{
			ItemID:    l.ItemID,
			QtyIssued: l.QtyIssued,
		}
	}

	gi, err := h.uc.ConfirmGoodsIssue(c.Request.Context(), c.Param("id"), usecase.GoodsIssueInput{
		DriverName:   req.DriverName,
		DriverPhone:  req.DriverPhone,
		VehiclePlate: req.VehiclePlate,
		MediaURL:     req.MediaURL,
		ShippingFee:  req.ShippingFee,
		Lines:        giLines,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gi)
}

// POST /api/v1/itos/:id/goods-receipt — Requester confirms receipt
func (h *itoHandler) GoodsReceipt(c *gin.Context) {
	var req struct {
		GoodsIssueID      string `json:"goods_issue_id" binding:"required"`
		ReceivedByStaffID string `json:"received_by_staff_id" binding:"required"`
		Notes             string `json:"notes"`
		DeliveryNoteURL   string `json:"delivery_note_url"`
		Lines             []struct {
			ItemID      string  `json:"item_id" binding:"required"`
			QtyExpected float64 `json:"qty_expected" binding:"required"`
			QtyReceived float64 `json:"qty_received"` // Can be 0 if all missing/damaged
			QtyDamaged  float64 `json:"qty_damaged"`
			Reason      string  `json:"reason"`
			EvidenceURL string  `json:"evidence_url"`
		} `json:"lines" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	grLines := make([]usecase.GRLineInput, len(req.Lines))
	for i, l := range req.Lines {
		grLines[i] = usecase.GRLineInput{
			ItemID:      l.ItemID,
			QtyExpected: l.QtyExpected,
			QtyReceived: l.QtyReceived,
			QtyDamaged:  l.QtyDamaged,
			Reason:      l.Reason,
			EvidenceURL: l.EvidenceURL,
		}
	}

	gr, err := h.uc.ConfirmGoodsReceipt(c.Request.Context(), c.Param("id"), req.GoodsIssueID, usecase.GoodsReceiptInput{
		ReceivedByStaffID: req.ReceivedByStaffID,
		Notes:             req.Notes,
		DeliveryNoteURL:   req.DeliveryNoteURL,
		Lines:             grLines,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gr)
}
