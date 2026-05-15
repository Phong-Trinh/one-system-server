package http

import (
	"github.com/gin-gonic/gin"

	"one-system-server/internal/domain/services"
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
	productionUC usecase.ProductionUseCase,
	itemUC usecase.ItemUseCase,
	allocationUC usecase.AllocationUseCase,
	batchRepo services.ProductionBatchRepository,
	sopRepo services.SOPRepository,
	orchestrator *usecase.OrderPoolingOrchestrator,
) *Router {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	// Middleware CORS
	engine.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := engine.Group("/api/v1")
	{
		// Items
		itemH := newItemHandler(itemUC)
		items := v1.Group("/items")
		{
			items.POST("", itemH.Create)
			items.GET("", itemH.List)
			items.GET("/:id", itemH.GetByID)
			items.PUT("/:id", itemH.Update)
			items.DELETE("/:id", itemH.Delete)
		}

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

		// KDS (Kitchen Display System) — Command & Confirm Flow
		kdsH := newKDSHandler(allocationUC, batchRepo, sopRepo, orchestrator)
		kds := v1.Group("/kds")
		{
			kds.POST("/batches/:id/confirm-placement", kdsH.ConfirmPlacement)
			kds.POST("/batches/:id/confirm-completion", kdsH.ConfirmCompletion)
			kds.GET("/batches", kdsH.ListBatches)   // ?node_id=&status=
			kds.GET("/pool", kdsH.GetPoolStatus)    // Pool countdown for UI
		}

		// Production (BOM, SOP, Orders)
		prodH := newProductionHandler(productionUC, orchestrator)
		prod := v1.Group("/production")
		{
			prod.POST("/boms", prodH.CreateBOM)
			prod.GET("/boms/by-item/:id", prodH.GetFullBOMByItem)
			prod.PUT("/boms/:id", prodH.UpdateBOM)

			prod.POST("/sops", prodH.CreateSOP)
			prod.GET("/sops/by-bom/:id", prodH.GetFullSOPByBOM)
			prod.PUT("/sops/:id", prodH.UpdateSOP)

			orders := prod.Group("/orders")
			{
				orders.POST("", prodH.CreateOrder)
				orders.GET("", prodH.ListOrders) // ?node_id=
				orders.GET("/:id", prodH.GetOrder)
				orders.PATCH("/:id/status", prodH.UpdateStatus)
			}
		}
	}

	return &Router{engine: engine}
}

// Run starts the HTTP server on the given address.
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
