package usecase_test

import (
	"context"
	"time"

	"one-system-server/internal/domain/models"
)

// ─── MOCK REPOSITORIES ───────────────────────────────────────────────────────

type mockProductionOrderRepo struct {
	pos map[string]*models.ProductionOrder
}

func newMockProductionOrderRepo() *mockProductionOrderRepo {
	return &mockProductionOrderRepo{pos: make(map[string]*models.ProductionOrder)}
}
func (m *mockProductionOrderRepo) Create(ctx context.Context, po *models.ProductionOrder) error {
	m.pos[po.ID] = po
	return nil
}
func (m *mockProductionOrderRepo) FindByID(ctx context.Context, id string) (*models.ProductionOrder, error) {
	return m.pos[id], nil
}
func (m *mockProductionOrderRepo) FindByNode(ctx context.Context, nodeID string) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockProductionOrderRepo) FindByStatus(ctx context.Context, status models.POStatus) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockProductionOrderRepo) FindAll(ctx context.Context) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockProductionOrderRepo) UpdateStatus(ctx context.Context, id string, status models.POStatus, actualOutput *float64) error {
	if po, ok := m.pos[id]; ok {
		po.Status = status
	}
	return nil
}
func (m *mockProductionOrderRepo) Update(ctx context.Context, po *models.ProductionOrder) error {
	m.pos[po.ID] = po
	return nil
}
func (m *mockProductionOrderRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockProductionOrderRepo) FindByReferenceOrderIDs(ctx context.Context, orderIDs []string) ([]*models.ProductionOrder, error) {
	return nil, nil
}
func (m *mockProductionOrderRepo) SaveSnapshot(ctx context.Context, snap *models.BOMSnapshot) error {
	return nil
}
func (m *mockProductionOrderRepo) GetSnapshot(ctx context.Context, poID string) (*models.BOMSnapshot, error) {
	return nil, nil
}
func (m *mockProductionOrderRepo) AssignStaff(ctx context.Context, assignment *models.POStaffAssignment) error {
	return nil
}
func (m *mockProductionOrderRepo) ListStaffAssignments(ctx context.Context, poID string) ([]*models.POStaffAssignment, error) {
	return nil, nil
}

type mockSOPRepo struct {
	sops  map[string]*models.SOP
	steps map[string]*models.SOPStep
}

func newMockSOPRepo() *mockSOPRepo {
	return &mockSOPRepo{sops: make(map[string]*models.SOP), steps: make(map[string]*models.SOPStep)}
}
func (m *mockSOPRepo) Create(ctx context.Context, sop *models.SOP) error { return nil }
func (m *mockSOPRepo) FindByID(ctx context.Context, id string) (*models.SOP, error) {
	return m.sops[id], nil
}
func (m *mockSOPRepo) FindByBOMID(ctx context.Context, bomID string) (*models.SOP, error) { return nil, nil }
func (m *mockSOPRepo) Update(ctx context.Context, sop *models.SOP) error                { return nil }
func (m *mockSOPRepo) Delete(ctx context.Context, id string) error                      { return nil }
func (m *mockSOPRepo) AddStep(ctx context.Context, step *models.SOPStep) error {
	m.steps[step.ID] = step
	return nil
}
func (m *mockSOPRepo) FindStepByID(ctx context.Context, sopStepID string) (*models.SOPStep, error) {
	return m.steps[sopStepID], nil
}
func (m *mockSOPRepo) ListSteps(ctx context.Context, sopID string) ([]*models.SOPStep, error) {
	var res []*models.SOPStep
	for _, s := range m.steps {
		if s.SOPID == sopID {
			res = append(res, s)
		}
	}
	return res, nil
}
func (m *mockSOPRepo) DeleteStep(ctx context.Context, sopID string, stepID string) error { return nil }
func (m *mockSOPRepo) DeleteStepsBySOPID(ctx context.Context, sopID string) error        { return nil }

type mockProductionBatchRepo struct {
	batches map[string]*models.ProductionBatch
}

func newMockProductionBatchRepo() *mockProductionBatchRepo {
	return &mockProductionBatchRepo{batches: make(map[string]*models.ProductionBatch)}
}
func (m *mockProductionBatchRepo) Create(ctx context.Context, batch *models.ProductionBatch) error {
	m.batches[batch.ID] = batch
	return nil
}
func (m *mockProductionBatchRepo) FindByID(ctx context.Context, id string) (*models.ProductionBatch, error) {
	return m.batches[id], nil
}
func (m *mockProductionBatchRepo) FindByNode(ctx context.Context, nodeID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	return nil, nil
}
func (m *mockProductionBatchRepo) FindByMachine(ctx context.Context, machineID string, statuses []models.BatchStatus) ([]*models.ProductionBatch, error) {
	var res []*models.ProductionBatch
	for _, b := range m.batches {
		res = append(res, b) // Simplify
	}
	return res, nil
}
func (m *mockProductionBatchRepo) UpdateStatus(ctx context.Context, id string, status models.BatchStatus) error {
	if b, ok := m.batches[id]; ok {
		b.Status = status
	}
	return nil
}
func (m *mockProductionBatchRepo) Update(ctx context.Context, batch *models.ProductionBatch) error {
	m.batches[batch.ID] = batch
	return nil
}
func (m *mockProductionBatchRepo) Delete(ctx context.Context, id string) error { return nil }

type mockStaffShiftRepo struct {
	shifts map[string]*models.StaffShift
}

func newMockStaffShiftRepo() *mockStaffShiftRepo {
	return &mockStaffShiftRepo{shifts: make(map[string]*models.StaffShift)}
}
func (m *mockStaffShiftRepo) Create(ctx context.Context, s *models.StaffShift) error {
	m.shifts[s.ID] = s
	return nil
}
func (m *mockStaffShiftRepo) FindByID(ctx context.Context, id string) (*models.StaffShift, error) {
	return m.shifts[id], nil
}
func (m *mockStaffShiftRepo) FindActiveByNode(ctx context.Context, nodeID string) ([]*models.StaffShift, error) {
	var res []*models.StaffShift
	for _, s := range m.shifts {
		if s.NodeID == nodeID && s.Status == models.ShiftActive {
			res = append(res, s)
		}
	}
	return res, nil
}
func (m *mockStaffShiftRepo) FindByStaff(ctx context.Context, staffID string) ([]*models.StaffShift, error) {
	return nil, nil
}
func (m *mockStaffShiftRepo) UpdateStatus(ctx context.Context, id string, status models.ShiftStatus, actualEnd *time.Time) error {
	return nil
}

type mockStaffTaskRepo struct {
	tasks map[string]*models.StaffTask
}

func newMockStaffTaskRepo() *mockStaffTaskRepo {
	return &mockStaffTaskRepo{tasks: make(map[string]*models.StaffTask)}
}
func (m *mockStaffTaskRepo) Create(ctx context.Context, t *models.StaffTask) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockStaffTaskRepo) FindByID(ctx context.Context, id string) (*models.StaffTask, error) {
	return m.tasks[id], nil
}
func (m *mockStaffTaskRepo) FindByPO(ctx context.Context, poID string) ([]*models.StaffTask, error) {
	var res []*models.StaffTask
	for _, t := range m.tasks {
		if t.POID == poID {
			res = append(res, t)
		}
	}
	return res, nil
}
func (m *mockStaffTaskRepo) FindByStaff(ctx context.Context, staffID string, statuses []models.TaskStatus) ([]*models.StaffTask, error) {
	var res []*models.StaffTask
	for _, t := range m.tasks {
		if t.AssignedTo == staffID {
			if len(statuses) == 0 {
				res = append(res, t)
			} else {
				for _, st := range statuses {
					if t.Status == st {
						res = append(res, t)
						break
					}
				}
			}
		}
	}
	return res, nil
}
func (m *mockStaffTaskRepo) FindByNode(ctx context.Context, nodeID string, statuses []models.TaskStatus) ([]*models.StaffTask, error) {
	var res []*models.StaffTask
	for _, t := range m.tasks {
		if t.NodeID == nodeID {
			if len(statuses) == 0 {
				res = append(res, t)
			} else {
				for _, st := range statuses {
					if t.Status == st {
						res = append(res, t)
						break
					}
				}
			}
		}
	}
	return res, nil
}
func (m *mockStaffTaskRepo) FindActiveByStaff(ctx context.Context, staffID string) (*models.StaffTask, error) {
	return nil, nil
}
func (m *mockStaffTaskRepo) FindWaitingByStaff(ctx context.Context, staffID string) ([]*models.StaffTask, error) {
	return nil, nil
}
func (m *mockStaffTaskRepo) FindQueued(ctx context.Context, nodeID string) ([]*models.StaffTask, error) {
	var res []*models.StaffTask
	for _, t := range m.tasks {
		if t.NodeID == nodeID && t.Status == models.TaskQueued {
			res = append(res, t)
		}
	}
	// Sort by Priority (SeqNo) then CreatedAt to simulate FIFO and topological ordering
	for i := 0; i < len(res)-1; i++ {
		for j := i + 1; j < len(res); j++ {
			swap := false
			if res[i].Priority > res[j].Priority {
				swap = true
			} else if res[i].Priority == res[j].Priority {
				if res[i].CreatedAt.After(res[j].CreatedAt) {
					swap = true
				} else if res[i].CreatedAt.Equal(res[j].CreatedAt) {
					if res[i].TaskKind == models.TaskKindRetrieve && res[j].TaskKind == models.TaskKindSetup {
						swap = true
					}
				}
			}
			if swap {
				res[i], res[j] = res[j], res[i]
			}
		}
	}
	return res, nil
}
func (m *mockStaffTaskRepo) Update(ctx context.Context, t *models.StaffTask) error {
	m.tasks[t.ID] = t
	return nil
}

type mockMachineRepo struct {
	machines map[string]*models.Machine
}

func newMockMachineRepo() *mockMachineRepo {
	return &mockMachineRepo{machines: make(map[string]*models.Machine)}
}
func (m *mockMachineRepo) Create(ctx context.Context, req *models.Machine) error { return nil }
func (m *mockMachineRepo) FindByID(ctx context.Context, id string) (*models.Machine, error) {
	return m.machines[id], nil
}
func (m *mockMachineRepo) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mac := range m.machines {
		if mac.NodeID == nodeID {
			res = append(res, mac)
		}
	}
	return res, nil
}
func (m *mockMachineRepo) FindByNodeAndStation(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) FindIdleByStationType(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) FindAll(ctx context.Context) ([]*models.Machine, error) {
	return nil, nil
}
func (m *mockMachineRepo) Update(ctx context.Context, req *models.Machine) error { return nil }
func (m *mockMachineRepo) UpdateStatus(ctx context.Context, id string, status models.MachineStatus, batchID *string) error {
	return nil
}
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error { return nil }
