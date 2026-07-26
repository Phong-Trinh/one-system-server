package main
import (
	"fmt"
	"time"
)
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
	if reqSlots > mt.MaxCapacity { return time.Time{} }
	candidates := []time.Time{earliestStart}
	for _, a := range mt.Allocations {
		if !a.End.Before(earliestStart) { candidates = append(candidates, a.End) }
	}
	for _, cand := range candidates {
		candEnd := cand.Add(duration)
		canFit := true
		events := []time.Time{cand}
		for _, a := range mt.Allocations {
			if a.Start.Before(candEnd) && a.End.After(cand) {
				if a.Start.After(cand) && a.Start.Before(candEnd) { events = append(events, a.Start) }
				if a.End.After(cand) && a.End.Before(candEnd) { events = append(events, a.End) }
			}
		}
		for _, ev := range events {
			used := 0.0
			for _, a := range mt.Allocations {
				if !ev.Before(a.Start) && ev.Before(a.End) { used += a.Slots }
			}
			if used+reqSlots > mt.MaxCapacity {
				canFit = false
				break
			}
		}
		if canFit { return cand }
	}
	return earliestStart
}
func main() {
	mt := &MachineTimeline{MaxCapacity: 48}
	t0, _ := time.Parse("15:04", "07:30")
	mt.AddAllocation(t0, t0.Add(15*time.Minute), 48) // 07:30 - 07:45
	mt.AddAllocation(t0.Add(15*time.Minute), t0.Add(30*time.Minute), 48) // 07:45 - 08:00
	tStart, _ := time.Parse("15:04", "07:48")
	w := mt.FindEarliestWindow(tStart, 15*time.Minute, 48)
	fmt.Printf("Window: %v\n", w.Format("15:04"))
}
