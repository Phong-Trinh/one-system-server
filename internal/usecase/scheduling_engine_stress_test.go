package usecase

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

var stepNames = map[string]string{
	"s1_dry":     "Chuẩn bị nguyên liệu khô",
	"s2_mix":     "Trộn bột Burger",
	"s3_proof":   "Ủ bột Burger",
	"s4_bake":    "Nướng Burger",
	"s1_prep_sw": "Cân đường, bột bánh ngọt",
	"s2_mix_sw":  "Trộn bột Bánh Ngọt",
	"s3_bake_sw": "Nướng Bánh Ngọt",
	"s1_prep_cr": "Cắt vỏ Croissant",
	"s2_bake_cr": "Nướng Croissant",
}

// resolveStepNameStress resolves the step name based on SOPStepID
func resolveStepNameStress(tk *models.StaffTask) string {
	name := stepNames[tk.SOPStepID]
	if tk.TaskKind == models.TaskKindFillIn && tk.ParentTaskID != nil {
		return fmt.Sprintf("[FILL-IN] %s", name)
	}
	if tk.TaskKind == models.TaskKindSetup {
		return "[SETUP] " + name
	}
	if tk.TaskKind == models.TaskKindRetrieve {
		return "[RETRIEVE] " + name
	}
	return name
}

func ptrStrStress(s string) *string { return &s }
func ptrIntStress(i int) *int { return &i }

func TestSchedulingEngine_T9_StressTest(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, disp, engine := setupTestEnv()

	nowBase := time.Date(2026, 7, 15, 7, 0, 0, 0, time.Local) // 07:00 AM

	// --- SETUP VIRTUAL ENVIRONMENT ---

	// Node
	nodeID := "node_1"

	// Staff: John and Alice
	shiftRepo.shifts = map[string]*models.StaffShift{
		"shift_john": {
			ID: "shift_john", StaffID: "baker_john", NodeID: nodeID,
			Status: models.ShiftActive,
		},
		"shift_alice": {
			ID: "shift_alice", StaffID: "baker_alice", NodeID: nodeID,
			Status: models.ShiftActive,
		},
	}

	// Machines: 1 Mixer, 1 Oven, 1 Proofer
	machineRepo.machines = map[string]*models.Machine{
		"m_mixer":   {ID: "m_mixer", NodeID: nodeID, EquipmentTypeID: "mixer", CurrentBatchID: nil, Status: "ONLINE"},
		"m_oven":    {ID: "m_oven", NodeID: nodeID, EquipmentTypeID: "oven", CurrentBatchID: nil, Status: "ONLINE"},
		"m_proofer": {ID: "m_proofer", NodeID: nodeID, EquipmentTypeID: "proofer", CurrentBatchID: nil, Status: "ONLINE"},
	}

	// SOPs for POs
	// PO_1: Burger Bun (Mixer -> Proofer -> Oven)
	sopBurger := "sop_burger"
	sopRepo.steps["s1_dry"] = &models.SOPStep{ID: "s1_dry", SOPID: sopBurger, SeqNo: 1, Duration: 2 * 60}
	sopRepo.steps["s2_mix"] = &models.SOPStep{
		ID: "s2_mix", SOPID: sopBurger, SeqNo: 2, DependsOn: []string{"s1_dry"},
		EquipmentTypeID: ptrStrStress("mixer"), IsIdleStep: true, Duration: 10 * 60, ActiveTime: ptrIntStress(2 * 60),
	}
	sopRepo.steps["s3_proof"] = &models.SOPStep{
		ID: "s3_proof", SOPID: sopBurger, SeqNo: 3, DependsOn: []string{"s2_mix"},
		EquipmentTypeID: ptrStrStress("proofer"), IsIdleStep: true, Duration: 30 * 60, ActiveTime: ptrIntStress(3 * 60),
	}
	sopRepo.steps["s4_bake"] = &models.SOPStep{
		ID: "s4_bake", SOPID: sopBurger, SeqNo: 4, DependsOn: []string{"s3_proof"},
		EquipmentTypeID: ptrStrStress("oven"), IsIdleStep: true, Duration: 15 * 60, ActiveTime: ptrIntStress(2 * 60),
	}

	// PO_2: Bánh mì ngọt (Mixer -> Oven)
	sopSweet := "sop_sweet"
	sopRepo.steps["s1_prep_sw"] = &models.SOPStep{ID: "s1_prep_sw", SOPID: sopSweet, SeqNo: 1, Duration: 3 * 60}
	sopRepo.steps["s2_mix_sw"] = &models.SOPStep{
		ID: "s2_mix_sw", SOPID: sopSweet, SeqNo: 2, DependsOn: []string{"s1_prep_sw"},
		EquipmentTypeID: ptrStrStress("mixer"), IsIdleStep: true, Duration: 8 * 60, ActiveTime: ptrIntStress(2 * 60),
	}
	sopRepo.steps["s3_bake_sw"] = &models.SOPStep{
		ID: "s3_bake_sw", SOPID: sopSweet, SeqNo: 3, DependsOn: []string{"s2_mix_sw"},
		EquipmentTypeID: ptrStrStress("oven"), IsIdleStep: true, Duration: 12 * 60, ActiveTime: ptrIntStress(2 * 60),
	}

	// PO_3: Croissant (Oven only)
	sopCroissant := "sop_croissant"
	sopRepo.steps["s1_prep_cr"] = &models.SOPStep{ID: "s1_prep_cr", SOPID: sopCroissant, SeqNo: 1, Duration: 5 * 60}
	sopRepo.steps["s2_bake_cr"] = &models.SOPStep{
		ID: "s2_bake_cr", SOPID: sopCroissant, SeqNo: 2, DependsOn: []string{"s1_prep_cr"},
		EquipmentTypeID: ptrStrStress("oven"), IsIdleStep: true, Duration: 20 * 60, ActiveTime: ptrIntStress(3 * 60),
	}

	po1 := &models.ProductionOrder{ID: "po_1", NodeID: nodeID, SOPID: sopBurger, Status: models.POInProgress, CreatedAt: nowBase}
	po2 := &models.ProductionOrder{ID: "po_2", NodeID: nodeID, SOPID: sopSweet, Status: models.POInProgress, CreatedAt: nowBase.Add(5 * time.Minute)}
	po3 := &models.ProductionOrder{ID: "po_3", NodeID: nodeID, SOPID: sopCroissant, Status: models.POInProgress, CreatedAt: nowBase.Add(10 * time.Minute)}
	poRepo.pos = make(map[string]*models.ProductionOrder)
	poRepo.pos["po_1"] = po1
	poRepo.pos["po_2"] = po2
	poRepo.pos["po_3"] = po3

	// SIMULATION LOOP
	maxVirtualTime := 180 // 180 minutes
	simulatedLogs := make([]string, 0)

	logSim := func(vTime int, msg string) {
		tStr := fmt.Sprintf("T+%03d [%s]", vTime, nowBase.Add(time.Duration(vTime)*time.Minute).Format("15:04"))
		simulatedLogs = append(simulatedLogs, fmt.Sprintf("%s %s", tStr, msg))
	}

	for vTime := 0; vTime <= maxVirtualTime; vTime++ {
		currentTime := nowBase.Add(time.Duration(vTime) * time.Minute)
		engine.(*schedulingEngine).now = func() time.Time { return currentTime }
		disp.(*dispatcher).now = func() time.Time { return currentTime }

		// 1. Triggers Events
		if vTime == 0 {
			logSim(vTime, "Khách order PO_1 (Burger Bun)")
			_, err := engine.SchedulePO(ctx, po1.ID)
			if err != nil {
				t.Fatalf("SchedulePO 1 failed: %v", err)
			}
		}
		if vTime == 5 {
			logSim(vTime, "Khách order PO_2 (Bánh Mì Ngọt)")
			_, err := engine.SchedulePO(ctx, po2.ID)
			if err != nil {
				t.Fatalf("SchedulePO 2 failed: %v", err)
			}
		}
		if vTime == 10 {
			logSim(vTime, "Khách order PO_3 (Croissant)")
			_, err := engine.SchedulePO(ctx, po3.ID)
			if err != nil {
				t.Fatalf("SchedulePO 3 failed: %v", err)
			}
		}

		// Update all statuses sequentially to simulate staff behaviors
		tasks := taskRepo.tasks

		// 2. Machine finish WAITING -> RETRIEVE task becomes action-ready
		for _, tk := range tasks {
			if tk.Status == models.TaskWaiting {
				step := sopRepo.steps[tk.SOPStepID]
				reqAtt := 0
				if step.RequiresAttentionAt != nil {
					reqAtt = *step.RequiresAttentionAt
				}
				retrieveStartTime := tk.ScheduledEnd.Add(-time.Duration(reqAtt) * time.Second)
				
				if !currentTime.Before(retrieveStartTime) {
					tk.Status = models.TaskPending
					logSim(vTime, fmt.Sprintf("Máy chạy xong cho task %s -> Cần RETRIEVE", resolveStepNameStress(tk)))
				}
			}
		}

		// 3. Staff actions (Start/Complete tasks)
		for _, staff := range []string{"baker_john", "baker_alice"} {
			var activeTask *models.StaffTask
			var pendingTasks []*models.StaffTask

			for _, tk := range tasks {
				if tk.AssignedTo == staff {
					if tk.Status == models.TaskActive {
						activeTask = tk
					} else if tk.Status == models.TaskPending || tk.Status == models.TaskWaiting {
						pendingTasks = append(pendingTasks, tk)
					}
				}
			}

			// Sort pending tasks by scheduled start time
			sort.Slice(pendingTasks, func(i, j int) bool {
				return pendingTasks[i].ScheduledStart.Before(pendingTasks[j].ScheduledStart)
			})

			if activeTask != nil {
				var duration time.Duration
				step := sopRepo.steps[activeTask.SOPStepID]
				if activeTask.TaskKind == models.TaskKindNormal {
					duration = time.Duration(step.Duration) * time.Second
				} else if activeTask.TaskKind == models.TaskKindSetup {
					duration = time.Duration(*step.ActiveTime) * time.Second
				} else if activeTask.TaskKind == models.TaskKindRetrieve {
					reqAtt := 0
					if step.RequiresAttentionAt != nil {
						reqAtt = *step.RequiresAttentionAt
					}
					duration = time.Duration(reqAtt) * time.Second
					if duration == 0 {
						duration = 1 * time.Minute // Mặc định 1 phút để test
					}
				} else if activeTask.TaskKind == models.TaskKindFillIn {
					duration = time.Duration(step.Duration) * time.Second
				}

				if currentTime.Sub(*activeTask.StartedAt) >= duration {
					activeTask.Status = models.TaskDone
					endTime := currentTime
					activeTask.CompletedAt = &endTime
					logSim(vTime, fmt.Sprintf("Nhân viên %s hoàn thành %s", staff, resolveStepNameStress(activeTask)))

					if activeTask.MachineID != "" && activeTask.TaskKind != models.TaskKindSetup {
						if activeTask.TaskKind == models.TaskKindRetrieve || activeTask.TaskKind == models.TaskKindNormal {
							machineRepo.machines[activeTask.MachineID].CurrentBatchID = nil // Free machine
						}
					}
					
					disp.Dispatch(ctx, nodeID)
					activeTask = nil // They are free now!
				}
			}

			if activeTask == nil {
				for _, tk := range pendingTasks {
					if tk.Status == models.TaskPending {
						if !currentTime.Before(tk.EarliestStart) && !currentTime.Before(tk.ScheduledStart) {
							// Đặc biệt với RETRIEVE, KDS chỉ cho phép bấm khi gần tới giờ
							if tk.TaskKind == models.TaskKindRetrieve {
								step := sopRepo.steps[tk.SOPStepID]
								reqAtt := 0
								if step != nil && step.RequiresAttentionAt != nil {
									reqAtt = *step.RequiresAttentionAt
								}
								retrieveStartTime := tk.ScheduledEnd.Add(-time.Duration(reqAtt) * time.Second)
								if currentTime.Before(retrieveStartTime) {
									continue // Chưa đến giờ lấy, bỏ qua
								}
							}

							tk.Status = models.TaskActive
							startTime := currentTime
							tk.StartedAt = &startTime
							logSim(vTime, fmt.Sprintf("Nhân viên %s bắt đầu %s", staff, resolveStepNameStress(tk)))
							break
						}
					}
				}
			}
		}
	}

	fmt.Println("\n=== KẾT QUẢ MÔ PHỎNG (VIRTUAL TIMELINE) ===")
	for _, l := range simulatedLogs {
		fmt.Println(l)
	}

	// 4. Kiểm tra Assertions
	t.Run("A1_AllTasksDone", func(t *testing.T) {
		for _, tk := range taskRepo.tasks {
			if tk.Status != models.TaskDone && tk.Status != models.TaskCancelled {
				t.Errorf("Task %s is not DONE. Status: %s. Name: %s", tk.ID, tk.Status, resolveStepNameStress(tk))
			}
		}
	})
	
	// A2: Check Staff Overlap
	t.Run("A2_NoStaffOverlap", func(t *testing.T) {
		for _, staff := range []string{"baker_john", "baker_alice"} {
			var staffTasks []*models.StaffTask
			for _, tk := range taskRepo.tasks {
				if tk.AssignedTo == staff && tk.StartedAt != nil && tk.CompletedAt != nil {
					staffTasks = append(staffTasks, tk)
				}
			}
			for i := 0; i < len(staffTasks); i++ {
				for j := i + 1; j < len(staffTasks); j++ {
					a, b := staffTasks[i], staffTasks[j]
					if a.StartedAt.Before(*b.CompletedAt) && b.StartedAt.Before(*a.CompletedAt) {
						t.Errorf("Staff overlap: %s đang làm %s (%s - %s) VÀ %s (%s - %s)",
							staff, resolveStepNameStress(a), a.StartedAt.Format("15:04"), a.CompletedAt.Format("15:04"),
							resolveStepNameStress(b), b.StartedAt.Format("15:04"), b.CompletedAt.Format("15:04"))
					}
				}
			}
		}
	})

}
