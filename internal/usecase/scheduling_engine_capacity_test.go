package usecase

import (
	"fmt"
	"testing"
	"time"

	"one-system-server/internal/domain/models"
)

func TestSchedulingEngine_T10_CapacityAndBatching(t *testing.T) {
	ctx, poRepo, sopRepo, _, shiftRepo, taskRepo, machineRepo, disp, engine := setupTestEnv()

	nowBase := time.Date(2026, 7, 16, 12, 0, 0, 0, time.Local) // 12:00 PM
	nodeID := "store_01"

	// Staff: 2 nhân viên
	shiftRepo.shifts["shift_1"] = &models.StaffShift{ID: "shift_1", NodeID: nodeID, StaffID: "staff_1", Status: models.ShiftActive}
	shiftRepo.shifts["shift_2"] = &models.StaffShift{ID: "shift_2", NodeID: nodeID, StaffID: "staff_2", Status: models.ShiftActive}

	// Machine: Bếp nướng (Grill) có 10 slots
	machineRepo.machines["m_grill"] = &models.Machine{ID: "m_grill", EquipmentTypeID: "grill", NodeID: nodeID, Status: models.MachineIdle, MaxCapacity: 10}

	// SOP: Bò nướng (TargetQty = 14 miếng bò, mỗi miếng 1 slot -> Total = 14 slots)
	// Machine chỉ có 10 slots -> phải chia làm 2 mẻ (mẻ 1: 10 miếng, mẻ 2: 4 miếng).
	sopBurger := "sop_beef"
	sopRepo.sops[sopBurger] = &models.SOP{ID: sopBurger} // Tạm bỏ qua Name vì mock ko có field Name

	sopRepo.steps["s1_grill"] = &models.SOPStep{
		ID: "s1_grill", SOPID: sopBurger, SeqNo: 1,
		EquipmentTypeID: ptrStrStress("grill"), IsIdleStep: true, Duration: 5 * 60, ActiveTime: ptrIntStress(1 * 60),
		SlotConsumption: 1.0, // 1 slot per unit
	}

	// Order: 14 miếng bò
	po1 := &models.ProductionOrder{
		ID: "po_1", NodeID: nodeID, SOPID: sopBurger, Status: models.POInProgress, 
		CreatedAt: nowBase, TargetQty: 14,
	}
	poRepo.pos["po_1"] = po1

	// SIMULATION LOOP
	maxVirtualTime := 30 // 30 minutes
	simulatedLogs := make([]string, 0)
	
	logSim := func(vTime int, msg string) {
		tStr := fmt.Sprintf("T+%02d [%s]", vTime, nowBase.Add(time.Duration(vTime)*time.Minute).Format("15:04"))
		simulatedLogs = append(simulatedLogs, fmt.Sprintf("%s %s", tStr, msg))
		fmt.Println(simulatedLogs[len(simulatedLogs)-1])
	}

	for vTime := 0; vTime <= maxVirtualTime; vTime++ {
		currentTime := nowBase.Add(time.Duration(vTime) * time.Minute)
		engine.(*schedulingEngine).now = func() time.Time { return currentTime }
		disp.(*dispatcher).now = func() time.Time { return currentTime }

		// Trigger Order
		if vTime == 0 {
			logSim(vTime, "Khách order 14 miếng bò nướng")
			_, err := engine.SchedulePO(ctx, po1.ID)
			if err != nil {
				t.Fatalf("SchedulePO failed: %v", err)
			}
			disp.Dispatch(ctx, nodeID)
		}

		// Staff Action Loop
		pendingTasks, _ := taskRepo.FindByNode(ctx, nodeID, []models.TaskStatus{models.TaskPending})
		activeTasks, _ := taskRepo.FindByNode(ctx, nodeID, []models.TaskStatus{models.TaskActive})

		for _, staff := range []string{"staff_1", "staff_2"} {
			var activeTask *models.StaffTask
			for _, tk := range activeTasks {
				if tk.AssignedTo == staff {
					activeTask = tk
					break
				}
			}

			if activeTask != nil {
				step := sopRepo.steps[activeTask.SOPStepID]
				duration := time.Duration(step.Duration) * time.Second

				if activeTask.TaskKind == models.TaskKindSetup {
					activeTime := 0
					if step.ActiveTime != nil {
						activeTime = *step.ActiveTime
					}
					duration = time.Duration(activeTime) * time.Second
				} else if activeTask.TaskKind == models.TaskKindRetrieve {
					reqAtt := 0
					if step.RequiresAttentionAt != nil {
						reqAtt = *step.RequiresAttentionAt
					}
					duration = time.Duration(reqAtt) * time.Second
					if duration == 0 {
						duration = 1 * time.Minute
					}
				}

				if currentTime.Sub(*activeTask.StartedAt) >= duration {
					activeTask.Status = models.TaskDone
					logSim(vTime, fmt.Sprintf("Nhân viên %s hoàn thành %s (Qty: %.1f)", staff, resolveStepNameStress(activeTask), activeTask.TargetQty))
					taskRepo.Update(ctx, activeTask)
					disp.Dispatch(ctx, nodeID)
					activeTask = nil
				}
			}

			if activeTask == nil {
				for _, tk := range pendingTasks {
					if tk.Status == models.TaskPending {
						if !currentTime.Before(tk.EarliestStart) && !currentTime.Before(tk.ScheduledStart) {
							if tk.TaskKind == models.TaskKindRetrieve {
								step := sopRepo.steps[tk.SOPStepID]
								reqAtt := 0
								if step != nil && step.RequiresAttentionAt != nil {
									reqAtt = *step.RequiresAttentionAt
								}
								retrieveStartTime := tk.ScheduledEnd.Add(-time.Duration(reqAtt) * time.Second)
								if currentTime.Before(retrieveStartTime) {
									continue
								}
							}

							tk.Status = models.TaskActive
							startTime := currentTime
							tk.StartedAt = &startTime
							tk.AssignedTo = staff
							logSim(vTime, fmt.Sprintf("Nhân viên %s bắt đầu %s (Qty: %.1f, Máy: %s)", staff, resolveStepNameStress(tk), tk.TargetQty, tk.MachineID))
							taskRepo.Update(ctx, tk)
							break
						}
					}
				}
			}
		}
	}

	// A3 Assertion
	t.Run("A3_MachineCapacityExceeded", func(t *testing.T) {
		allTasks, _ := taskRepo.FindByPO(ctx, po1.ID)
		setupCount := 0
		for _, tk := range allTasks {
			if tk.SOPStepID == "s1_grill" && tk.TaskKind == models.TaskKindSetup {
				setupCount++
				if tk.RequiredSlots > 10 {
					t.Errorf("Task %s vượt capacity: %.1f > 10", tk.ID, tk.RequiredSlots)
				}
			}
		}
		if setupCount != 2 {
			t.Errorf("Expected 2 SETUP tasks for s1_grill due to batching, got %d", setupCount)
		} else {
			t.Logf("✅ Đã chia thành %d mẻ thành công", setupCount)
		}
	})
}
