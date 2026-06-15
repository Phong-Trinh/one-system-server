package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/services"
)

type PRHandler struct {
	prService services.PRService
}

func newPRHandler(svc services.PRService) *PRHandler {
	return &PRHandler{prService: svc}
}

func (h *PRHandler) Submit(c *gin.Context) {
	var req services.SubmitPRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	pr, err := h.prService.SubmitPR(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, pr)
}

func (h *PRHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	pr, lines, err := h.prService.GetPR(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	
	// Create a combined DTO for the response
	resp := gin.H{
		"pr":    pr,
		"lines": lines,
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PRHandler) List(c *gin.Context) {
	// Filter by node_id or org_id
	nodeID := c.Query("node_id")
	orgID := c.Query("org_id")

	if nodeID != "" {
		prs, err := h.prService.ListPRsByNode(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, prs)
		return
	}

	if orgID != "" {
		// Just listing pending by org for HQ dashboard
		prs, err := h.prService.ListPendingByOrg(c.Request.Context(), orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, prs)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide node_id or org_id"})
}

type ReviewRequest struct {
	ReviewerStaffID string                     `json:"reviewer_staff_id"`
	Note            *string                    `json:"note"`
	// Lines carries HQ's verified corrections for each PR line.
	// Required when approving — must cover every line in the PR.
	Lines           []services.PRLineCorrection `json:"lines"`
}

func (h *PRHandler) Approve(c *gin.Context) {
	id := c.Param("id")
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if err := h.prService.ApprovePR(c.Request.Context(), id, req.ReviewerStaffID, req.Note, req.Lines); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "PR approved"})
}


func (h *PRHandler) Reject(c *gin.Context) {
	id := c.Param("id")
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	if req.Note == nil || *req.Note == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection note/reason is required"})
		return
	}

	if err := h.prService.RejectPR(c.Request.Context(), id, req.ReviewerStaffID, *req.Note); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "PR rejected"})
}
