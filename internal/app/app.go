package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
	mongorepo "one-system-server/internal/infrastructure/persistence/mongodb"
	transport "one-system-server/internal/transport/http"
	"one-system-server/internal/usecase"
)

// App holds the live server and its dependencies.
type App struct {
	router      *transport.Router
	mongoClient *mongorepo.Client
}

// New builds and wires the full application.
func New(ctx context.Context) (*App, error) {
	// ── Config ────────────────────────────────────────────────────────────────
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AutomaticEnv() // env vars override file values

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("app: load config: %w", err)
	}

	uri := viper.GetString("mongodb.uri")
	dbName := viper.GetString("mongodb.database")
	if uri == "" || dbName == "" {
		return nil, fmt.Errorf("app: mongodb.uri and mongodb.database must be set in configs/config.yaml")
	}

	// ── MongoDB ───────────────────────────────────────────────────────────────
	mongoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	mongoClient, err := mongorepo.NewClient(mongoCtx, uri)
	if err != nil {
		return nil, fmt.Errorf("app: mongodb: %w", err)
	}
	log.Info().Str("database", dbName).Msg("connected to MongoDB Atlas")

	// ── Repositories (Infrastructure) ─────────────────────────────────────────
	orgRepo := mongorepo.NewOrgRepository(mongoClient, dbName)
	nodeRepo := mongorepo.NewNodeRepository(mongoClient, dbName)
	stationTypeRepo := mongorepo.NewStationTypeRepository(mongoClient, dbName)
	machineRepo := mongorepo.NewMachineRepository(mongoClient, dbName)
	staffRepo := mongorepo.NewStaffRepository(mongoClient, dbName)

	bomRepo := mongorepo.NewBOMRepository(mongoClient, dbName)
	sopRepo := mongorepo.NewSOPRepository(mongoClient, dbName)
	poRepo := mongorepo.NewProductionOrderRepository(mongoClient, dbName)
	batchRepo := mongorepo.NewProductionBatchRepository(mongoClient, dbName)
	itemRepo := mongorepo.NewItemRepository(mongoClient, dbName)
	shiftRepo := mongorepo.NewStaffShiftRepository(mongoClient, dbName)
	taskRepo := mongorepo.NewStaffTaskRepository(mongoClient, dbName)

	nodeItemConfigRepo := mongorepo.NewNodeItemConfigRepository(mongoClient, dbName)

	// Supply Chain Repositories
	stockRepo := mongorepo.NewNodeStockRepository(mongoClient, dbName)
	supplierRepo := mongorepo.NewSupplierRepository(mongoClient, dbName)
	itoRepo := mongorepo.NewInternalTransferOrderRepository(mongoClient, dbName)
	itoLineRepo := mongorepo.NewITOLineRepository(mongoClient, dbName)
	prRepo := mongorepo.NewPurchaseRequisitionRepository(mongoClient, dbName)
	prLineRepo := mongorepo.NewPRLineRepository(mongoClient, dbName)
	puroRepo := mongorepo.NewPurchaseOrderRepository(mongoClient, dbName)
	puroLineRepo := mongorepo.NewPurchaseOrderLineRepository(mongoClient, dbName)
	giRepo := mongorepo.NewGoodsIssueRepository(mongoClient, dbName)
	giLineRepo := mongorepo.NewGoodsIssueLineRepository(mongoClient, dbName)
	grRepo := mongorepo.NewGoodsReceiptRepository(mongoClient, dbName)
	grLineRepo := mongorepo.NewGoodsReceiptLineRepository(mongoClient, dbName)
	dtRepo := mongorepo.NewDiscrepancyTicketRepository(mongoClient, dbName)
	invoiceRepo := mongorepo.NewSupplierInvoiceRepository(mongoClient, dbName)
	invoiceLineRepo := mongorepo.NewSupplierInvoiceLineRepository(mongoClient, dbName)
	txRepo := mongorepo.NewTransactionRepository(mongoClient, dbName)
	b2bRepo := mongorepo.NewB2BSalesOrderRepository(mongoClient, dbName)
	b2bLineRepo := mongorepo.NewB2BSalesOrderLineRepository(mongoClient, dbName)
	assetRepo := mongorepo.NewAssetRepository(mongoClient, dbName)
	eqTypeRepo := mongorepo.NewEquipmentTypeRepository(mongoClient, dbName)

	// ── Use Cases (Application) ───────────────────────────────────────────────
	orgUC := usecase.NewOrgUseCase(orgRepo)
	nodeUC := usecase.NewNodeUseCase(nodeRepo, orgRepo)
	stationTypeUC := usecase.NewStationTypeUseCase(stationTypeRepo)
	machineUC := usecase.NewMachineUseCase(machineRepo, nodeRepo, stationTypeRepo)
	staffUC := usecase.NewStaffUseCase(staffRepo, nodeRepo)
	productionUC := usecase.NewProductionUseCase(poRepo, bomRepo, sopRepo, nodeRepo)
	itemUC := usecase.NewItemUseCase(itemRepo)
	allocationUC := usecase.NewAllocationUseCase(poRepo, batchRepo, machineRepo, sopRepo)

	dispatcher := usecase.NewDispatcher(shiftRepo, machineRepo, batchRepo, taskRepo, sopRepo)
	schedulingEngine := usecase.NewSchedulingEngine(poRepo, sopRepo, taskRepo, machineRepo, dispatcher)
	allocationUC.SetSchedulingEngine(schedulingEngine, dispatcher)

	// ── Orchestrator (Auto-Decomposition Engine) ─────────────────────────────
	orchestratorCfg := usecase.DefaultOrchestratorConfig()
	orchestrator := usecase.NewOrderPoolingOrchestrator(allocationUC, poRepo, orchestratorCfg)
	orchestrator.Start(ctx) // Starts background flush goroutine

	// ── Supply Chain Facade ───────────────────────────────────────────────────
	supplyRepos := usecase.SupplyChainRepos{
		Stock:         stockRepo,
		Config:        nodeItemConfigRepo,
		Supplier:      supplierRepo,
		ITO:           itoRepo,
		ITOLine:       itoLineRepo,
		PR:            prRepo,
		PRLine:        prLineRepo,
		PurO:          puroRepo,
		PurOLine:      puroLineRepo,
		GI:            giRepo,
		GILine:        giLineRepo,
		GR:            grRepo,
		GRLine:        grLineRepo,
		DT:            dtRepo,
		Invoice:       invoiceRepo,
		InvoiceLine:   invoiceLineRepo,
		Transaction:   txRepo,
		B2BOrder:      b2bRepo,
		B2BOrderLine:  b2bLineRepo,
		Asset:         assetRepo,
		Machine:       machineRepo,
		Node:          nodeRepo,
		EquipmentType: eqTypeRepo,
	}
	supplyFacade := usecase.NewSupplyChainFacade(supplyRepos)

	// ── Order UseCase (Sale Orders at Store — triggers stock-out + ROP) ───────
	orderRepo := mongorepo.NewOrderRepository(mongoClient, dbName)
	orderUC := usecase.NewOrderUseCase(orderRepo, supplyFacade)

	// ── Late-bind ProductionUseCase into SupplyChainFacade ────────────────────
	// This enables auto-PO creation at Factory when Store ITO is triggered.
	supplyFacade.SetProductionUseCase(productionUC, orchestrator)
	// Wire the facade to the allocationUC so it can do stock deduction
	allocationUC.SetFacade(supplyFacade)

	// ── Transport (HTTP) ──────────────────────────────────────────────────────
	router := transport.NewRouter(
		orgUC, nodeUC, stationTypeUC, machineUC, staffUC, productionUC, itemUC, allocationUC,
		batchRepo, sopRepo, orchestrator,
		supplyFacade, supplierRepo, eqTypeRepo,
		orderUC, stockRepo, nodeItemConfigRepo,
	)

	a := &App{router: router, mongoClient: mongoClient}

	// ── Seed Mock Data ────────────────────────────────────────────────────────
	if err := a.seedData(ctx, orgRepo, nodeRepo, staffRepo); err != nil {
		log.Error().Err(err).Msg("failed to seed initial data")
	}

	// ── Seed Kitchen Demo Data (Station types, Machines, Items, BOMs, SOPs) ──
	if err := SeedKitchenData(ctx, stationTypeRepo, machineRepo, itemRepo, bomRepo, sopRepo, nodeItemConfigRepo, supplierRepo, stockRepo); err != nil {
		log.Error().Err(err).Msg("failed to seed kitchen demo data")
	}

	return a, nil
}

func (a *App) seedData(ctx context.Context, orgRepo services.OrgRepository, nodeRepo services.NodeRepository, staffRepo services.StaffRepository) error {
	orgId := "SNAPBITE_ORG"
	siteId := "SITE_1"
	siteId2 := "SITE_Q1"

	// 1. Ensure Org exists
	existingOrg, _ := orgRepo.FindByID(ctx, orgId)
	if existingOrg == nil {
		log.Info().Msg("Seeding SnapBite Organization...")
		_ = orgRepo.Create(ctx, &models.Organization{
			ID:   orgId,
			Name: "SnapBite Chain",
		})
	}

	// 2. Ensure Nodes exist (1 HQ, 1 Factory, 1 Store at the same site, 1 Store at a different site)
	nodes := []models.Node{
		{ID: "HQ", OrgID: orgId, Type: models.NodeHQ, Name: "Headquarters", Address: "123 Main St", SiteID: &siteId},
		{ID: "FACTORY", OrgID: orgId, Type: models.NodeFactory, Name: "Factory", Address: "123 Main St", SiteID: &siteId},
		{ID: "STORE", OrgID: orgId, Type: models.NodeStore, Name: "Store #1", Address: "123 Main St", SiteID: &siteId},
		{ID: "STORE2", OrgID: orgId, Type: models.NodeStore, Name: "S#2 chi nhánh quận 1", Address: "Quận 1", SiteID: &siteId2},
	}

	for _, n := range nodes {
		existingNode, _ := nodeRepo.FindByID(ctx, n.ID)
		if existingNode == nil {
			log.Info().Str("node_id", n.ID).Msg("Seeding Node...")
			n.CreatedAt = time.Now()
			n.UpdatedAt = time.Now()
			_ = nodeRepo.Create(ctx, &n)
		}
	}

	// 3. Ensure Admin Staff exists for HQ
	adminStaffId := "staff_hq_admin"
	existingStaff, _ := staffRepo.FindByID(ctx, adminStaffId)
	if existingStaff == nil {
		log.Info().Msg("Seeding Admin Staff...")
		_ = staffRepo.Create(ctx, &models.Staff{
			ID:       adminStaffId,
			NodeID:   "HQ",
			Name:     "System Admin",
			WageRate: 0,
		})
	}

	return nil
}

// Run starts the HTTP server.
func (a *App) Run() error {
	port := viper.GetString("server.port")
	if port == "" {
		port = "8080"
	}
	log.Info().Str("port", port).Msg("starting HTTP server")
	return a.router.Run(":" + port)
}

// Close releases all resources.
func (a *App) Close() {
	if err := a.mongoClient.Disconnect(context.Background()); err != nil {
		log.Error().Err(err).Msg("error disconnecting MongoDB")
	}
}
