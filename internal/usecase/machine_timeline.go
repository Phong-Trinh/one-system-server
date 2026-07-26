package usecase

import (
	"sort"
	"time"
)

// MachineAllocation represents a time block where a specific capacity of the machine is used.
type MachineAllocation struct {
	Start time.Time
	End   time.Time
	Slots float64
}

// MachineTimeline manages the capacity of a machine over time.
type MachineTimeline struct {
	Allocations []MachineAllocation
	MaxCapacity float64
}

// AddAllocation registers a new capacity usage on the timeline.
func (mt *MachineTimeline) AddAllocation(start, end time.Time, slots float64) {
	if slots <= 0 || !end.After(start) {
		return
	}
	mt.Allocations = append(mt.Allocations, MachineAllocation{Start: start, End: end, Slots: slots})
}

// FindEarliestWindow finds the earliest time >= earliestStart where the machine has `reqSlots` available continuously for `duration`.
func (mt *MachineTimeline) FindEarliestWindow(earliestStart time.Time, duration time.Duration, reqSlots float64) time.Time {
	if reqSlots > mt.MaxCapacity {
		return time.Time{} // Cannot fit at all
	}

	// Candidate start times: earliestStart, and the End time of every existing allocation.
	candidates := []time.Time{earliestStart}
	for _, a := range mt.Allocations {
		if !a.End.Before(earliestStart) {
			candidates = append(candidates, a.End)
		}
	}

	// Sort candidates in ascending order
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Before(candidates[j])
	})

	for _, cand := range candidates {
		candEnd := cand.Add(duration)
		canFit := true

		// Collect all event times (starts and ends) within the window [cand, candEnd)
		events := []time.Time{cand}
		for _, a := range mt.Allocations {
			// If allocation overlaps with our candidate window
			if a.Start.Before(candEnd) && a.End.After(cand) {
				if a.Start.After(cand) && a.Start.Before(candEnd) {
					events = append(events, a.Start)
				}
				if a.End.After(cand) && a.End.Before(candEnd) {
					events = append(events, a.End)
				}
			}
		}

		// Sort events to evaluate capacity incrementally or at each point
		sort.Slice(events, func(i, j int) bool {
			return events[i].Before(events[j])
		})

		// At each event point, check if total used slots + reqSlots > MaxCapacity
		for _, ev := range events {
			used := 0.0
			for _, a := range mt.Allocations {
				// if `ev` is inside the allocation [Start, End)
				if !ev.Before(a.Start) && ev.Before(a.End) {
					used += a.Slots
				}
			}
			if used+reqSlots > mt.MaxCapacity {
				canFit = false
				break
			}
		}

		if canFit {
			return cand
		}
	}

	return earliestStart
}
