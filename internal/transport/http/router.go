package http

import (
	"github.com/gin-gonic/gin"

	"one-system-server/internal/usecase"
)

// Router wires all Layer 1 HTTP routes.
type Router struct {
	engine *gin.Engine
}

// NewRouter creates the Gin engine and registers all Layer 1 routes.
func NewRouter(
	orgUC usecase.OrgUseCase,
	nodeUC usecase.NodeUseCase,
	stationTypeUC usecase.StationTypeUseCase,
	machineUC usecase.MachineUseCase,
	staffUC usecase.StaffUseCase,
) *Router {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := engine.Group("/api/v1")
	{
		// Organizations
		orgH := newOrgHandler(orgUC)
		orgs := v1.Group("/orgs")
		{
			orgs.POST("", orgH.Create)
			orgs.GET("", orgH.List)
			orgs.GET("/:id", orgH.GetByID)
			orgs.PUT("/:id", orgH.Update)
			orgs.DELETE("/:id", orgH.Delete)
		}

		// Nodes
		nodeH := newNodeHandler(nodeUC)
		nodes := v1.Group("/nodes")
		{
			nodes.POST("", nodeH.Create)
			nodes.GET("/:id", nodeH.GetByID)
			nodes.GET("", nodeH.ListByOrg) // ?org_id=
			nodes.PUT("/:id", nodeH.Update)
			nodes.DELETE("/:id", nodeH.Delete)
		}

		// Station Types
		stH := newStationTypeHandler(stationTypeUC)
		stTypes := v1.Group("/station-types")
		{
			stTypes.POST("", stH.Create)
			stTypes.GET("", stH.List)
			stTypes.GET("/:id", stH.GetByID)
			stTypes.PUT("/:id", stH.Update)
			stTypes.DELETE("/:id", stH.Delete)
		}

		// Machines
		machineH := newMachineHandler(machineUC)
		machines := v1.Group("/machines")
		{
			machines.POST("", machineH.Create)
			machines.GET("/:id", machineH.GetByID)
			machines.GET("", machineH.ListByNode) // ?node_id=
			machines.PUT("/:id", machineH.Update)
			machines.DELETE("/:id", machineH.Delete)
		}

		// Staff
		staffH := newStaffHandler(staffUC)
		staff := v1.Group("/staff")
		{
			staff.POST("", staffH.Create)
			staff.GET("/:id", staffH.GetByID)
			staff.GET("", staffH.ListByNode) // ?node_id=
			staff.PUT("/:id", staffH.Update)
			staff.DELETE("/:id", staffH.Delete)
		}
	}

	return &Router{engine: engine}
}

// Run starts the HTTP server on the given address.
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
