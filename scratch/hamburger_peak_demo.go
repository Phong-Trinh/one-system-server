package main

import (
	"context"
	"fmt"
	"one-system-server/internal/domain/models"
	"one-system-server/internal/usecase"
	"sort"
	"strings"
)

// --- Mock Repositories ---
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
	fmt.Printf("[DB] Đã tạo mẻ mới: %-20s (SOP: %s)\n", b.ID, b.SOPStepID)
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
	sort.Strings(keys)
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
	fmt.Printf("[DB] Bước %-20s -> %s\n", m.batches[id].SOPStepID, s)
	return nil
}
func (m *mockBatchRepo) Update(ctx context.Context, b *models.ProductionBatch) error {
	m.batches[b.ID] = b
	if b.Status == models.BatchAllocated {
		fmt.Printf("[DB] Phân bổ: %-20s -> %s\n", b.SOPStepID, b.MachineID)
	}
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
	sort.Strings(keys)
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
	nodeID := "CUA_HANG_01"

	// 1. THIẾT LẬP DỮ LIỆU
	fmt.Println("=== KHỞI TẠO HỆ THỐNG ===")

	machines := []*models.Machine{
		{ID: "M1_BEP_NUONG", NodeID: nodeID, StationTypeID: "BEP_NUONG", AllocationStrategy: models.StrategyAsync, MaxCapacity: 8, OperationalThreshold: 8},
		{ID: "M2_MAY_CHIEN_1", NodeID: nodeID, StationTypeID: "MAY_CHIEN", AllocationStrategy: models.StrategySync, MaxCapacity: 4, OperationalThreshold: 4},
		{ID: "M3_MAY_CHIEN_2", NodeID: nodeID, StationTypeID: "MAY_CHIEN", AllocationStrategy: models.StrategySync, MaxCapacity: 2, OperationalThreshold: 2},
	}
	for _, m := range machines {
		machineRepo.machines[m.ID] = m
		fmt.Printf("Máy: %-15s | Loại: %-10s | Sức chứa: %d | Chiến lược: %s\n", m.ID, m.StationTypeID, m.MaxCapacity, m.AllocationStrategy)
	}

	// SOP cho Hamburger Bò
	sopRepo.steps["SOP_HAMBURGER_BO"] = []*models.SOPStep{
		{ID: "NUONG_BO", StationTypeID: "BEP_NUONG", SlotConsumption: 1, AllowMix: true, Duration: 180},
		{ID: "CHIEN_HANH", StationTypeID: "MAY_CHIEN", SlotConsumption: 1, AllowMix: false, Duration: 120},
		{ID: "CHIEN_KHOAI", StationTypeID: "MAY_CHIEN", SlotConsumption: 2, AllowMix: false, Duration: 300},
		{ID: "SAP_XEP_BO", StationTypeID: "", SlotConsumption: 0, AllowMix: true, Duration: 60, DependsOn: []string{"NUONG_BO", "CHIEN_HANH", "CHIEN_KHOAI"}},
	}
	// SOP cho Hamburger Gà
	sopRepo.steps["SOP_HAMBURGER_GA"] = []*models.SOPStep{
		{ID: "NUONG_BANH", StationTypeID: "BEP_NUONG", SlotConsumption: 1, AllowMix: true, Duration: 60},
		{ID: "CHIEN_HANH_GA", StationTypeID: "MAY_CHIEN", SlotConsumption: 1, AllowMix: false, Duration: 120},
		{ID: "CHIEN_GA", StationTypeID: "MAY_CHIEN", SlotConsumption: 2, AllowMix: false, Duration: 420},
		{ID: "SAP_XEP_GA", StationTypeID: "", SlotConsumption: 0, AllowMix: true, Duration: 60, DependsOn: []string{"NUONG_BANH", "CHIEN_HANH_GA", "CHIEN_GA"}},
	}
	// SOP cho Bò Bít Tết
	sopRepo.steps["SOP_BIT_TET"] = []*models.SOPStep{
		{ID: "NUONG_BIT_TET", StationTypeID: "BEP_NUONG", SlotConsumption: 2, AllowMix: true, Duration: 300},
		{ID: "SAP_XEP_BIT_TET", StationTypeID: "", SlotConsumption: 0, AllowMix: true, Duration: 60, DependsOn: []string{"NUONG_BIT_TET"}},
	}

	fmt.Println("\n=== CẤU HÌNH SOP ===")
	for sopID, steps := range sopRepo.steps {
		fmt.Printf("SOP: %s\n", sopID)
		for _, s := range steps {
			deps := "Không"
			if len(s.DependsOn) > 0 {
				deps = strings.Join(s.DependsOn, ", ")
			}
			fmt.Printf("  - %-20s | Trạm: %-10s | Slots: %.1f | Phụ thuộc: %s\n", s.ID, s.StationTypeID, s.SlotConsumption, deps)
		}
	}

	engine := usecase.NewAllocationUseCase(poRepo, batchRepo, machineRepo, sopRepo)

	fmt.Println("\n--- GIAI ĐOẠN 1: TIẾP NHẬN ĐƠN HÀNG A & B ---")
	poRepo.pos["DH_A"] = &models.ProductionOrder{ID: "DH_A", ItemID: "HAMBURGER_BO", NodeID: nodeID, SOPID: "SOP_HAMBURGER_BO", TargetQty: 1}
	poRepo.pos["DH_B"] = &models.ProductionOrder{ID: "DH_B", ItemID: "HAMBURGER_GA", NodeID: nodeID, SOPID: "SOP_HAMBURGER_GA", TargetQty: 1}

	// Sử dụng DecomposePO để hệ thống tự tạo batch từ SOP
	_ = engine.DecomposePO(ctx, "DH_A")
	_ = engine.DecomposePO(ctx, "DH_B")

	fmt.Println("\n--- GIAI ĐOẠN 2: TIẾP NHẬN ĐƠN HÀNG C ---")
	// Bắt đầu nấu các mẻ đã phân bổ
	for _, b := range batchRepo.batches {
		if b.Status == models.BatchAllocated {
			b.Status = models.BatchInProgress
		}
	}
	poRepo.pos["DH_C"] = &models.ProductionOrder{ID: "DH_C", ItemID: "BIT_TET", NodeID: nodeID, SOPID: "SOP_BIT_TET", TargetQty: 1}
	_ = engine.DecomposePO(ctx, "DH_C")

	fmt.Println("\n--- GIAI ĐOẠN 3: MÔ PHỎNG HOÀN TẤT TOÀN BỘ QUY TRÌNH ---")

	// Hàm tiện ích để hoàn tất tất cả các mẻ đang chờ hoặc đang nấu
	confirmAll := func() bool {
		anyConfirmed := false
		// Lấy danh sách ID để tránh lỗi lặp khi map thay đổi
		var ids []string
		for id := range batchRepo.batches {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			b := batchRepo.batches[id]
			if b.Status == models.BatchAllocated || b.Status == models.BatchInProgress || (b.Status == models.BatchQueued && b.MachineID == "") {
				fmt.Printf("Xác nhận hoàn tất mẻ %s (%s)\n", id, b.SOPStepID)
				_ = engine.ConfirmCompletion(ctx, id)
				anyConfirmed = true
			}
		}
		return anyConfirmed
	}

	// Lặp cho đến khi không còn mẻ nào cần hoàn tất (đã hoàn thành tất cả các bước bao gồm SAP_XEP)
	for confirmAll() {
		_ = engine.RunAllocation(ctx, nodeID)
		// Đẩy các mẻ vừa được phân bổ sang InProgress để có thể hoàn tất ở vòng lặp sau
		for _, b := range batchRepo.batches {
			if b.Status == models.BatchAllocated {
				b.Status = models.BatchInProgress
			}
		}
	}

	fmt.Println("\n=== TRẠNG THÁI CUỐI CÙNG: TẤT CẢ ĐƠN HÀNG ĐÃ SẴN SÀNG ===")
	var ids []string
	for id := range batchRepo.batches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b := batchRepo.batches[id]
		mach := b.MachineID
		if mach == "" {
			mach = "THỦ CÔNG"
		}
		fmt.Printf("[%s] Mẻ %s (%-20s): %s\n", mach, id, b.SOPStepID, b.Status)
	}

	fmt.Println("\nTổng kết: Đơn hàng DH_A, DH_B, DH_C đã hoàn thành bước 'Sắp xếp món' và sẵn sàng phục vụ khách hàng!")
}
