package assignment

import (
	"context"
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
