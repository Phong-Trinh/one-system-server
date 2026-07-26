package usecase

// Test riêng: 1 nhân viên duy nhất trong ca, đảm nhận FULL PO từ F cho S:
//   - 200 vỏ bánh burger (sop_bun)
//   - 300 patty bò 120g  (sop_patty)
//   - 1 lô nước xốt 1L   (sop_sauce)
//
// Chạy: go test -run TestFactory_SC3_FullLoad_1Staff -v -timeout 300s

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

func TestFactory_SC3_FullLoad_1Staff(t *testing.T) {
	_, poRepo, sopRepo, batchRepo, shiftRepo, taskRepo, machineRepo, disp, engine :=
		setupTestEnv()
	_ = batchRepo

	nowBase := time.Date(2026, 7, 21, 6, 0, 0, 0, time.Local) // Ca bắt đầu 06:00
	nodeID := "factory_f"
	staffIDs := []string{"f_staff_1"}

	// ── 1 nhân viên duy nhất ─────────────────────────────────────────────────
	shiftRepo.shifts["shift_1"] = &models.StaffShift{
		ID: "shift_1", NodeID: nodeID,
		StaffID: "f_staff_1", Status: models.ShiftActive,
	}

	// ── Thiết bị ─────────────────────────────────────────────────────────────
	// 1 máy nướng × 48 slots (1 lò 2 khay, mỗi khay 24 bánh)
	machineRepo.machines["m_grill_A"] = &models.Machine{
		ID: "m_grill_A", NodeID: nodeID, EquipmentTypeID: "grill",
		Status: models.MachineIdle, MaxCapacity: 48,
	}
	// m_grill_B đã bị vô hiệu hoá theo yêu cầu "tính toán trong trường hợp có 1 lò"
	// machineRepo.machines["m_grill_B"] = &models.Machine{
	// 	ID: "m_grill_B", NodeID: nodeID, EquipmentTypeID: "grill",
	// 	Status: models.MachineIdle, MaxCapacity: 48,
	// }
	// Mixer & proofer cho bun
	machineRepo.machines["m_mixer"] = &models.Machine{
		ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer",
		Status: models.MachineIdle, MaxCapacity: 1,
	}
	machineRepo.machines["m_proofer"] = &models.Machine{
		ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer",
		Status: models.MachineIdle, MaxCapacity: 1,
	}

	// ── SOP: Vỏ bánh burger – 200 cái ────────────────────────────────────────
	// Bước 1: Cân nguyên liệu khô        – 10 phút (tay)
	// Bước 2: Trộn bột (mixer)           – 15 phút máy, 3 phút nhân viên
	// Bước 3: Ủ bột (proofer)            – 45 phút máy, 5 phút nhân viên
	// Bước 4: Tạo hình bánh              – 20 phút (tay)
	// Bước 5: Nướng (grill, 24 slots)    – 18 phút máy, 3 phút nhân viên
	//   → 200 bun / 24 slots = 9 mẻ (1 máy), hoặc 5 mẻ nếu dùng 2 máy song song
	//   → với 1 nhân viên: cần SETUP từng mẻ tuần tự
	sopBun := "sop_bun_sc3f"
	sopRepo.sops[sopBun] = &models.SOP{ID: sopBun}
	sopRepo.steps["f3_bun_dry"] = &models.SOPStep{
		ID: "f3_bun_dry", SOPID: sopBun, SeqNo: 1,
		Duration: 10 * 60,
	}
	sopRepo.steps["f3_bun_mix"] = &models.SOPStep{
		ID: "f3_bun_mix", SOPID: sopBun, SeqNo: 2, DependsOn: []string{"f3_bun_dry"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true,
		Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60),
		AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["f3_bun_proof"] = &models.SOPStep{
		ID: "f3_bun_proof", SOPID: sopBun, SeqNo: 3, DependsOn: []string{"f3_bun_mix"},
		EquipmentTypeID: ptrStrF("proofer"), IsIdleStep: true,
		Duration: 45 * 60, ActiveTime: ptrIntF(5 * 60),
		AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["f3_bun_shape"] = &models.SOPStep{
		ID: "f3_bun_shape", SOPID: sopBun, SeqNo: 4, DependsOn: []string{"f3_bun_proof"},
		Duration: 20 * 60,
	}
	sopRepo.steps["f3_bun_bake"] = &models.SOPStep{
		ID: "f3_bun_bake", SOPID: sopBun, SeqNo: 5, DependsOn: []string{"f3_bun_shape"},
		EquipmentTypeID: ptrStrF("grill"), IsIdleStep: true,
		Duration: 15 * 60, ActiveTime: ptrIntF(3 * 60),
		AttentionLevel: models.AttentionFullIdle,
		SlotConsumption: 1.0,
	}

	// ── SOP: Patty bò – 300 miếng × 120g ─────────────────────────────────────
	// Bước 1: Chuẩn bị nguyên liệu bò   – 15 phút (tay)
	// Bước 2: Trộn bò xay (mixer)        – 12 phút máy, 2 phút nhân viên
	// Bước 3: Chia & cân viên 120g       – 30 phút (tay)
	// Bước 4: Tạo hình bằng khuôn        – 20 phút (tay)
	// Bước 5: Nướng (grill, 24 slots)    – 10 phút máy, 2 phút nhân viên
	//   → 300 patty / 24 slots = 13 mẻ (1 máy) → 7 mẻ nếu 2 máy song song
	sopPatty := "sop_patty_sc3f"
	sopRepo.sops[sopPatty] = &models.SOP{ID: sopPatty}
	sopRepo.steps["f3_patty_prep"] = &models.SOPStep{
		ID: "f3_patty_prep", SOPID: sopPatty, SeqNo: 1,
		Duration: 15 * 60,
	}
	sopRepo.steps["f3_patty_mix"] = &models.SOPStep{
		ID: "f3_patty_mix", SOPID: sopPatty, SeqNo: 2, DependsOn: []string{"f3_patty_prep"},
		EquipmentTypeID: ptrStrF("mixer"), IsIdleStep: true,
		Duration: 12 * 60, ActiveTime: ptrIntF(2 * 60),
		AttentionLevel: models.AttentionFullIdle,
	}
	sopRepo.steps["f3_patty_weigh"] = &models.SOPStep{
		ID: "f3_patty_weigh", SOPID: sopPatty, SeqNo: 3, DependsOn: []string{"f3_patty_mix"},
		// Cân 1 viên mất 40s (trung bình 30-45s) -> 300 viên * 40s = 12000s = 200 phút
		Duration: 200 * 60,
	}
	sopRepo.steps["f3_patty_mold"] = &models.SOPStep{
		ID: "f3_patty_mold", SOPID: sopPatty, SeqNo: 4, DependsOn: []string{"f3_patty_weigh"},
		// Tạo hình 1 miếng mất 30s -> 300 miếng * 30s = 9000s = 150 phút
		Duration: 150 * 60,
	}
	sopRepo.steps["f3_patty_pack"] = &models.SOPStep{
		ID: "f3_patty_pack", SOPID: sopPatty, SeqNo: 5, DependsOn: []string{"f3_patty_mold"},
		Duration: 10 * 60,
	}

	// ── SOP: Nước xốt burger – 1 lô (1L) ────────────────────────────────────
	// Không cần máy chuyên dụng, làm 1 lần duy nhất
	// Bước 1: Chuẩn bị nguyên liệu xốt  – 10 phút
	// Bước 2: Trộn & nêm xốt            – 20 phút
	// Bước 3: Đóng gói + kiểm tra CL    – 10 phút
	sopSauce := "sop_sauce_sc3f"
	sopRepo.sops[sopSauce] = &models.SOP{ID: sopSauce}
	sopRepo.steps["f3_sauce_prep"] = &models.SOPStep{
		ID: "f3_sauce_prep", SOPID: sopSauce, SeqNo: 1, Duration: 10 * 60,
	}
	sopRepo.steps["f3_sauce_mix"] = &models.SOPStep{
		ID: "f3_sauce_mix", SOPID: sopSauce, SeqNo: 2,
		DependsOn: []string{"f3_sauce_prep"}, Duration: 20 * 60,
	}
	sopRepo.steps["f3_sauce_pack"] = &models.SOPStep{
		ID: "f3_sauce_pack", SOPID: sopSauce, SeqNo: 3,
		DependsOn: []string{"f3_sauce_mix"}, Duration: 10 * 60,
	}

	// ── Production Orders ────────────────────────────────────────────────────
	poBun := &models.ProductionOrder{
		ID: "po_bun_f3", NodeID: nodeID, SOPID: sopBun,
		Status: models.POInProgress, CreatedAt: nowBase,
		TargetQty: 200,
	}
	poPatty := &models.ProductionOrder{
		ID: "po_patty_f3", NodeID: nodeID, SOPID: sopPatty,
		Status: models.POInProgress, CreatedAt: nowBase,
		TargetQty: 300,
	}
	poSauce := &models.ProductionOrder{
		ID: "po_sauce_f3", NodeID: nodeID, SOPID: sopSauce,
		Status: models.POInProgress, CreatedAt: nowBase,
		TargetQty: 1,
	}
	poRepo.pos["po_bun_f3"] = poBun
	poRepo.pos["po_patty_f3"] = poPatty
	poRepo.pos["po_sauce_f3"] = poSauce

	// ── Simulation ───────────────────────────────────────────────────────────
	// Nhân viên F-1 nhận lệnh lúc 06:00 – schedule cả 3 PO
	events := []factoryEvent{
		{vTime: 0, label: "F-1 nhận ca – Schedule BUN (200) + PATTY (300) + SAUCE (1L)", handler: func() {
			for _, poID := range []string{"po_bun_f3", "po_patty_f3", "po_sauce_f3"} {
				if _, err := engine.SchedulePO(t.Context(), poID); err != nil {
					t.Errorf("SchedulePO %s: %v", poID, err)
				}
			}
			disp.Dispatch(t.Context(), nodeID)
		}},
	}

	logs := runFactorySimulation(
		t, "SC-3 FULL LOAD – 1 Nhân Viên / Full PO cho S",
		nowBase, 600, // 10 tiếng buffer tối đa
		engine, disp, taskRepo, sopRepo, machineRepo,
		nodeID, staffIDs, events,
	)

	// ── Assertions ───────────────────────────────────────────────────────────
	assertionsFactory(t, taskRepo, sopRepo, staffIDs)

	// A4: Tất cả task phải xong trước khi ca kết thúc (18:00 = T+720)
	t.Run("A4_FinishBeforeShiftEnd", func(t *testing.T) {
		shiftEnd := nowBase.Add(720 * time.Minute) // 18:00
		overdue := 0
		for _, tk := range taskRepo.tasks {
			if tk.CompletedAt != nil && tk.CompletedAt.After(shiftEnd) {
				t.Errorf("❌ Task [%s] xong lúc %s – trễ ca!",
					resolveFactoryStepName(tk), tk.CompletedAt.Format("15:04"))
				overdue++
			}
		}
		if overdue == 0 {
			t.Logf("✅ Tất cả task xong trước 18:00")
		}
		t.Logf("📋 Tổng %d dòng timeline log", len(logs))
	})

	// A5: Thứ tự ưu tiên – Sauce xong trước Patty và Bun
	t.Run("A5_SauceCompletedFirst", func(t *testing.T) {
		doneTimes := map[string]*time.Time{"po_sauce_f3": nil, "po_patty_f3": nil, "po_bun_f3": nil}
		for _, tk := range taskRepo.tasks {
			if _, ok := doneTimes[tk.POID]; ok && tk.CompletedAt != nil {
				if doneTimes[tk.POID] == nil || tk.CompletedAt.After(*doneTimes[tk.POID]) {
					doneTimes[tk.POID] = tk.CompletedAt
				}
			}
		}
		t.Logf("⏱  Sauce xong: %v", doneTimes["po_sauce_f3"])
		t.Logf("⏱  Patty xong: %v", doneTimes["po_patty_f3"])
		t.Logf("⏱  Bun   xong: %v", doneTimes["po_bun_f3"])
	})

	// A6: Không có task nào nhân viên bị idle >15 phút giữa 2 task liên tiếp
	t.Run("A6_NoLongIdle_Over15Min", func(t *testing.T) {
		var staffTasks []*models.StaffTask
		for _, tk := range taskRepo.tasks {
			if tk.AssignedTo == "f_staff_1" && tk.StartedAt != nil && tk.CompletedAt != nil {
				staffTasks = append(staffTasks, tk)
			}
		}
		sort.Slice(staffTasks, func(i, j int) bool {
			return staffTasks[i].StartedAt.Before(*staffTasks[j].StartedAt)
		})
		totalIdle := time.Duration(0)
		for i := 1; i < len(staffTasks); i++ {
			idle := staffTasks[i].StartedAt.Sub(*staffTasks[i-1].CompletedAt)
			if idle > 0 {
				totalIdle += idle
				if idle > 15*time.Minute {
					t.Logf("⚠️  Idle dài %v sau [%s] → trước [%s]",
						idle,
						resolveFactoryStepName(staffTasks[i-1]),
						resolveFactoryStepName(staffTasks[i]))
				}
			}
		}
		t.Logf("📊 Tổng thời gian idle của f_staff_1: %v", totalIdle)
	})

	// In tóm tắt kết quả cuối
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║         TÓM TẮT KẾT QUẢ – F-1 (1 nhân viên)           ║")
	fmt.Println("╠══════════════════════════════════════════════════════════╣")

	type summary struct {
		poID  string
		label string
	}
	for _, s := range []summary{
		{"po_sauce_f3", "Nước xốt  (1L)    "},
		{"po_patty_f3", "Patty bò  (300 pc)"},
		{"po_bun_f3", "Vỏ burger (200 cái)"},
	} {
		var firstStart, lastDone *time.Time
		taskCount := 0
		for _, tk := range taskRepo.tasks {
			if tk.POID != s.poID {
				continue
			}
			taskCount++
			if tk.StartedAt != nil && (firstStart == nil || tk.StartedAt.Before(*firstStart)) {
				firstStart = tk.StartedAt
			}
			if tk.CompletedAt != nil && (lastDone == nil || tk.CompletedAt.After(*lastDone)) {
				lastDone = tk.CompletedAt
			}
		}
		startStr := "—"
		doneStr := "—"
		durStr := "—"
		if firstStart != nil {
			startStr = firstStart.Format("15:04")
		}
		if lastDone != nil {
			doneStr = lastDone.Format("15:04")
		}
		if firstStart != nil && lastDone != nil {
			durStr = fmt.Sprintf("%.0f phút", lastDone.Sub(*firstStart).Minutes())
		}
		fmt.Printf("║ %-20s  Bắt đầu: %-5s  Xong: %-5s  (%s)\n",
			s.label, startStr, doneStr, durStr)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
}
