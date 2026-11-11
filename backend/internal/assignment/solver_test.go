package assignment

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

func job(id string, start, end int) Job {
	b := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	return Job{ID: id, Start: b.Add(time.Duration(start) * time.Minute), OccupiedUntil: b.Add(time.Duration(end) * time.Minute)}
}
func TestJointAllocation(t *testing.T) {
	a, b := job("A", 0, 60), job("B", 0, 60)
	p, q := Pair{"S1", "R1"}, Pair{"S2", "R2"}
	for _, tt := range []struct {
		name    string
		domains map[string][]Pair
		want    Outcome
	}{{"one room", map[string][]Pair{"A": {p}, "B": {{"S2", "R1"}}}, UNSAT}, {"two pairs", map[string][]Pair{"A": {p, q}, "B": {p, q}}, SAT}, {"greedy trap", map[string][]Pair{"A": {p, q}, "B": {p}}, SAT}} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Solve(context.Background(), []Job{a, b}, tt.domains, 100)
			if err != nil || r.Outcome != tt.want {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	a.Fixed = &p
	r, _ := Solve(context.Background(), []Job{a, b}, map[string][]Pair{"A": {p, q}, "B": {p}}, 100)
	if r.Outcome != UNSAT {
		t.Fatal("fixed assignment moved")
	}
	r, _ = Solve(context.Background(), []Job{a}, map[string][]Pair{"A": {p}}, 0)
	if r.Outcome != UNKNOWN {
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, _ = Solve(ctx, []Job{a}, map[string][]Pair{"A": {p}}, 100)
	if r.Outcome != UNKNOWN {
		t.Fatal(r)
	}
}
func TestComponentAndBuffer(t *testing.T) {
	a, b, c, d := job("a", 0, 75), job("b", 60, 120), job("c", 100, 160), job("d", 160, 180)
	if got := Component([]Job{a, b, c, d}, "a"); len(got) != 3 {
		t.Fatal(got)
	}
	if Overlaps(c, d) {
		t.Fatal("touching intervals overlap")
	}
}

func TestSolverAgainstExhaustiveEnumeration(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 73))
	pairs := []Pair{{"S1", "R1"}, {"S1", "R2"}, {"S2", "R1"}, {"S2", "R2"}}
	for sample := 0; sample < 150; sample++ {
		jobs := []Job{}
		domains := map[string][]Pair{}
		for i := 0; i < 5; i++ {
			id := fmt.Sprint(i)
			start := rng.IntN(4) * 30
			j := job(id, start, start+45)
			for _, p := range pairs {
				if rng.IntN(2) == 0 {
					domains[id] = append(domains[id], p)
				}
			}
			if rng.IntN(4) == 0 && len(domains[id]) > 0 {
				p := domains[id][0]
				j.Fixed = &p
			}
			jobs = append(jobs, j)
		}
		chosen := make([]Pair, len(jobs))
		var brute func(int) bool
		brute = func(n int) bool {
			if n == len(jobs) {
				return true
			}
			j := jobs[n]
			for _, p := range domains[j.ID] {
				if j.Fixed != nil && *j.Fixed != p {
					continue
				}
				allowed := true
				for i := 0; i < n; i++ {
					other := jobs[i]
					if j.Start.Before(other.OccupiedUntil) && other.Start.Before(j.OccupiedUntil) && (p.SpecialistID == chosen[i].SpecialistID || p.RoomID == chosen[i].RoomID) {
						allowed = false
					}
				}
				if allowed {
					chosen[n] = p
					if brute(n + 1) {
						return true
					}
				}
			}
			return false
		}
		want := brute(0)
		result, e := Solve(context.Background(), jobs, domains, 100000)
		if e != nil || result.Outcome == UNKNOWN || (result.Outcome == SAT) != want {
			t.Fatalf("sample %d: %+v %v expected SAT %v", sample, result, e, want)
		}
	}
}
