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
	supplyFacade *usecase.SupplyChainFacade,
	supplierRepo services.SupplierRepository,
	eqTypeRepo services.EquipmentTypeRepository,
	orderUC usecase.OrderUseCase,
	stockRepo services.NodeStockRepository,
	configRepo services.NodeItemConfigRepository,
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
			kds.GET("/batches", kdsH.ListBatches)  // ?node_id=&status=
			kds.GET("/pool", kdsH.GetPoolStatus)   // Pool countdown for UI
		}

		// Production (BOM, SOP, Production Orders)
		prodH := newProductionHandler(productionUC, orchestrator, supplyFacade)
		prod := v1.Group("/production")
		{
			prod.GET("/boms", prodH.ListBOMs)
			prod.POST("/boms", prodH.CreateBOM)
			prod.GET("/boms/:id", prodH.GetBOMByID)
			prod.GET("/boms/by-item/:id", prodH.GetFullBOMByItem)
			prod.PUT("/boms/:id", prodH.UpdateBOM)

			prod.GET("/sops", prodH.ListSOPs)
			prod.POST("/sops", prodH.CreateSOP)
			prod.GET("/sops/by-bom/:id", prodH.GetFullSOPByBOM)
			prod.PUT("/sops/:id", prodH.UpdateSOP)

			prodOrders := prod.Group("/orders")
			{
				prodOrders.POST("", prodH.CreateOrder)
				prodOrders.GET("", prodH.ListOrders) // ?node_id=
				prodOrders.GET("/:id", prodH.GetOrder)
				prodOrders.PATCH("/:id/status", prodH.UpdateStatus)
			}
		}

		// Sale Orders (Store POS — triggers StockOut + ROP)
		orderH := newOrderHandler(orderUC, "SNAPBITE_ORG", "HQ")
		saleOrders := v1.Group("/orders")
		{
			saleOrders.POST("", orderH.Create)
			saleOrders.GET("", orderH.List)      // ?node_id=
			saleOrders.GET("/:id", orderH.GetByID)
			saleOrders.PATCH("/:id/complete", orderH.Complete)
			saleOrders.PATCH("/:id/cancel", orderH.Cancel)
		}

		// Internal Transfer Orders (ITO lifecycle)
		itoH := newITOHandler(supplyFacade.ITO)
		itos := v1.Group("/itos")
		{
			itos.POST("", itoH.Create)
			itos.GET("", itoH.List)      // ?node_id=
			itos.GET("/:id", itoH.GetByID)
			itos.PATCH("/:id/approve", itoH.Approve)
			itos.PATCH("/:id/reject", itoH.Reject)
			itos.POST("/:id/goods-issue", itoH.GoodsIssue)
			itos.POST("/:id/goods-receipt", itoH.GoodsReceipt)
		}

		// Inventory (NodeStock + NodeItemConfig)
		invH := newInventoryHandler(supplyFacade.Inventory, stockRepo, configRepo)
		v1.GET("/inventory", invH.ListStock)           // ?node_id=
		v1.POST("/inventory/init", invH.InitStock)
		v1.GET("/node-item-configs", invH.ListConfigs) // ?node_id=
		v1.PUT("/node-item-configs", invH.UpsertConfig)

		// Supply Chain: CapEx / Procurement Flow
		prH := newPRHandler(supplyFacade.PR)
		prs := v1.Group("/prs")
		{
			prs.POST("", prH.Submit)
			prs.GET("", prH.List)
			prs.GET("/:id", prH.GetByID)
			prs.PATCH("/:id/approve", prH.Approve)
			prs.PATCH("/:id/reject", prH.Reject)
		}

		puroH := newPurOHandler(supplyFacade.PurO)
		puros := v1.Group("/puros")
		{
			puros.POST("", puroH.Create)
			puros.GET("", puroH.List)
			puros.GET("/:id", puroH.GetByID)
			puros.PATCH("/:id/on-way", puroH.MarkOnWayDelivery)
			puros.PATCH("/:id/confirm", puroH.Confirm)
			puros.PATCH("/:id/confirm-draft", puroH.ConfirmDraft)
		}

		grH := newGRHandler(supplyFacade.GR)
		grs := v1.Group("/grs")
		{
			grs.POST("", grH.ConfirmPurO)
			grs.GET("/:id", grH.GetByID)
		}

		invHandler := newInvoiceHandler(supplyFacade.Invoice)
		invoices := v1.Group("/invoices")
		{
			invoices.POST("", invHandler.Record)
			invoices.GET("/:id", invHandler.GetByID)
			invoices.POST("/:id/3way-match", invHandler.Match3Way)
		}

		assetH := newAssetHandler(supplyFacade.Asset)
		assets := v1.Group("/assets")
		{
			assets.GET("", assetH.List)
			assets.GET("/:id", assetH.GetByID)
			assets.POST("/:id/register-machine", assetH.RegisterMachine)
			assets.PATCH("/:id/status", assetH.SyncStatus)
		}

		supplierH := newSupplierHandler(supplierRepo, supplyFacade.PurO)
		v1.GET("/suppliers", supplierH.List)
		v1.POST("/suppliers", supplierH.Create)
		v1.POST("/suppliers/historical-prices", supplierH.GetHistoricalPrices)

		eqTypeH := newEquipmentTypeHandler(eqTypeRepo)
		v1.GET("/equipment-types", eqTypeH.List)
		v1.POST("/equipment-types", eqTypeH.Create)
	}

	return &Router{engine: engine}
}

// Run starts the HTTP server on the given address.
func (r *Router) Run(addr string) error {
	return r.engine.Run(addr)
}
