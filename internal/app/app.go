package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

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

	// ── Use Cases (Application) ───────────────────────────────────────────────
	orgUC := usecase.NewOrgUseCase(orgRepo)
	nodeUC := usecase.NewNodeUseCase(nodeRepo, orgRepo)
	stationTypeUC := usecase.NewStationTypeUseCase(stationTypeRepo)
	machineUC := usecase.NewMachineUseCase(machineRepo, nodeRepo, stationTypeRepo)
	staffUC := usecase.NewStaffUseCase(staffRepo, nodeRepo)
	productionUC := usecase.NewProductionUseCase(poRepo, bomRepo, sopRepo, nodeRepo)
	itemUC := usecase.NewItemUseCase(itemRepo)
	allocationUC := usecase.NewAllocationUseCase(poRepo, batchRepo, machineRepo, sopRepo)

	// ── Transport (HTTP) ──────────────────────────────────────────────────────
	router := transport.NewRouter(orgUC, nodeUC, stationTypeUC, machineUC, staffUC, productionUC, itemUC, allocationUC)


	return &App{router: router, mongoClient: mongoClient}, nil
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
