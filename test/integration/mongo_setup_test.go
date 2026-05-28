package integration

import (
	"context"
	"os"
	"time"

	mongorepo "one-system-server/internal/infrastructure/persistence/mongodb"
	"one-system-server/internal/usecase"
)

// setupFacade determines whether to use the mock facade or the real MongoDB facade.
func setupFacade() *testContext {
	if os.Getenv("USE_REAL_DB") == "true" {
		return setupMongoTestFacade()
	}
	return setupTestFacade()
}

func setupMongoTestFacade() *testContext {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uri := "mongodb://localhost:27017"
	dbName := "one_system_test"

	client, err := mongorepo.NewClient(ctx, uri)
	if err != nil {
		panic("failed to connect to mongodb for tests: " + err.Error())
	}

	// Drop the database for a clean slate
	if err := client.DB(dbName).Drop(ctx); err != nil {
		panic("failed to drop test database: " + err.Error())
	}

	// Instantiate all real MongoDB repositories
	stockRepo := mongorepo.NewNodeStockRepository(client, dbName)
	configRepo := mongorepo.NewNodeItemConfigRepository(client, dbName)
	supplierRepo := mongorepo.NewSupplierRepository(client, dbName)
	itoRepo := mongorepo.NewInternalTransferOrderRepository(client, dbName)
	itoLineRepo := mongorepo.NewITOLineRepository(client, dbName)
	prRepo := mongorepo.NewPurchaseRequisitionRepository(client, dbName)
	prLineRepo := mongorepo.NewPRLineRepository(client, dbName)
	purORepo := mongorepo.NewPurchaseOrderRepository(client, dbName)
	purOLineRepo := mongorepo.NewPurchaseOrderLineRepository(client, dbName)
	giRepo := mongorepo.NewGoodsIssueRepository(client, dbName)
	giLineRepo := mongorepo.NewGoodsIssueLineRepository(client, dbName)
	grRepo := mongorepo.NewGoodsReceiptRepository(client, dbName)
	grLineRepo := mongorepo.NewGoodsReceiptLineRepository(client, dbName)
	dtRepo := mongorepo.NewDiscrepancyTicketRepository(client, dbName)
	invRepo := mongorepo.NewSupplierInvoiceRepository(client, dbName)
	invLineRepo := mongorepo.NewSupplierInvoiceLineRepository(client, dbName)
	txRepo := mongorepo.NewTransactionRepository(client, dbName)
	b2bRepo := mongorepo.NewB2BSalesOrderRepository(client, dbName)
	b2bLineRepo := mongorepo.NewB2BSalesOrderLineRepository(client, dbName)
	assetRepo := mongorepo.NewAssetRepository(client, dbName)
	
	// App-level repos
	nodeRepo := mongorepo.NewNodeRepository(client, dbName)
	eqTypeRepo := mongorepo.NewEquipmentTypeRepository(client, dbName)
	machineRepo := mongorepo.NewMachineRepository(client, dbName)

	repos := usecase.SupplyChainRepos{
		Stock:         stockRepo,
		Config:        configRepo,
		Supplier:      supplierRepo,
		ITO:           itoRepo,
		ITOLine:       itoLineRepo,
		PR:            prRepo,
		PRLine:        prLineRepo,
		PurO:          purORepo,
		PurOLine:      purOLineRepo,
		GI:            giRepo,
		GILine:        giLineRepo,
		GR:            grRepo,
		GRLine:        grLineRepo,
		DT:            dtRepo,
		Invoice:       invRepo,
		InvoiceLine:   invLineRepo,
		Transaction:   txRepo,
		B2BOrder:      b2bRepo,
		B2BOrderLine:  b2bLineRepo,
		Asset:         assetRepo,
		Machine:       machineRepo,
		Node:          nodeRepo,
		EquipmentType: eqTypeRepo,
	}

	facade := usecase.NewSupplyChainFacade(repos)

	return &testContext{
		Facade:          facade,
		PRRepo:          prRepo,
		PRLineRepo:      prLineRepo,
		PurORepo:        purORepo,
		PurOLineRepo:    purOLineRepo,
		SupplierRepo:    supplierRepo,
		GRRepo:          grRepo,
		GRLineRepo:      grLineRepo,
		GIRepo:          giRepo,
		GILineRepo:      giLineRepo,
		DTRepo:          dtRepo,
		InvoiceRepo:     invRepo,
		InvoiceLineRepo: invLineRepo,
		TxRepo:          txRepo,
		AssetRepo:       assetRepo,
		MachineRepo:     machineRepo,
		EqTypeRepo:      eqTypeRepo,
		NodeRepo:        nodeRepo,
		NodeStockRepo:   stockRepo,
		NodeItemConfigRepo: configRepo,
		ITORepo:         itoRepo,
		ITOLineRepo:     itoLineRepo,
	}
}
