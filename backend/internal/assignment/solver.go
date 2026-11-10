// Package assignment solves joint staff/room allocation without I/O or shared state.
package assignment

import (
	"context"
	"errors"
	"sort"
	"time"
)

type Pair struct{ SpecialistID, RoomID string }
type Job struct {
	ID, ServiceID        string
	Start, OccupiedUntil time.Time
	Fixed                *Pair
}
type Outcome string

const (
	SAT     Outcome = "SAT"
	UNSAT   Outcome = "UNSAT"
	UNKNOWN Outcome = "UNKNOWN"
)

type Result struct {
	Outcome Outcome
	Witness map[string]Pair
	Nodes   int
}

func Overlaps(a, b Job) bool {
	return a.Start.Before(b.OccupiedUntil) && b.Start.Before(a.OccupiedUntil)
}

// Component returns the entire transitive overlap component anchored at id.
func Component(jobs []Job, id string) []Job {
	selected := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, a := range jobs {
			if selected[a.ID] {
				continue
			}
			for _, b := range jobs {
				if selected[b.ID] && Overlaps(a, b) {
					selected[a.ID] = true
					changed = true
					break
				}
			}
		}
	}
	out := make([]Job, 0)
	for _, j := range jobs {
		if selected[j.ID] {
			out = append(out, j)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func Solve(ctx context.Context, jobs []Job, domains map[string][]Pair, maxNodes int) (Result, error) {
	result := Result{Outcome: UNSAT}
	seen := map[string]bool{}
	choices := make(map[string][]Pair, len(jobs))
	assigned := map[string]Pair{}
	for _, j := range jobs {
		if seen[j.ID] || j.ID == "" || !j.Start.Before(j.OccupiedUntil) {
			return result, errors.New("invalid assignment jobs")
		}
		seen[j.ID] = true
		unique := map[Pair]bool{}
		for _, p := range domains[j.ID] {
			if p.SpecialistID != "" && p.RoomID != "" && (j.Fixed == nil || *j.Fixed == p) {
				unique[p] = true
			}
		}
		for p := range unique {
			choices[j.ID] = append(choices[j.ID], p)
		}
		sort.Slice(choices[j.ID], func(a, b int) bool {
			x, y := choices[j.ID][a], choices[j.ID][b]
			if x.SpecialistID != y.SpecialistID {
				return x.SpecialistID < y.SpecialistID
			}
			return x.RoomID < y.RoomID
		})
	}
	compatible := func(j Job, p Pair) bool {
		for _, other := range jobs {
			if q, ok := assigned[other.ID]; ok && Overlaps(j, other) && (q.SpecialistID == p.SpecialistID || q.RoomID == p.RoomID) {
				return false
			}
		}
		return true
	}
	var visit func() Outcome
	visit = func() Outcome {
		if ctx.Err() != nil || result.Nodes >= maxNodes {
			return UNKNOWN
		}
		result.Nodes++
		if len(assigned) == len(jobs) {
			return SAT
		}
		var chosen Job
		var best []Pair
		found := false
		for _, j := range jobs {
			if _, ok := assigned[j.ID]; ok {
				continue
			}
			available := make([]Pair, 0, len(choices[j.ID]))
			for _, p := range choices[j.ID] {
				if compatible(j, p) {
					available = append(available, p)
				}
			}
			if len(available) == 0 {
				return UNSAT
			}
			if !found || len(available) < len(best) || (len(available) == len(best) && j.ID < chosen.ID) {
				chosen = j
				best = available
				found = true
			}
		}
		for _, p := range best {
			assigned[chosen.ID] = p
			out := visit()
			if out == SAT {
				return SAT
			}
			delete(assigned, chosen.ID)
			if out == UNKNOWN {
				return UNKNOWN
			}
		}
		return UNSAT
	}
	result.Outcome = visit()
	if result.Outcome == SAT {
		result.Witness = assigned
	}
	return result, nil
}
