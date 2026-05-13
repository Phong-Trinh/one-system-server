package main

import (
	"context"
	"fmt"
	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
	"sort"
)

// --- Mock Repositories (Đã được tối ưu thứ tự) ---
type mockPORepo struct {
	pos map[string]*models.ProductionOrder
}

func (m *mockPORepo) FindByID(ctx context.Context, id string) (*models.ProductionOrder, error) {
	return m.pos[id], nil
}
func (m *mockPORepo) Create(ctx context.Context, po *models.ProductionOrder) error {
	m.pos[po.ID] = po
	return nil
}
func (m *mockPORepo) FindByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockPORepo) FindByStatus(ctx context.Context, s models.POStatus) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockPORepo) FindAll(ctx context.Context) ([]*models.ProductionOrder, error) { return nil, nil }
func (m *mockPORepo) UpdateStatus(ctx context.Context, id string, s models.POStatus, out *float64) error {
	return nil
}
func (m *mockPORepo) Update(ctx context.Context, po *models.ProductionOrder) error {
	m.pos[po.ID] = po
	return nil
}
func (m *mockPORepo) Delete(ctx context.Context, id string) error                      { return nil }
func (m *mockPORepo) SaveSnapshot(ctx context.Context, snap *models.BOMSnapshot) error { return nil }
func (m *mockPORepo) GetSnapshot(ctx context.Context, poID string) (*models.BOMSnapshot, error) {
	return nil, nil
}
func (m *mockPORepo) AssignStaff(ctx context.Context, ass *models.POStaffAssignment) error {
	return nil
}
func (m *mockPORepo) ListStaffAssignments(ctx context.Context, poID string) ([]*models.POStaffAssignment, error) {
	return nil, nil
}

type mockBatchRepo struct {
	batches map[string]*models.ProductionBatch
}

func (m *mockBatchRepo) Create(ctx context.Context, b *models.ProductionBatch) error {
	m.batches[b.ID] = b
	return nil
}
func (m *mockBatchRepo) FindByID(ctx context.Context, id string) (*models.ProductionBatch, error) {
	return m.batches[id], nil
}
func (m *mockBatchRepo) FindByNode(ctx context.Context, nodeID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	var res []*models.ProductionBatch
	var keys []string
	for k := range m.batches {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Đảm bảo thứ tự cố định
	for _, k := range keys {
		b := m.batches[k]
		if b.NodeID == nodeID {
			if len(statuses) == 0 {
				res = append(res, b)
				continue
			}
			for _, s := range statuses {
				if b.Status == s {
					res = append(res, b)
					break
				}
			}
		}
	}
	return res, nil
}
func (m *mockBatchRepo) FindByMachine(ctx context.Context, machineID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	var res []*models.ProductionBatch
	for _, b := range m.batches {
		if b.MachineID == machineID {
			for _, s := range statuses {
				if b.Status == s {
					res = append(res, b)
					break
				}
			}
		}
	}
	return res, nil
}
func (m *mockBatchRepo) UpdateStatus(ctx context.Context, id string, s models.BatchStatus) error {
	m.batches[id].Status = s
	fmt.Printf("[DB] %-10s -> %s\n", m.batches[id].SOPStepID, s)
	return nil
}
func (m *mockBatchRepo) Update(ctx context.Context, b *models.ProductionBatch) error {
	m.batches[b.ID] = b
	fmt.Printf("[DB] Allocation: %-10s -> %s\n", b.SOPStepID, b.MachineID)
	return nil
}
func (m *mockBatchRepo) Delete(ctx context.Context, id string) error { return nil }

type mockMachineRepo struct{ machines map[string]*models.Machine }

func (m *mockMachineRepo) FindByNodeID(ctx context.Context, id string) ([]*models.Machine, error) {
	var res []*models.Machine
	var keys []string
	for k := range m.machines {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Đảm bảo thứ tự máy cố định (GRILL, then FRYERS)
	for _, k := range keys {
		if m.machines[k].NodeID == id {
			res = append(res, m.machines[k])
		}
	}
	return res, nil
}
func (m *mockMachineRepo) FindByID(ctx context.Context, id string) (*models.Machine, error) {
	return m.machines[id], nil
}
func (m *mockMachineRepo) FindAll(ctx context.Context) ([]*models.Machine, error) { return nil, nil }
func (m *mockMachineRepo) FindIdleByStationType(ctx context.Context, nid, stid string) ([]*models.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) UpdateStatus(ctx context.Context, id string, s models.MachineStatus, bid *string) error {
	return nil
}
func (m *mockMachineRepo) Update(ctx context.Context, mach *models.Machine) error { return nil }
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error            { return nil }
func (m *mockMachineRepo) Create(ctx context.Context, mach *models.Machine) error { return nil }

type mockSOPRepo struct{ steps map[string][]*models.SOPStep }

func (m *mockSOPRepo) ListSteps(ctx context.Context, sopID string) ([]*models.SOPStep, error) {
	return m.steps[sopID], nil
}
func (m *mockSOPRepo) FindStepByID(ctx context.Context, id string) (*models.SOPStep, error) {
	for _, steps := range m.steps {
		for _, s := range steps {
			if s.ID == id {
				return s, nil
			}
		}
	}
	return nil, nil
}
func (m *mockSOPRepo) Create(ctx context.Context, sop *models.SOP) error            { return nil }
func (m *mockSOPRepo) FindByID(ctx context.Context, id string) (*models.SOP, error) { return nil, nil }
func (m *mockSOPRepo) FindByBOMID(ctx context.Context, bid string) (*models.SOP, error) {
	return nil, nil
}
func (m *mockSOPRepo) Update(ctx context.Context, sop *models.SOP) error          { return nil }
func (m *mockSOPRepo) Delete(ctx context.Context, id string) error                { return nil }
func (m *mockSOPRepo) AddStep(ctx context.Context, step *models.SOPStep) error    { return nil }
func (m *mockSOPRepo) DeleteStep(ctx context.Context, sopID, stepID string) error { return nil }

func main() {
	ctx := context.Background()
	poRepo := &mockPORepo{pos: make(map[string]*models.ProductionOrder)}
	batchRepo := &mockBatchRepo{batches: make(map[string]*models.ProductionBatch)}
	machineRepo := &mockMachineRepo{machines: make(map[string]*models.Machine)}
	sopRepo := &mockSOPRepo{steps: make(map[string][]*models.SOPStep)}
	nodeID := "STORE_001"

	// 1. SETUP DATA
	machineRepo.machines["M1_GRILL"] = &models.Machine{ID: "M1_GRILL", NodeID: nodeID, StationTypeID: "GRILL", AllocationStrategy: models.StrategyAsync, MaxCapacity: 8, OperationalThreshold: 8}
	machineRepo.machines["M2_FRYER_1"] = &models.Machine{ID: "M2_FRYER_1", NodeID: nodeID, StationTypeID: "FRYER", AllocationStrategy: models.StrategySync, MaxCapacity: 4, OperationalThreshold: 4}
	machineRepo.machines["M3_FRYER_2"] = &models.Machine{ID: "M3_FRYER_2", NodeID: nodeID, StationTypeID: "FRYER", AllocationStrategy: models.StrategySync, MaxCapacity: 2, OperationalThreshold: 2}

	sopRepo.steps["SOP_A"] = []*models.SOPStep{{ID: "A_BEEF", StationTypeID: "GRILL"}, {ID: "A_ONION", StationTypeID: "FRYER"}, {ID: "A_FRIES", StationTypeID: "FRYER"}}
	sopRepo.steps["SOP_B"] = []*models.SOPStep{{ID: "B_BUN", StationTypeID: "GRILL"}, {ID: "B_ONION", StationTypeID: "FRYER"}, {ID: "B_CHICKEN", StationTypeID: "FRYER"}}
	sopRepo.steps["SOP_C"] = []*models.SOPStep{{ID: "C_BEEF", StationTypeID: "GRILL"}}

	engine := usecase.NewAllocationUseCase(poRepo, batchRepo, machineRepo, sopRepo)

	fmt.Println("--- PHASE 1: ĐƠN HÀNG A & B ĐẾN (09:00) ---")
	// Tạo batch thủ công với ItemID và Slots chính xác
	batchRepo.batches["B1"] = &models.ProductionBatch{ID: "B1", POID: "PO_A", SOPStepID: "A_BEEF", NodeID: nodeID, Status: models.BatchQueued, ItemID: "BEEF", SlotsUsed: 1}
	batchRepo.batches["B2"] = &models.ProductionBatch{ID: "B2", POID: "PO_A", SOPStepID: "A_ONION", NodeID: nodeID, Status: models.BatchQueued, ItemID: "ONION", SlotsUsed: 1}
	batchRepo.batches["B3"] = &models.ProductionBatch{ID: "B3", POID: "PO_A", SOPStepID: "A_FRIES", NodeID: nodeID, Status: models.BatchQueued, ItemID: "FRIES", SlotsUsed: 2}
	batchRepo.batches["B4"] = &models.ProductionBatch{ID: "B4", POID: "PO_B", SOPStepID: "B_BUN", NodeID: nodeID, Status: models.BatchQueued, ItemID: "BUN", SlotsUsed: 1}
	batchRepo.batches["B5"] = &models.ProductionBatch{ID: "B5", POID: "PO_B", SOPStepID: "B_ONION", NodeID: nodeID, Status: models.BatchQueued, ItemID: "ONION", SlotsUsed: 1}
	batchRepo.batches["B6"] = &models.ProductionBatch{ID: "B6", POID: "PO_B", SOPStepID: "B_CHICKEN", NodeID: nodeID, Status: models.BatchQueued, ItemID: "CHICKEN", SlotsUsed: 2}

	poRepo.pos["PO_A"] = &models.ProductionOrder{ID: "PO_A", NodeID: nodeID, SOPID: "SOP_A"}
	poRepo.pos["PO_B"] = &models.ProductionOrder{ID: "PO_B", NodeID: nodeID, SOPID: "SOP_B"}

	_ = engine.RunAllocation(ctx, nodeID)

	fmt.Println("\n--- PHASE 2: ĐƠN HÀNG C ĐẾN (09:03) ---")
	// Di chuyển các mẻ đang gán sang InProgress
	for _, b := range batchRepo.batches {
		if b.Status == models.BatchAllocated {
			b.Status = models.BatchInProgress
		}
	}

	batchRepo.batches["B7"] = &models.ProductionBatch{ID: "B7", POID: "PO_C", SOPStepID: "C_BEEF", NodeID: nodeID, Status: models.BatchQueued, ItemID: "BEEF", SlotsUsed: 1}
	poRepo.pos["PO_C"] = &models.ProductionOrder{ID: "PO_C", NodeID: nodeID, SOPID: "SOP_C"}
	_ = engine.RunAllocation(ctx, nodeID)

	fmt.Println("\n--- PHASE 3: HOÀN TẤT MẺ HÀNH TÂY ---")
	fmt.Println("Action: Onion batch on M2_FRYER_1 finished. Picking up Chicken...")
	_ = engine.ConfirmCompletion(ctx, "B2") // Hoàn tất Onion A
	_ = engine.ConfirmCompletion(ctx, "B5") // Hoàn tất Onion B (Gom lô)

	fmt.Println("\n--- TRẠNG THÁI CUỐI CÙNG ---")
	// Tạo danh sách để in theo thứ tự
	var ids []string
	for id := range batchRepo.batches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b := batchRepo.batches[id]
		mach := b.MachineID
		if mach == "" {
			mach = "QUEUE"
		}
		fmt.Printf("[%s] Batch %s (%-10s): %s\n", mach, id, b.SOPStepID, b.Status)
	}
}
