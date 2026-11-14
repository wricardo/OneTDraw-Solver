package solver

import (
	"sync"
	"testing"
	"time"
)

// level54Puzzle is the bundled puzzle whose walk space is too large to
// enumerate, which makes it the right subject for budget and cancel tests.
func level54Puzzle(t *testing.T) *Puzzle {
	t.Helper()
	blob, err := readPuzzleFile("../puzzles/level54.json")
	if err != nil {
		t.Fatalf("read puzzle: %v", err)
	}
	p, err := NewPuzzleFromBytes(blob)
	if err != nil {
		t.Fatalf("parse puzzle: %v", err)
	}
	return p
}

// The reachability pruning must not change which solutions exist. Comparing
// against the documented counts of the bundled puzzles catches both a prune
// that is too aggressive (loss) and one that is wrong (gain).
func TestPruningPreservesSolutionCounts(t *testing.T) {
	cases := []struct {
		file  string
		count int
	}{
		{"regular_triangle", 6},
		{"directional_triangle", 3},
		{"house", 88},
		{"jamaican_flag", 44},
		{"littlet", 18},
		{"level53", 3648},
		{"level56", 576},
		{"level55", 28032},
		{"level57", 90624},
		{"jose", 0},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			blob, err := readPuzzleFile("../puzzles/" + tc.file + ".json")
			if err != nil {
				t.Fatalf("read puzzle: %v", err)
			}
			p, err := NewPuzzleFromBytes(blob)
			if err != nil {
				t.Fatalf("parse puzzle: %v", err)
			}
			if got := GetNumberOfSolutions(p); got != tc.count {
				t.Errorf("%s: expected %d solutions, got %d", tc.file, tc.count, got)
			}
		})
	}
}

// Every solution the solver returns must actually be a legal walk: consecutive
// nodes joined by an edge, each edge used exactly as many times as allowed.
func TestSolutionsAreValidWalks(t *testing.T) {
	blob, err := readPuzzleFile("../puzzles/level53.json")
	if err != nil {
		t.Fatalf("read puzzle: %v", err)
	}
	p, err := NewPuzzleFromBytes(blob)
	if err != nil {
		t.Fatalf("parse puzzle: %v", err)
	}

	solutions := Solve(p)
	if len(*solutions) == 0 {
		t.Fatal("expected solutions")
	}

	for _, sol := range *solutions {
		budget := make(map[[2]uint16]int)
		for _, e := range p.Edges {
			budget[[2]uint16{e.PointA, e.PointB}] += int(e.Count)
			budget[[2]uint16{e.PointB, e.PointA}] += int(e.Count)
		}
		for i := 0; i+1 < len(sol); i++ {
			a, b := sol[i], sol[i+1]
			key := [2]uint16{a, b}
			if budget[key] == 0 {
				t.Fatalf("walk uses unavailable edge %d->%d in %v", a, b, sol)
			}
			// Decrement both orientations so a two-way edge can only be spent
			// as often as its Count allows.
			budget[[2]uint16{a, b}]--
			budget[[2]uint16{b, a}]--
		}
		for key, left := range budget {
			if left != 0 {
				t.Fatalf("walk %v left %d traversals of edge %v", sol, left, key)
			}
		}
	}
}

// A budget must return promptly with partial results rather than running on.
func TestSolveWithinRespectsBudget(t *testing.T) {
	p := level54Puzzle(t)
	budget := 250 * time.Millisecond

	started := time.Now()
	solutions, timedOut := SolveWithin(p, 0, budget)
	elapsed := time.Since(started)

	if !timedOut {
		t.Error("expected the search to report a timeout")
	}
	if len(*solutions) == 0 {
		t.Error("expected some solutions before the deadline")
	}
	// Allow generous slack for a loaded machine, but catch a budget that is
	// ignored entirely (the unbounded search takes far longer than this).
	if elapsed > 20*budget {
		t.Errorf("budget not respected: took %v for a %v budget", elapsed, budget)
	}
}

// Closing the cancel channel must stop the search quickly.
func TestSolveWithCancelStops(t *testing.T) {
	p := level54Puzzle(t)
	cancel := make(chan struct{})

	go func() {
		time.Sleep(100 * time.Millisecond)
		close(cancel)
	}()

	started := time.Now()
	_, stats := SolveWith(p, 0, 0, nil, cancel)
	elapsed := time.Since(started)

	if !stats.Cancelled {
		t.Error("expected stats to report cancellation")
	}
	if stats.Exhaustive {
		t.Error("a cancelled search cannot be exhaustive")
	}
	if elapsed > 10*time.Second {
		t.Errorf("cancel was not honoured promptly: %v", elapsed)
	}
}

// Reaching the solution limit is not the same as being cancelled or timing out.
func TestLimitIsNotCancellation(t *testing.T) {
	p := level54Puzzle(t)
	solutions, stats := SolveWith(p, 100, 0, nil, nil)

	if len(*solutions) != 100 {
		t.Errorf("expected exactly 100 solutions, got %d", len(*solutions))
	}
	if !stats.Limited {
		t.Error("expected stats to report the limit was reached")
	}
	if stats.Cancelled {
		t.Error("hitting the limit is not a cancellation")
	}
	if stats.TimedOut {
		t.Error("hitting the limit is not a timeout")
	}
	if stats.Exhaustive {
		t.Error("stopping at the limit is not exhaustive")
	}
}

// A search that finishes on its own must be reported as exhaustive, so the UI
// can say "exhaustive" instead of warning about truncation.
func TestSmallPuzzleIsExhaustive(t *testing.T) {
	blob, err := readPuzzleFile("../puzzles/house.json")
	if err != nil {
		t.Fatalf("read puzzle: %v", err)
	}
	p, err := NewPuzzleFromBytes(blob)
	if err != nil {
		t.Fatalf("parse puzzle: %v", err)
	}

	solutions, stats := SolveWith(p, 20000, 10*time.Second, nil, nil)
	if len(*solutions) != 88 {
		t.Errorf("expected 88 solutions, got %d", len(*solutions))
	}
	if !stats.Exhaustive {
		t.Error("a completed search under the limit should be exhaustive")
	}
	if stats.Elapsed <= 0 {
		t.Error("expected a non-zero elapsed time")
	}
}

// Progress must be reported during a long search, and throttled rather than
// emitted once per solution.
func TestProgressIsReportedAndThrottled(t *testing.T) {
	p := level54Puzzle(t)

	var mu sync.Mutex
	calls := 0
	var lastFound int
	progress := func(stats SolveStats) {
		mu.Lock()
		calls++
		if stats.Found < lastFound {
			t.Errorf("progress went backwards: %d after %d", stats.Found, lastFound)
		}
		lastFound = stats.Found
		mu.Unlock()
	}

	solutions, _ := SolveWith(p, 20000, 2*time.Second, progress, nil)

	mu.Lock()
	defer mu.Unlock()
	if calls == 0 {
		t.Fatal("expected progress callbacks")
	}
	if len(*solutions) > 0 && calls > len(*solutions) {
		t.Errorf("progress not throttled: %d callbacks for %d solutions", calls, len(*solutions))
	}
}

// Counting must report incompleteness rather than passing a partial number off
// as the total.
func TestCountReportsIncompleteness(t *testing.T) {
	p := level54Puzzle(t)

	count, stats := CountWith(p, 200*time.Millisecond, nil)
	if stats.Exhaustive {
		t.Error("a budgeted count of this puzzle should not be exhaustive")
	}
	if !stats.TimedOut {
		t.Error("expected a timeout to be reported")
	}
	if count == 0 {
		t.Error("expected a partial count")
	}
}

// The documented total must still be produced when the count is allowed to run.
func TestCountIsCompleteWithoutBudget(t *testing.T) {
	blob, err := readPuzzleFile("../puzzles/level57.json")
	if err != nil {
		t.Fatalf("read puzzle: %v", err)
	}
	p, err := NewPuzzleFromBytes(blob)
	if err != nil {
		t.Fatalf("parse puzzle: %v", err)
	}

	count, stats := CountWith(p, 0, nil)
	if count != 90624 {
		t.Errorf("expected 90624, got %d", count)
	}
	if !stats.Exhaustive {
		t.Error("expected an exhaustive count")
	}
}
