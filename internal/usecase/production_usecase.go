package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
)

// ── Interface ─────────────────────────────────────────────────────────────────

type ProductionUseCase interface {
	// CreateProductionOrder initiates a new PO by snapshotting the BOM/SOP.
	CreateProductionOrder(ctx context.Context, bomID, nodeID string, targetQty float64) (*models.ProductionOrder, error)
	GetProductionOrder(ctx context.Context, id string) (*models.ProductionOrder, error)
	ListProductionOrdersByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error)
	ListAllOrders(ctx context.Context) ([]*models.ProductionOrder, error)
	UpdatePOStatus(ctx context.Context, id string, status models.POStatus, actualOutput *float64) error

	// BOM & SOP Management
	CreateBOM(ctx context.Context, outputItemID string, lines []*models.BOMLine) (*models.BOM, error)
	GetBOMByID(ctx context.Context, bomID string) (*models.BOM, []*models.BOMLine, error)
	GetFullBOMByItem(ctx context.Context, itemID string) (*models.BOM, []*models.BOMLine, error)
	ListBOMs(ctx context.Context) ([]*models.BOM, error)
	UpdateBOM(ctx context.Context, bomID string, lines []*models.BOMLine) error

	CreateSOP(ctx context.Context, bomID string, steps []*models.SOPStep) (*models.SOP, error)
	GetFullSOPByBOM(ctx context.Context, bomID string) (*models.SOP, []*models.SOPStep, error)
	ListSOPs(ctx context.Context) ([]*models.SOP, error)
	UpdateSOP(ctx context.Context, sopID string, steps []*models.SOPStep) error
}

// ── Implementation ────────────────────────────────────────────────────────────

type productionUseCase struct {
	poRepo   services.ProductionOrderRepository
	bomRepo  services.BOMRepository
	sopRepo  services.SOPRepository
	nodeRepo services.NodeRepository
}

func NewProductionUseCase(
	poRepo services.ProductionOrderRepository,
	bomRepo services.BOMRepository,
	sopRepo services.SOPRepository,
	nodeRepo services.NodeRepository,
) ProductionUseCase {
	return &productionUseCase{
		poRepo:   poRepo,
		bomRepo:  bomRepo,
		sopRepo:  sopRepo,
		nodeRepo: nodeRepo,
	}
}

func (uc *productionUseCase) CreateProductionOrder(ctx context.Context, bomID, nodeID string, targetQty float64) (*models.ProductionOrder, error) {
	// 1. Validate BOM exists
	bom, err := uc.bomRepo.FindByID(ctx, bomID)
	if err != nil {
		return nil, err
	}
	if bom == nil {
		return nil, fmt.Errorf("BOM %q not found", bomID)
	}

	// 2. Validate Node exists
	node, err := uc.nodeRepo.FindByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found", nodeID)
	}

	// 3. Find associated SOP
	sop, err := uc.sopRepo.FindByBOMID(ctx, bomID)
	if err != nil {
		return nil, err
	}
	if sop == nil {
		return nil, fmt.Errorf("no SOP found for BOM %q", bomID)
	}

	// 4. Calculate Yield (default to 1.0 if not specified in a more complex config)
	yieldRate := 1.0 // This could be pulled from Item-specific config in the future
	plannedInput := targetQty / yieldRate

	// 5. Create the Production Order
	po := &models.ProductionOrder{
		ID:           uuid.NewString(),
		ItemID:       bom.OutputItemID,
		BOMID:        bomID,
		SOPID:        sop.ID,
		NodeID:       nodeID,
		TargetQty:    targetQty,
		YieldRate:    yieldRate,
		PlannedInput: plannedInput,
		Status:       models.POPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := uc.poRepo.Create(ctx, po); err != nil {
		return nil, err
	}

	// 6. Snapshot the BOM (Crucial step for audit/costing)
	lines, err := uc.bomRepo.ListLines(ctx, bomID)
	if err != nil {
		return nil, fmt.Errorf("failed to list BOM lines for snapshot: %w", err)
	}

	snapshotData := struct {
		BOM      *models.BOM       `json:"bom"`
		BOMLines []*models.BOMLine `json:"lines"`
	}{
		BOM:      bom,
		BOMLines: lines,
	}

	jsonData, _ := json.Marshal(snapshotData)
	snap := &models.BOMSnapshot{
		POID:             po.ID,
		LockedBOMVersion: bom.Version,
		SnapshotData:     jsonData,
	}

	if err := uc.poRepo.SaveSnapshot(ctx, snap); err != nil {
		return nil, err
	}

	return po, nil
}

func (uc *productionUseCase) GetProductionOrder(ctx context.Context, id string) (*models.ProductionOrder, error) {
	po, err := uc.poRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if po == nil {
		return nil, fmt.Errorf("production order %q not found", id)
	}
	return po, nil
}

func (uc *productionUseCase) ListProductionOrdersByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error) {
	return uc.poRepo.FindByNode(ctx, nodeID)
}

func (uc *productionUseCase) ListAllOrders(ctx context.Context) ([]*models.ProductionOrder, error) {
	return uc.poRepo.FindAll(ctx)
}

func (uc *productionUseCase) UpdatePOStatus(ctx context.Context, id string, status models.POStatus, actualOutput *float64) error {
	return uc.poRepo.UpdateStatus(ctx, id, status, actualOutput)
}

// ── BOM & SOP Helpers ─────────────────────────────────────────────────────────

func (uc *productionUseCase) CreateBOM(ctx context.Context, outputItemID string, lines []*models.BOMLine) (*models.BOM, error) {
	bom := &models.BOM{
		ID:           uuid.NewString(),
		OutputItemID: outputItemID,
		Version:      1,
	}

	if err := uc.bomRepo.Create(ctx, bom); err != nil {
		return nil, err
	}

	for _, line := range lines {
		line.ID = uuid.NewString()
		line.BOMID = bom.ID
		if err := uc.bomRepo.AddLine(ctx, line); err != nil {
			return nil, err
		}
	}

	return bom, nil
}

func (uc *productionUseCase) GetBOMByID(ctx context.Context, bomID string) (*models.BOM, []*models.BOMLine, error) {
	bom, err := uc.bomRepo.FindByID(ctx, bomID)
	if err != nil {
		return nil, nil, err
	}
	if bom == nil {
		return nil, nil, fmt.Errorf("BOM %q not found", bomID)
	}
	lines, err := uc.bomRepo.ListLines(ctx, bom.ID)
	return bom, lines, err
}

func (uc *productionUseCase) GetFullBOMByItem(ctx context.Context, itemID string) (*models.BOM, []*models.BOMLine, error) {
	boms, err := uc.bomRepo.FindByOutputItem(ctx, itemID)
	if err != nil || len(boms) == 0 {
		return nil, nil, err
	}

	// For now, just take the latest version
	bom := boms[len(boms)-1]
	lines, err := uc.bomRepo.ListLines(ctx, bom.ID)
	return bom, lines, err
}

func (uc *productionUseCase) ListBOMs(ctx context.Context) ([]*models.BOM, error) {
	return uc.bomRepo.FindAll(ctx)
}

func (uc *productionUseCase) ListSOPs(ctx context.Context) ([]*models.SOP, error) {
	// Collect all BOMs then get their SOPs — SOPRepo doesn't have FindAll yet.
	// This is a simple implementation; can be optimised with a direct FindAll on SOPs.
	boms, err := uc.bomRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	var sops []*models.SOP
	for _, bom := range boms {
		sop, err := uc.sopRepo.FindByBOMID(ctx, bom.ID)
		if err != nil {
			continue
		}
		if sop != nil {
			sops = append(sops, sop)
		}
	}
	return sops, nil
}

func (uc *productionUseCase) UpdateBOM(ctx context.Context, bomID string, lines []*models.BOMLine) error {
	// Simple implementation: clear existing lines and add new ones
	existingLines, err := uc.bomRepo.ListLines(ctx, bomID)
	if err != nil {
		return err
	}

	for _, l := range existingLines {
		_ = uc.bomRepo.DeleteLine(ctx, l.ID)
	}

	for _, line := range lines {
		line.ID = uuid.NewString()
		line.BOMID = bomID
		if err := uc.bomRepo.AddLine(ctx, line); err != nil {
			return err
		}
	}
	return nil
}

func validateDAG(steps []*models.SOPStep) error {
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	nodes := make(map[string]bool)

	for _, step := range steps {
		if step.ID == "" {
			return fmt.Errorf("all steps must have an ID")
		}
		nodes[step.ID] = true
		inDegree[step.ID] = 0 // Initialize
	}

	for _, step := range steps {
		for _, dep := range step.DependsOn {
			if !nodes[dep] {
				return fmt.Errorf("step %q depends on unknown step %q", step.ID, dep)
			}
			adj[dep] = append(adj[dep], step.ID)
			inDegree[step.ID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visitedCount != len(nodes) {
		return fmt.Errorf("circular dependency detected in SOP steps")
	}

	return nil
}

func (uc *productionUseCase) CreateSOP(ctx context.Context, bomID string, steps []*models.SOPStep) (*models.SOP, error) {
	if err := validateDAG(steps); err != nil {
		return nil, err
	}

	sop := &models.SOP{
		ID:    uuid.NewString(),
		BOMID: bomID,
	}

	if err := uc.sopRepo.Create(ctx, sop); err != nil {
		return nil, err
	}

	for i, step := range steps {
		step.SOPID = sop.ID
		step.SeqNo = i + 1
		if err := uc.sopRepo.AddStep(ctx, step); err != nil {
			return nil, err
		}
	}

	return sop, nil
}

func (uc *productionUseCase) GetFullSOPByBOM(ctx context.Context, bomID string) (*models.SOP, []*models.SOPStep, error) {
	sop, err := uc.sopRepo.FindByBOMID(ctx, bomID)
	if err != nil || sop == nil {
		return nil, nil, err
	}

	steps, err := uc.sopRepo.ListSteps(ctx, sop.ID)
	return sop, steps, err
}

func (uc *productionUseCase) UpdateSOP(ctx context.Context, sopID string, steps []*models.SOPStep) error {
	if err := validateDAG(steps); err != nil {
		return err
	}

	existingSteps, err := uc.sopRepo.ListSteps(ctx, sopID)
	if err != nil {
		return err
	}

	for _, s := range existingSteps {
		_ = uc.sopRepo.DeleteStep(ctx, sopID, s.ID)
	}

	for i, step := range steps {
		step.SOPID = sopID
		step.SeqNo = i + 1
		if err := uc.sopRepo.AddStep(ctx, step); err != nil {
			return err
		}
	}
	return nil
}
