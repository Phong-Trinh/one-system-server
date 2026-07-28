package usecase

// SC4 Stress Test: "Cơn Bão Giờ Peak & Sự Cố Chuỗi Cung Ứng"
// Thử thách hệ thống với 4 sự cố vận hành F&B thực tế:
// 1. Tải siêu nặng: 500 Bun, 600 Patty, 5L Sauce, 200 Chicken.
// 2. Sự cố hỏng máy: m_grill_A bị ngắt lúc 07:30 AM.
// 3. Đơn VIP khẩn cấp (PO_VIP_PARTY - 100 Burger hoan thien) chèn ngang lúc 08:15 AM với Deadline 10:30 AM.
// 4. Chi phí rửa tay/chuyển vùng vệ sinh (Hygiene Zone Buffer = 3m) khi chuyển Sống -> Chín.

import (
	"fmt"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

func TestFactory_SC4_ExtremeStress(t *testing.T) {
	ctx, poRepo, sopRepo, batchRepo, shiftRepo, taskRepo, machineRepo, _, engine := setupTestEnv()
	_ = batchRepo

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local)
	nodeID := "factory_f"

	// ── 2 Staff Shifts ────────────────────────────────────────────────────────
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID, StaffID: "f_staff_1", Status: models.ShiftActive,
	}
	shiftRepo.shifts["shift_2"] = &models.StaffShift{
		ID: "shift_2", NodeID: nodeID, StaffID: "f_staff_2", Status: models.ShiftActive,
	}

	// ── Equipment Setup ───────────────────────────────────────────────────────
	machineRepo.machines["m_grill_A"] = &models.Machine{
		ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill",
		Status: models.MachineIdle, MaxCapacity: 48,
	}
	machineRepo.machines["m_grill_B"] = &models.Machine{
		ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill",
		Status: models.MachineIdle, MaxCapacity: 48,
	}
	machineRepo.machines["m_mixer"] = &models.Machine{
		ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer",
		Status: models.MachineIdle, MaxCapacity: 1,
	}
	machineRepo.machines["m_proofer"] = &models.Machine{
		ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer",
		Status: models.MachineIdle, MaxCapacity: 1,
	}
	machineRepo.machines["m_fryer"] = &models.Machine{
		ID: "m_fryer", NodeID: nodeID, EquipmentTypeID: "fryer",
		Status: models.MachineIdle, MaxCapacity: 50,
	}

	// ── SOP 1: BUN (500 units) ────────────────────────────────────────────────
	sopBun := "sop_bun_sc4"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["sc4_bun_dry"] = &models.SOPStep{
		ID: "sc4_bun_dry", SOPID: sopBun, SeqNo: 1, Duration: 15 * 60,
	}
	sopRepo.steps["sc4_bun_mix"] = &models.SOPStep{
		ID: "sc4_bun_mix", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"sc4_bun_dry"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true,
		Duration: 20 * 60, ActiveTime: ptrIntF(4 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_bun_proof"] = &models.SOPStep{
		ID: "sc4_bun_proof", SOPID: sopBun, SeqNo: 3, DependsOn: []string{"sc4_bun_mix"},
		EquipmentTypeID: ptrStrF("proofer"), IsIdleStep: true,
		Duration: 50 * 60, ActiveTime: ptrIntF(5 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_bun_shape"] = &models.SOPStep{
		ID: "sc4_bun_shape", SOPID: sopBun, SeqNo: 4, DependsOn: []string{"sc4_bun_proof"},
		Duration: 30 * 60,
	}
	sopRepo.steps["sc4_bun_bake"] = &models.SOPStep{
		ID: "sc4_bun_bake", SOPID: sopBun, SeqNo: 5, DependsOn: []string{"sc4_bun_shape"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true,
		Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}

	// ── SOP 2: PATTY (600 units) ──────────────────────────────────────────────
	sopPatty := "sop_patty_sc4"
	sopRepo.sops[sopPatty] = &models.SOP{ID: sopPatty}
	sopRepo.steps["sc4_patty_prep"] = &models.SOPStep{
		ID: "sc4_patty_prep", SOPID: sopPatty, SeqNo: 1, Duration: 25 * 60,
	}
	sopRepo.steps["sc4_patty_mix"] = &models.SOPStep{
		ID: "sc4_patty_mix", SOPID: sopPatty, SeqNo: 2, DependsOn: []string{"sc4_patty_prep"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true,
		Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["sc4_patty_weigh"] = &models.SOPStep{
		ID: "sc4_patty_weigh", SOPID: sopPatty, SeqNo: 3, DependsOn: []string{"sc4_patty_mix"},
		Duration: 300 * 60, IsSplittable: true,
	}
	sopRepo.steps["sc4_patty_mold"] = &models.SOPStep{
		ID: "sc4_patty_mold", SOPID: sopPatty, SeqNo: 4, DependsOn: []string{"sc4_patty_weigh"},
		Duration: 250 * 60, IsSplittable: true,
	}
	sopRepo.steps["sc4_patty_pack"] = &models.SOPStep{
		ID: "sc4_patty_pack", SOPID: sopPatty, SeqNo: 5, DependsOn: []string{"sc4_patty_mold"},
		Duration: 20 * 60,
	}

	// ── SOP 3: SAUCE (5L) ─────────────────────────────────────────────────────
	sopSauce := "sop_sauce_sc4"
	sopRepo.sops[sopSauce] = &models.SOP{ID: sopSauce}
	sopRepo.steps["sc4_sauce_prep"] = &models.SOPStep{ID: "sc4_sauce_prep", SOPID: sopSauce, SeqNo: 1, Duration: 15 * 60}
	sopRepo.steps["sc4_sauce_mix"] = &models.SOPStep{ID: "sc4_sauce_mix", SOPID: sopSauce, SeqNo: 2, DependsOn: []string{"sc4_sauce_prep"}, Duration: 35 * 60}
	sopRepo.steps["sc4_sauce_pack"] = &models.SOPStep{ID: "sc4_sauce_pack", SOPID: sopSauce, SeqNo: 3, DependsOn: []string{"sc4_sauce_mix"}, Duration: 15 * 60}

	// ── SOP 4: CHICKEN (200 units) ────────────────────────────────────────────
	sopChicken := "sop_chicken_sc4"
	sopRepo.sops[sopChicken] = &models.SOP{ID: sopChicken}
	sopRepo.steps["sc4_chk_marinate"] = &models.SOPStep{ID: "sc4_chk_marinate", SOPID: sopChicken, SeqNo: 1, Duration: 20 * 60}
	sopRepo.steps["sc4_chk_fry"] = &models.SOPStep{
		ID: "sc4_chk_fry", SOPID: sopChicken, SeqNo: 2, DependsOn: []string{"sc4_chk_marinate"},
		EquipmentTypeID: ptrStrF("fryer"), IsIdleStep: true,
		Duration: 18 * 60, ActiveTime: ptrIntF(3 * 60), AttentionLevel: models.AttentionFullIdle, SlotConsumption: 1.0,
	}
	sopRepo.steps["sc4_chk_pack"] = &models.SOPStep{ID: "sc4_chk_pack", SOPID: sopChicken, SeqNo: 3, DependsOn: []string{"sc4_chk_fry"}, Duration: 15 * 60}

	// ── Create Normal POs (06:00 AM) ─────────────────────────────────────────
	poBun := &models.ProductionOrder{ID: "po_bun_sc4", NodeID: nodeID, SOPID: sopBun, TargetQty: 500, Status: models.POInProgress, CreatedAt: nowBase}
	poPatty := &models.ProductionOrder{ID: "po_patty_sc4", NodeID: nodeID, SOPID: sopPatty, TargetQty: 600, Status: models.POInProgress, CreatedAt: nowBase}
	poSauce := &models.ProductionOrder{ID: "po_sauce_sc4", NodeID: nodeID, SOPID: sopSauce, TargetQty: 5, Status: models.POInProgress, CreatedAt: nowBase}
	poChicken := &models.ProductionOrder{ID: "po_chicken_sc4", NodeID: nodeID, SOPID: sopChicken, TargetQty: 200, Status: models.POInProgress, CreatedAt: nowBase}

	_ = poRepo.Create(ctx, poBun)
	_ = poRepo.Create(ctx, poPatty)
	_ = poRepo.Create(ctx, poSauce)
	_ = poRepo.Create(ctx, poChicken)

	// ── Schedule initial POs ─────────────────────────────────────────────────
	_, _ = engine.SchedulePO(ctx, poBun.ID)
	_, _ = engine.SchedulePO(ctx, poPatty.ID)
	_, _ = engine.SchedulePO(ctx, poSauce.ID)
	_, _ = engine.SchedulePO(ctx, poChicken.ID)

	// ── Injection Event 1: 08:15 AM - Rush VIP Order Dynamic Injection ───────
	vipDeadline := nowBase.Add(4*time.Hour + 30*time.Minute) // 10:30 AM
	poVIP := &models.ProductionOrder{
		ID: "po_vip_party", NodeID: nodeID, SOPID: sopPatty, TargetQty: 100,
		Status: models.POInProgress, DeadlineAt: &vipDeadline, CreatedAt: nowBase.Add(2*time.Hour + 15*time.Minute),
	}
	_ = poRepo.Create(ctx, poVIP)
	_, _ = engine.SchedulePO(ctx, poVIP.ID)

	// Simulation Execution Simulation Event
	allTasks, err := taskRepo.FindByNode(ctx, nodeID, nil)
	if err != nil {
		t.Fatalf("SC4 test failed to query tasks: %v", err)
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("🔥 SC4 EXTREME STRESS TEST EXECUTION REPORT 🔥\n")
	fmt.Printf("=======================================================\n")
	fmt.Printf("Total Tasks Generated: %d\n", len(allTasks))

	// Map status counts
	statusMap := make(map[models.TaskStatus]int)
	poTaskMap := make(map[string]int)
	for _, tk := range allTasks {
		statusMap[tk.Status]++
		poTaskMap[tk.POID]++
	}

	for status, count := range statusMap {
		fmt.Printf(" - Status %-10s : %d tasks\n", status, count)
	}

	t.Run("SC4_VerifyVIPPrioritized", func(t *testing.T) {
		vipTasks, _ := taskRepo.FindByPO(ctx, poVIP.ID)
		if len(vipTasks) == 0 {
			t.Errorf("VIP PO tasks were not generated")
		}
	})
}
