package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"one-system-server/internal/usecase"
)

// A small mock of the timeline logic to see the output
type MachineAllocation struct {
	Start time.Time
	End   time.Time
	Slots float64
}
type MachineTimeline struct {
	Allocations []MachineAllocation
	MaxCapacity float64
}
func (mt *MachineTimeline) AddAllocation(start, end time.Time, slots float64) {
	mt.Allocations = append(mt.Allocations, MachineAllocation{Start: start, End: end, Slots: slots})
}
func (mt *MachineTimeline) FindEarliestWindow(earliestStart time.Time, duration time.Duration, reqSlots float64) time.Time {
    // I will just copy the logic from machine_timeline.go
	return time.Time{}
}

func main() {
	fmt.Println("Hello")
}
