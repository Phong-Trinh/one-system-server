package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
)

type nodeHandler struct {
	uc usecase.NodeUseCase
}

func newNodeHandler(uc usecase.NodeUseCase) *nodeHandler {
	return &nodeHandler{uc: uc}
}

// POST /api/v1/nodes
func (h *nodeHandler) Create(c *gin.Context) {
	var req struct {
		OrgID   string          `json:"org_id"  binding:"required"`
		Type    models.NodeType `json:"type"    binding:"required"`
		Name    string          `json:"name"    binding:"required"`
		Address string          `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	node, err := h.uc.Create(c.Request.Context(), req.OrgID, req.Type, req.Name, req.Address)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, node)
}

// GET /api/v1/nodes/:id
func (h *nodeHandler) GetByID(c *gin.Context) {
	node, err := h.uc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// GET /api/v1/nodes?org_id=
func (h *nodeHandler) ListByOrg(c *gin.Context) {
	orgID := c.Query("org_id")
	var nodes []*models.Node
	var err error

	if orgID != "" {
		nodes, err = h.uc.ListByOrg(c.Request.Context(), orgID)
	} else {
		nodes, err = h.uc.ListAll(c.Request.Context())
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, nodes)
}

// PUT /api/v1/nodes/:id
func (h *nodeHandler) Update(c *gin.Context) {
	var req struct {
		Name    string `json:"name"    binding:"required"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	node, err := h.uc.Update(c.Request.Context(), c.Param("id"), req.Name, req.Address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, node)
}

// DELETE /api/v1/nodes/:id
func (h *nodeHandler) Delete(c *gin.Context) {
	if err := h.uc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
