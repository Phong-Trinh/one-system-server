// package main

// import (
// 	"context"
// 	"fmt"
// 	"one-system-server/internal/domain/models"
// 	"one-system-server/internal/usecase"
// )

// // --- Mock PORepository ---
// type mockPORepo struct {
// 	pos map[string]*models.ProductionOrder
// }
// func (m *mockPORepo) FindByID(ctx context.Context, id string) (*models.ProductionOrder, error) { return m.pos[id], nil }
// func (m *mockPORepo) Create(ctx context.Context, po *models.ProductionOrder) error { m.pos[po.ID] = po; return nil }
// func (m *mockPORepo) FindByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error) { return nil, nil }
// func (m *mockPORepo) FindByStatus(ctx context.Context, s models.POStatus) ([]*models.ProductionOrder, error) { return nil, nil }
// func (m *mockPORepo) FindAll(ctx context.Context) ([]*models.ProductionOrder, error) { return nil, nil }
// func (m *mockPORepo) UpdateStatus(ctx context.Context, id string, s models.POStatus, out *float64) error { return nil }
// func (m *mockPORepo) Update(ctx context.Context, po *models.ProductionOrder) error { m.pos[po.ID] = po; return nil }
// func (m *mockPORepo) Delete(ctx context.Context, id string) error { delete(m.pos, id); return nil }
// func (m *mockPORepo) SaveSnapshot(ctx context.Context, snap *models.BOMSnapshot) error { return nil }
// func (m *mockPORepo) GetSnapshot(ctx context.Context, poID string) (*models.BOMSnapshot, error) { return nil, nil }
// func (m *mockPORepo) AssignStaff(ctx context.Context, ass *models.POStaffAssignment) error { return nil }
// func (m *mockPORepo) ListStaffAssignments(ctx context.Context, poID string) ([]*models.POStaffAssignment, error) { return nil, nil }

// // --- Mock BatchRepository ---
// type mockBatchRepo struct {
// 	batches map[string]*models.ProductionBatch
// }
// func (m *mockBatchRepo) Create(ctx context.Context, b *models.ProductionBatch) error {
// 	m.batches[b.ID] = b
// 	fmt.Printf("[DB] Created Batch: %s (Step: %s, Status: %s)\n", b.ID[:8], b.SOPStepID, b.Status)
// 	return nil
// }
// func (m *mockBatchRepo) FindByID(ctx context.Context, id string) (*models.ProductionBatch, error) { return m.batches[id], nil }
// func (m *mockBatchRepo) FindByNode(ctx context.Context, nodeID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
// 	var res []*models.ProductionBatch
// 	for _, b := range m.batches {
// 		if b.NodeID == nodeID {
// 			if len(statuses) == 0 {
// 				res = append(res, b)
// 				continue
// 			}
// 			for _, s := range statuses {
// 				if b.Status == s {
// 					res = append(res, b)
// 					break
// 				}
// 			}
// 		}
// 	}
// 	return res, nil
// }
// func (m *mockBatchRepo) FindByMachine(ctx context.Context, machineID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
// 	var res []*models.ProductionBatch
// 	for _, b := range m.batches {
// 		if b.MachineID == machineID {
// 			for _, s := range statuses {
// 				if b.Status == s {
// 					res = append(res, b)
// 					break
// 				}
// 			}
// 		}
// 	}
// 	return res, nil
// }
// func (m *mockBatchRepo) UpdateStatus(ctx context.Context, id string, s models.BatchStatus) error {
// 	m.batches[id].Status = s
// 	fmt.Printf("[DB] Status Change -> Batch %s (%s): %s\n", id[:8], m.batches[id].SOPStepID, s)
// 	return nil
// }
// func (m *mockBatchRepo) Update(ctx context.Context, b *models.ProductionBatch) error {
// 	m.batches[b.ID] = b
// 	fmt.Printf("[DB] Updated Batch %s: Machine=%s, Status=%s\n", b.ID[:8], b.MachineID, b.Status)
// 	return nil
// }
// func (m *mockBatchRepo) Delete(ctx context.Context, id string) error { delete(m.batches, id); return nil }

// // --- Mock MachineRepository ---
// type mockMachineRepo struct {
// 	machines map[string]*models.Machine
// }
// func (m *mockMachineRepo) Create(ctx context.Context, mach *models.Machine) error { m.machines[mach.ID] = mach; return nil }
// func (m *mockMachineRepo) FindByID(ctx context.Context, id string) (*models.Machine, error) { return m.machines[id], nil }
// func (m *mockMachineRepo) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error) {
// 	var res []*models.Machine
// 	for _, mach := range m.machines {
// 		if mach.NodeID == nodeID {
// 			res = append(res, mach)
// 		}
// 	}
// 	return res, nil
// }
// func (m *mockMachineRepo) FindAll(ctx context.Context) ([]*models.Machine, error) { return nil, nil }
// func (m *mockMachineRepo) FindIdleByStationType(ctx context.Context, nid, stid string) ([]*models.Machine, error) { return nil, nil }
// func (m *mockMachineRepo) UpdateStatus(ctx context.Context, id string, s models.MachineStatus, bid *string) error { return nil }
// func (m *mockMachineRepo) Update(ctx context.Context, mach *models.Machine) error { m.machines[mach.ID] = mach; return nil }
// func (m *mockMachineRepo) Delete(ctx context.Context, id string) error { delete(m.machines, id); return nil }

// // --- Mock SOPRepository ---
// type mockSOPRepo struct {
// 	steps map[string][]*models.SOPStep
// }
// func (m *mockSOPRepo) Create(ctx context.Context, sop *models.SOP) error { return nil }
// func (m *mockSOPRepo) FindByID(ctx context.Context, id string) (*models.SOP, error) { return nil, nil }
// func (m *mockSOPRepo) FindByBOMID(ctx context.Context, bid string) (*models.SOP, error) { return nil, nil }
// func (m *mockSOPRepo) Update(ctx context.Context, sop *models.SOP) error { return nil }
// func (m *mockSOPRepo) Delete(ctx context.Context, id string) error { return nil }
// func (m *mockSOPRepo) AddStep(ctx context.Context, step *models.SOPStep) error { return nil }
// func (m *mockSOPRepo) FindStepByID(ctx context.Context, id string) (*models.SOPStep, error) {
// 	for _, steps := range m.steps {
// 		for _, s := range steps {
// 			if s.ID == id {
// 				return s, nil
// 			}
// 		}
// 	}
// 	return nil, nil
// }
// func (m *mockSOPRepo) ListSteps(ctx context.Context, sopID string) ([]*models.SOPStep, error) { return m.steps[sopID], nil }
// func (m *mockSOPRepo) DeleteStep(ctx context.Context, sopID, stepID string) error { return nil }

// func main() {
// 	ctx := context.Background()
// 	poRepo := &mockPORepo{pos: make(map[string]*models.ProductionOrder)}
// 	batchRepo := &mockBatchRepo{batches: make(map[string]*models.ProductionBatch)}
// 	machineRepo := &mockMachineRepo{machines: make(map[string]*models.Machine)}
// 	sopRepo := &mockSOPRepo{steps: make(map[string][]*models.SOPStep)}

// 	// 1. SETUP DATA
// 	nodeID := "STORE_001"
// 	sopID := "SOP_BURGER"

// 	machineRepo.machines["M1_GRILL"] = &models.Machine{ID: "M1_GRILL", NodeID: nodeID, StationTypeID: "ST_GRILL", AllocationStrategy: models.StrategyAsync, MaxCapacity: 8, OperationalThreshold: 6}
// 	machineRepo.machines["M2_ASSY"] = &models.Machine{ID: "M2_ASSY", NodeID: nodeID, StationTypeID: "ST_ASSY", AllocationStrategy: models.StrategyAsync, MaxCapacity: 2, OperationalThreshold: 2}

// 	sopRepo.steps[sopID] = []*models.SOPStep{
// 		{ID: "STEP_BEEF", SOPID: sopID, StationTypeID: "ST_GRILL", Duration: 10, Description: "Grill Beef"},
// 		{ID: "STEP_BUN", SOPID: sopID, StationTypeID: "ST_GRILL", Duration: 5, Description: "Grill Bun"},
// 		{ID: "STEP_ASSY", SOPID: sopID, StationTypeID: "ST_ASSY", Duration: 15, Description: "Assemble", DependsOn: []string{"STEP_BEEF", "STEP_BUN"}},
// 	}

// 	poID := "PO_123"
// 	poRepo.pos[poID] = &models.ProductionOrder{ID: poID, SOPID: sopID, NodeID: nodeID, Status: models.POPending}

// 	engine := usecase.NewAllocationUseCase(poRepo, batchRepo, machineRepo, sopRepo)

// 	fmt.Println("--- PHASE 1: DECOMPOSE PO ---")
// 	fmt.Println("Hành động: Chia nhỏ PO thành các bước ban đầu (Thịt & Bánh)")
// 	_ = engine.DecomposePO(ctx, poID)

// 	fmt.Println("\n--- PHASE 2: CONFIRM BEEF COMPLETED ---")
// 	fmt.Println("Hành động: Thịt đã chín. Kiểm tra xem bước Assemble có được kích hoạt chưa...")
// 	var beefBatchID string
// 	for id, b := range batchRepo.batches {
// 		if b.SOPStepID == "STEP_BEEF" { beefBatchID = id }
// 	}
// 	_ = engine.ConfirmCompletion(ctx, beefBatchID)

// 	fmt.Println("\n--- PHASE 3: CONFIRM BUN COMPLETED ---")
// 	fmt.Println("Hành động: Bánh đã chín. Bước Assemble sẽ được kích hoạt vì đã đủ 2 điều kiện (Thịt + Bánh)...")
// 	var bunBatchID string
// 	for id, b := range batchRepo.batches {
// 		if b.SOPStepID == "STEP_BUN" { bunBatchID = id }
// 	}
// 	_ = engine.ConfirmCompletion(ctx, bunBatchID)

// 	fmt.Println("\n--- TRẠNG THÁI CUỐI CÙNG ---")
// 	for _, b := range batchRepo.batches {
// 		fmt.Printf("Batch %s (%s): %s trên Máy %s\n", b.ID[:8], b.SOPStepID, b.Status, b.MachineID)
// 	}
// }