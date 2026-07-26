package usecase

import (
	"testing"
	"time"
)

func TestMachineTimeline_FindEarliestWindow(t *testing.T) {
	now := time.Now()
	
	// Create a timeline with max capacity of 10 slots
	timeline := &MachineTimeline{
		Allocations: make([]MachineAllocation, 0),
		MaxCapacity: 10.0,
	}

	// Add some allocations
	timeline.AddAllocation(now, now.Add(10*time.Minute), 5.0)
	timeline.AddAllocation(now.Add(5*time.Minute), now.Add(15*time.Minute), 5.0)

	w1 := timeline.FindEarliestWindow(now, 2*time.Minute, 4.0)
	if !w1.Equal(now) {
		t.Errorf("w1 expected %v, got %v", now, w1)
	}

	w2 := timeline.FindEarliestWindow(now, 10*time.Minute, 4.0)
	if !w2.Equal(now.Add(10*time.Minute)) {
		t.Errorf("w2 expected %v, got %v", now.Add(10*time.Minute), w2)
	}

	w3 := timeline.FindEarliestWindow(now, 5*time.Minute, 6.0)
	if !w3.Equal(now.Add(15*time.Minute)) {
		t.Errorf("w3 expected %v, got %v", now.Add(15*time.Minute), w3)
	}
	
	w4 := timeline.FindEarliestWindow(now, 1*time.Minute, 12.0)
	if !w4.IsZero() {
		t.Errorf("w4 expected zero time, got %v", w4)
	}
}
