package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tri is a closed triangle: three nodes, three unit edges.
func tri() MapConfig {
	return MapConfig{
		ID: "tri", Name: "Triangle",
		Points: []PointConfig{{Point: 1, Level: 1}, {Point: 2, Level: 1}, {Point: 3, Level: 1}},
		Edges: []EdgeConfig{
			{PointA: 1, PointB: 2},
			{PointA: 2, PointB: 3},
			{PointA: 3, PointB: 1},
		},
	}
}

func mustMap(t *testing.T, cfg MapConfig) *Map {
	t.Helper()
	m, err := NewMap(cfg)
	if err != nil {
		t.Fatalf("NewMap: %v", err)
	}
	return m
}

func TestNewMapDefaultsAndAnalysis(t *testing.T) {
	m := mustMap(t, tri())
	if len(m.Edges) != 3 {
		t.Fatalf("edges = %d, want 3", len(m.Edges))
	}
	// Omitted Count means 1.
	for _, e := range m.Edges {
		if e.Count != 1 {
			t.Errorf("edge %d-%d count = %d, want 1", e.A, e.B, e.Count)
		}
	}
	if m.StepCount != 3 {
		t.Errorf("StepCount = %d, want 3", m.StepCount)
	}
	if m.MaxMoves != 3 {
		t.Errorf("MaxMoves = %d, want the step count 3", m.MaxMoves)
	}
	if m.Analysis.Verdict != VerdictCircuit {
		t.Errorf("verdict = %s, want CIRCUIT", m.Analysis.Verdict)
	}
	if len(m.Analysis.OddNodes) != 0 {
		t.Errorf("odd nodes = %v, want none in a closed tour", m.Analysis.OddNodes)
	}
	if m.Par != 3 {
		t.Errorf("Par = %d, want 3", m.Par)
	}
}

func TestParityVerdicts(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MapConfig
		verdict Verdict
		odd     int
	}{
		{
			name:    "closed tour",
			cfg:     tri(),
			verdict: VerdictCircuit,
			odd:     0,
		},
		{
			// A path 1-2-3 is an open trail: it must start at an end.
			name: "open trail",
			cfg: MapConfig{
				ID: "path", Name: "Path",
				Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}},
			},
			verdict: VerdictTrail,
			odd:     2,
		},
		{
			// A star with four leaves has four odd nodes: no single trail.
			name: "four odd nodes is impossible",
			cfg: MapConfig{
				ID: "star", Name: "Star",
				Edges: []EdgeConfig{
					{PointA: 1, PointB: 2}, {PointA: 1, PointB: 3},
					{PointA: 1, PointB: 4}, {PointA: 1, PointB: 5},
				},
			},
			verdict: VerdictImpossible,
			odd:     4,
		},
		{
			// Directed edges break the undirected parity argument.
			name: "directed edges",
			cfg: MapConfig{
				ID: "dir", Name: "Directed",
				Edges: []EdgeConfig{
					{PointA: 1, PointB: 2, Direction: &Direction{From: 1, To: 2, Unidirectional: true}},
					{PointA: 2, PointB: 3},
				},
			},
			verdict: VerdictDirected,
			// Degrees are counted undirected even for one-way edges, so the ends
			// of the chain are odd; the verdict is DIRECTED, which ignores them.
			odd: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := mustMap(t, tc.cfg)
			if m.Analysis.Verdict != tc.verdict {
				t.Errorf("verdict = %s, want %s", m.Analysis.Verdict, tc.verdict)
			}
			if len(m.Analysis.OddNodes) != tc.odd {
				t.Errorf("odd nodes = %v, want %d", m.Analysis.OddNodes, tc.odd)
			}
		})
	}
}

func TestDisconnectedIsImpossible(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "split", Name: "Split",
		Edges: []EdgeConfig{
			{PointA: 1, PointB: 2},
			{PointA: 3, PointB: 4},
		},
	})
	if m.Analysis.Verdict != VerdictImpossible {
		t.Fatalf("verdict = %s, want IMPOSSIBLE for two separate edges", m.Analysis.Verdict)
	}
	if m.Analysis.Connected {
		t.Error("Connected = true, want false")
	}
	if m.Par != -1 {
		t.Errorf("Par = %d, want -1 for an unsolvable map", m.Par)
	}
}

func TestValidationRejectsBadMaps(t *testing.T) {
	tests := []struct {
		name string
		cfg  MapConfig
	}{
		{"no id", MapConfig{Name: "X", Edges: []EdgeConfig{{PointA: 1, PointB: 2}}}},
		{"bad id", MapConfig{ID: "Not Valid", Name: "X", Edges: []EdgeConfig{{PointA: 1, PointB: 2}}}},
		{"no name", MapConfig{ID: "x", Edges: []EdgeConfig{{PointA: 1, PointB: 2}}}},
		{"no edges", MapConfig{ID: "x", Name: "X"}},
		{"self loop", MapConfig{ID: "x", Name: "X", Edges: []EdgeConfig{{PointA: 1, PointB: 1}}}},
		{"negative count", MapConfig{ID: "x", Name: "X", Edges: []EdgeConfig{{PointA: 1, PointB: 2, Count: -3}}}},
		{"count too large", MapConfig{ID: "x", Name: "X", Edges: []EdgeConfig{{PointA: 1, PointB: 2, Count: MaxEdgeCount + 1}}}},
		{
			"direction does not match endpoints",
			MapConfig{ID: "x", Name: "X", Edges: []EdgeConfig{
				{PointA: 1, PointB: 2, Direction: &Direction{From: 1, To: 3, Unidirectional: true}},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewMap(tc.cfg); err == nil {
				t.Fatal("expected a validation error, got nil")
			}
		})
	}
}

func TestStartAndTravel(t *testing.T) {
	m := mustMap(t, tri())
	s := NewState(m)

	// Travel before starting is refused, not counted.
	if _, err := s.Travel(m, 2); err != ErrNotStarted {
		t.Fatalf("Travel before Start: err = %v, want ErrNotStarted", err)
	}
	if s.Moves != 0 {
		t.Errorf("Moves = %d after a refused travel, want 0", s.Moves)
	}

	if ev, err := s.Start(m, 1); err != nil || ev.Blocked {
		t.Fatalf("Start: ev=%+v err=%v", ev, err)
	}
	if s.Pos != 1 || s.Status != StatusPlaying {
		t.Fatalf("after Start: pos=%d status=%s", s.Pos, s.Status)
	}
	if s.Moves != 0 {
		t.Errorf("Moves = %d after Start, want 0 (starting is not a counted move)", s.Moves)
	}

	// Draw 1-2.
	ev, err := s.Travel(m, 2)
	if err != nil || ev.Blocked {
		t.Fatalf("Travel 1->2: ev=%+v err=%v", ev, err)
	}
	if ev.Kind != StepDraw || s.Moves != 1 || s.Remaining != 2 {
		t.Fatalf("after travel: kind=%s moves=%d remaining=%d", ev.Kind, s.Moves, s.Remaining)
	}
	if s.Pos != 2 {
		t.Errorf("Pos = %d, want 2", s.Pos)
	}
}

func TestBlockedMoveChangesNothing(t *testing.T) {
	m := mustMap(t, tri())
	s := NewState(m)
	s.Start(m, 1)

	// 1 and 3 are joined, so use a 4-node map for a true non-neighbour.
	m2 := mustMap(t, MapConfig{
		ID: "q", Name: "Q",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 3, PointB: 4}},
	})
	s2 := NewState(m2)
	s2.Start(m2, 1)
	before := s2

	ev, err := s2.Travel(m2, 3)
	if err != nil {
		t.Fatalf("Travel: %v", err)
	}
	if !ev.Blocked || ev.Reason == "" {
		t.Fatalf("expected a blocked step with a reason, got %+v", ev)
	}
	if s2.Moves != before.Moves || s2.Remaining != before.Remaining || s2.Pos != before.Pos {
		t.Errorf("a blocked step changed state: moves=%d remaining=%d pos=%d", s2.Moves, s2.Remaining, s2.Pos)
	}
	if s2.Blocked != 1 {
		t.Errorf("Blocked = %d, want 1", s2.Blocked)
	}
	_ = s
}

func TestRepeatedEdgeNeedsTwoTraversals(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "dbl", Name: "Doubled",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2, Count: 2}},
	})
	s := NewState(m)
	s.Start(m, 1)

	if ev, _ := s.Travel(m, 2); ev.Blocked {
		t.Fatalf("first traversal blocked: %s", ev.Reason)
	}
	if s.Status == StatusWon {
		t.Fatal("won after only one of two traversals")
	}
	if ev, _ := s.Travel(m, 1); ev.Blocked {
		t.Fatalf("second traversal blocked: %s", ev.Reason)
	}
	if s.Status != StatusWon {
		t.Fatalf("status = %s after both traversals, want WON", s.Status)
	}
	// A third traversal is refused: the game is over.
	if _, err := s.Travel(m, 2); err != ErrTerminal {
		t.Errorf("travel after WON: err = %v, want ErrTerminal", err)
	}
}

func TestOneWayEdge(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "oneway", Name: "One way",
		Edges: []EdgeConfig{
			{PointA: 1, PointB: 2, Direction: &Direction{From: 1, To: 2, Unidirectional: true}},
		},
	})
	// Backwards is blocked.
	s := NewState(m)
	s.Start(m, 2)
	ev, _ := s.Travel(m, 1)
	if !ev.Blocked {
		t.Fatal("walking a one-way edge backwards was allowed")
	}
	// Forwards works.
	s2 := NewState(m)
	s2.Start(m, 1)
	ev, _ = s2.Travel(m, 2)
	if ev.Blocked {
		t.Fatalf("walking a one-way edge forwards was blocked: %s", ev.Reason)
	}
	if s2.Status != StatusWon {
		t.Errorf("status = %s, want WON", s2.Status)
	}
}

func TestDeadEndLoses(t *testing.T) {
	// Two edges in a row: 1-2 and 2-3. Starting at 2 and going to 1 strands 2-3.
	m := mustMap(t, MapConfig{
		ID: "chain", Name: "Chain",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}},
	})
	s := NewState(m)
	s.Start(m, 2)
	ev, _ := s.Travel(m, 1)
	if ev.Blocked {
		t.Fatalf("travel was blocked: %s", ev.Reason)
	}
	if s.Status != StatusLost {
		t.Fatalf("status = %s, want LOST after stranding edge 2-3", s.Status)
	}
	if s.LossReason != LossDeadEnd {
		t.Errorf("loss reason = %q, want DEAD_END", s.LossReason)
	}
}

func TestUndoRestoresTheEdge(t *testing.T) {
	m := mustMap(t, tri())
	s := NewState(m)
	s.Start(m, 1)
	s.Travel(m, 2)
	s.Travel(m, 3)

	ev, ok := s.Undo(m)
	if !ok {
		t.Fatal("Undo reported nothing to undo")
	}
	if ev.Edge < 0 {
		t.Error("undone event has no edge id")
	}
	if s.Pos != 2 || s.Moves != 1 || s.Remaining != 2 {
		t.Errorf("after undo: pos=%d moves=%d remaining=%d, want 2/1/2", s.Pos, s.Moves, s.Remaining)
	}
	// Undoing back to the start stops; the first node is not an edge.
	s.Undo(m)
	if _, ok := s.Undo(m); ok {
		t.Error("Undo removed the starting node, which is not a drawn edge")
	}
	// The edge is usable again.
	if ev, _ := s.Travel(m, 2); ev.Blocked {
		t.Errorf("edge not usable after undo: %s", ev.Reason)
	}
}

func TestUndoClearsLost(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "chain2", Name: "Chain",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}},
	})
	// A path 1-2-3 started at the middle end 2 is a losing start; from 1 it is
	// winnable. The point of this test is that Undo clears the loss flag.
	s := NewState(m)
	s.Start(m, 2)
	s.Travel(m, 1) // strands 2-3
	if s.Status != StatusLost {
		t.Fatal("expected LOST after stranding an edge")
	}
	s.Undo(m)
	if s.Status != StatusPlaying {
		t.Errorf("status after undo = %s, want PLAYING", s.Status)
	}
	if s.Pos != 2 {
		t.Errorf("pos after undo = %d, want 2", s.Pos)
	}
	// From 2 the other branch is taken by undoing fully and restarting at 1.
	s.Undo(m) // back to the starting node, no edge undone
	s2 := NewState(m)
	s2.Start(m, 1)
	if ev, _ := s2.Travel(m, 2); ev.Blocked {
		t.Fatalf("cannot draw 1-2: %s", ev.Reason)
	}
	if ev, _ := s2.Travel(m, 3); ev.Blocked {
		t.Fatalf("cannot draw 2-3: %s", ev.Reason)
	}
	if s2.Status != StatusWon {
		t.Errorf("status = %s, want WON", s2.Status)
	}
}

func TestPlanSolvesEveryBundledMap(t *testing.T) {
	files, err := filepath.Glob("../../maps/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no maps found")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			blob, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var cfg MapConfig
			if err := json.Unmarshal(blob, &cfg); err != nil {
				t.Fatal(err)
			}
			m := mustMap(t, cfg)

			plan, err := Plan(m, NewState(m))
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			solvable := m.Analysis.Verdict != VerdictImpossible
			if plan.Found != solvable {
				t.Fatalf("Plan.Found = %v, want %v (verdict %s)", plan.Found, solvable, m.Analysis.Verdict)
			}
			if !plan.Found {
				return
			}
			// The plan must be a real walk that draws every edge.
			if err := ValidSolution(m, plan.Path()); err != nil {
				t.Fatalf("plan is not a valid solution: %v", err)
			}
			if got := len(plan.Steps); got != m.StepCount {
				t.Errorf("plan length = %d, want StepCount %d", got, m.StepCount)
			}
		})
	}
}

func TestPlanFromMidTrail(t *testing.T) {
	m := mustMap(t, tri())
	s := NewState(m)
	s.Start(m, 1)
	s.Travel(m, 3) // draw 1-3

	plan, err := Plan(m, s)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Found {
		t.Fatal("no completion found from node 3")
	}
	if plan.Start != 3 {
		t.Errorf("plan start = %d, want the current node 3", plan.Start)
	}
	if len(plan.Steps) != 2 {
		t.Errorf("steps = %d, want 2 remaining", len(plan.Steps))
	}
	full := plan.FullPathFrom(s)
	if err := ValidSolution(m, full); err != nil {
		t.Errorf("completion invalid: %v", err)
	}
	// The plan alone is not a solution from scratch: it must be continued from
	// where the trail already stands.
	if err := ValidSolution(m, plan.Path()); err == nil {
		t.Error("a mid-trail plan was accepted as a complete trail")
	}
}

func TestPlanReportsLost(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "c", Name: "C",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}},
	})
	s := NewState(m)
	s.Start(m, 2)
	s.Travel(m, 1) // stranded 2-3
	plan, err := Plan(m, s)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Found {
		t.Error("Plan found a completion from a lost state")
	}
}

func TestHint(t *testing.T) {
	m := mustMap(t, tri())
	next, ok := Hint(m, NewState(m))
	if !ok {
		t.Fatal("Hint found no move on a solvable map")
	}
	if !m.NodeExists(next) {
		t.Errorf("hint %d is not a node of the map", next)
	}
}

func TestPlanRespectsDeadline(t *testing.T) {
	m := mustMap(t, tri())
	// A zero budget must return promptly with whatever it has (nothing).
	started := time.Now()
	plan, err := planWithin(m, NewState(m), 0)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second {
		t.Error("planWithin overran its budget")
	}
	_ = plan // Found may be either; the point is that it returned.
}

func TestValidSolutionRejectsShortPath(t *testing.T) {
	m := mustMap(t, tri())
	if err := ValidSolution(m, []int{1, 2}); err == nil {
		t.Fatal("a path leaving an edge undrawn was accepted")
	}
	if err := ValidSolution(m, []int{1, 2, 3, 1}); err != nil {
		t.Fatalf("a complete trail was rejected: %v", err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	cfg := MapConfig{
		ID: "rt", Name: "Round Trip", Description: "d", Difficulty: "hard", MaxMoves: 9,
		Points: []PointConfig{
			{Point: 1, Level: 1}, {Point: 2, Level: 2},
			// A layout-only spacer: hidden and joined to nothing.
			{Point: 3, Level: 2, Hide: true},
		},
		Edges: []EdgeConfig{
			{PointA: 1, PointB: 2, Count: 2},
			{PointA: 2, PointB: 4, Direction: &Direction{From: 2, To: 4, Unidirectional: true}},
		},
	}
	m := mustMap(t, cfg)
	blob, err := json.Marshal(m.Config())
	if err != nil {
		t.Fatal(err)
	}
	var back MapConfig
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatal(err)
	}
	m2, err := NewMap(back)
	if err != nil {
		t.Fatalf("re-parsing a round-tripped map: %v", err)
	}
	if len(m2.Edges) != len(m.Edges) || m2.StepCount != m.StepCount {
		t.Errorf("round trip changed the map: edges %d vs %d, steps %d vs %d",
			len(m2.Edges), len(m.Edges), m2.StepCount, m.StepCount)
	}
	if !m2.Edges[1].Directed() {
		t.Error("round trip lost the one-way constraint")
	}
	// Hidden spacer points survive.
	if len(m2.Config().Points) != 3 {
		t.Errorf("round trip dropped points: got %d, want 3", len(m2.Config().Points))
	}
	// The hidden spacer is carried through untouched.
	if !m2.Config().Points[2].Hidden() {
		t.Error("round trip lost the hidden flag on a spacer point")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	m := mustMap(t, tri())
	c := m.Clone()
	c.Edges[0].Count = 99
	c.Nodes[0].Pos = Vec{X: 1, Y: 1}
	if m.Edges[0].Count == 99 {
		t.Error("editing a clone changed the original's edges")
	}
	if m.Nodes[0].Pos.X == 1 {
		t.Error("editing a clone changed the original's nodes")
	}
}

func TestCloneDoesNotReplan(t *testing.T) {
	// Clone must be a pure copy; a map that took seconds to plan would make
	// per-move snapshots unusable otherwise.
	m := mustMap(t, tri())
	started := time.Now()
	for i := 0; i < 200; i++ {
		_ = m.Clone()
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Errorf("200 clones took %s; Clone must not re-run the solver", elapsed)
	}
}

func TestLayoutSeparatesNodes(t *testing.T) {
	// A three-node map with no levels at all used to collapse onto one line,
	// which draws a triangle as a single segment. Every node must stay apart.
	m := mustMap(t, MapConfig{
		ID: "flat", Name: "Flat",
		Points: []PointConfig{{Point: 1}, {Point: 2}, {Point: 3}},
		Edges:  []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}, {PointA: 3, PointB: 1}},
	})
	const minGap = 60 // comfortably more than the 26-unit node radius
	for i := 0; i < len(m.Nodes); i++ {
		for j := i + 1; j < len(m.Nodes); j++ {
			a, b := m.Nodes[i].Pos, m.Nodes[j].Pos
			dx, dy := float64(a.X-b.X), float64(a.Y-b.Y)
			if d := dx*dx + dy*dy; d < minGap*minGap {
				t.Errorf("nodes %d and %d are only %.0f apart (want >= %d)",
					m.Nodes[i].ID, m.Nodes[j].ID, sqrt(d), minGap)
			}
		}
	}
	// A collinear layout would leave every node on one y.
	sameY := true
	for _, n := range m.Nodes[1:] {
		if n.Pos.Y != m.Nodes[0].Pos.Y {
			sameY = false
		}
	}
	if sameY {
		t.Error("all nodes share one y coordinate: the layout collapsed to a line")
	}
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	// Newton's method, to keep this test dependency-free.
	x := v
	for i := 0; i < 30; i++ {
		x = (x + v/x) / 2
	}
	return x
}

func TestStepEventReportsBlockReason(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "dup", Name: "Dup",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 3, PointB: 4}},
	})
	s := NewState(m)
	s.Start(m, 1)
	ev, _ := s.Travel(m, 3)
	if !ev.Blocked || ev.Reason == "" {
		t.Fatalf("expected a reasoned block, got %+v", ev)
	}
	if ev.From != 1 || ev.To != 1 {
		t.Errorf("blocked event moved the pen: from=%d to=%d", ev.From, ev.To)
	}
	if ev.Edge != -1 {
		t.Errorf("blocked event reports edge %d, want -1", ev.Edge)
	}
}

func TestUsageAndPath(t *testing.T) {
	m := mustMap(t, MapConfig{
		ID: "u", Name: "U",
		Edges: []EdgeConfig{{PointA: 1, PointB: 2, Count: 3}},
	})
	s := NewState(m)
	if got := s.Usage(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("initial usage = %v, want [3]", got)
	}
	s.Start(m, 1)
	s.Travel(m, 2)
	s.Travel(m, 1)
	if got := s.Usage(); got[0] != 1 {
		t.Errorf("usage = %v, want [1] after two of three traversals", got)
	}
	if len(s.Path) != 3 {
		t.Errorf("path = %v, want three nodes", s.Path)
	}
}
