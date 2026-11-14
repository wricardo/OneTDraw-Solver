package solver

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Solution []uint16

type Direction struct {
	From           uint16
	To             uint16
	Unidirectional bool
}

type Edge struct {
	PointA    uint16
	PointB    uint16
	Direction Direction
	Count     uint16
}

func (this *Edge) visit() {
	if this.Count > 0 {
		this.Count = this.Count - 1
	} else {
		panic("trying to visit already visited edge")
	}
}

func (this *Edge) Copy() Edge {
	c := Edge{}
	c.PointB = this.PointB
	c.PointA = this.PointA
	c.Count = this.Count
	c.Direction = this.Direction

	return c
}

func (this *Edge) canGoTo(pointFrom uint16, pointTo uint16) bool {
	if this.Count == 0 {
		return false
	}
	if this.Direction.Unidirectional == true {
		if this.Direction.From == pointFrom && this.Direction.To == pointTo {
			return true
		}
	} else {
		return true
	}
	return false
}

type Puzzle struct {
	Edges []Edge
	count uint16
}

func NewPuzzle(edges []Edge) *Puzzle {
	total_count := countTotalEdges(&edges)
	np := Puzzle{Edges: edges, count: total_count}
	return &np
}

func NewPuzzleFromBytes(puzzle_bytes []byte) (*Puzzle, error) {
	var p Puzzle
	err := json.Unmarshal(puzzle_bytes, &p)
	if err != nil {
		return nil, err
	}
	return NewPuzzle(p.Edges), nil
}

func countTotalEdges(edges *[]Edge) (total_count uint16) {
	total_count = 0
	for _, v := range *edges {
		total_count += v.Count
	}
	return
}

func (this *Puzzle) Copy() Puzzle {
	c := Puzzle{}
	c.count = this.count
	for _, v := range this.Edges {
		c.Edges = append(c.Edges, v.Copy())
	}
	return c
}

func (this *Puzzle) isSolved() bool {
	return this.count == 0
}

func (this *Puzzle) listStartingPoints() []uint16 {
	sp := make([]uint16, 0, len(this.Edges)*2)
	for _, edge := range this.Edges {
		sp = append(sp, edge.PointA)
		sp = append(sp, edge.PointB)
	}
	sp = removeDuplicates(sp)
	return sp
}

func (this *Puzzle) getEdge(e1 *uint16, e2 *uint16) (*Edge, error) {
	for k, edge := range this.Edges {
		if edge.PointA == *e1 && edge.PointB == *e2 {
			return &this.Edges[k], nil
		} else if edge.PointA == *e2 && edge.PointB == *e1 {
			return &this.Edges[k], nil
		}
	}
	return nil, errors.New("edge not found")
}

func (this *Puzzle) visitEdge(edge *Edge) {
	this.count = this.count - 1
	edge.visit()
}

func (this *Puzzle) listPossibleEdgesToVisit(from *uint16) []uint16 {
	r := make([]uint16, 0, len(this.Edges))
	for _, edge := range this.Edges {
		if edge.PointA == *from && edge.canGoTo(edge.PointA, edge.PointB) {
			r = append(r, edge.PointB)
		}
		if edge.PointB == *from && edge.canGoTo(edge.PointB, edge.PointA) {
			r = append(r, edge.PointA)
		}
	}
	r = removeDuplicates(r)
	return r
}

// solveControl lets every exploration goroutine see the shared stop signal and
// the running total of solutions found, so a search can be bounded by a
// solution limit, a deadline, or a caller's cancel.
type solveControl struct {
	limit       int64     // <= 0 means unlimited
	deadline    time.Time // zero means no deadline
	stop        int32
	canceled    int32 // set only by cancel(), not by the limit or the deadline
	deadlineHit int32 // set when the deadline stops the search
	mu          sync.Mutex
	found       int64
	calls       int64
	visited     int64
	progress    func(found int64, visited int64)
	lastPing    time.Time
}

// stopping reports whether a cancel, a deadline, or the solution limit has been
// reached. It is checked on every node of the search.
func (this *solveControl) stopping() bool {
	if atomic.LoadInt32(&this.stop) == 1 {
		return true
	}
	if this.deadline.IsZero() {
		return false
	}
	// Sampling the clock: the recursive search calls this far more often than a
	// clock read is worth, so only every 1024th call actually looks.
	if atomic.AddInt64(&this.calls, 1)&1023 != 0 {
		return false
	}
	if time.Now().After(this.deadline) {
		atomic.StoreInt32(&this.deadlineHit, 1)
		atomic.StoreInt32(&this.stop, 1)
		return true
	}
	return false
}

// cancel stops the search at the next node it visits.
func (this *solveControl) cancel() {
	atomic.StoreInt32(&this.canceled, 1)
	atomic.StoreInt32(&this.stop, 1)
}

// cancelled reports whether the caller stopped the search, as opposed to it
// reaching its solution limit or its deadline.
func (this *solveControl) cancelled() bool { return atomic.LoadInt32(&this.canceled) == 1 }

// limitReached reports whether the search stopped because it had enough.
func (this *solveControl) limitReached() bool {
	return this.limit > 0 && atomic.LoadInt64(&this.found) >= this.limit
}

// timedOut reports whether the deadline stopped the search.
func (this *solveControl) timedOut() bool { return atomic.LoadInt32(&this.deadlineHit) == 1 }

// offer records a found solution and reports whether the limit is now reached.
// Progress is throttled: a big puzzle can find thousands of solutions a second,
// and an event per solution would swamp the client for no added information.
func (this *solveControl) offer() bool {
	this.mu.Lock()
	this.found++
	done := this.limit > 0 && this.found >= this.limit
	found := this.found
	emit := this.progress != nil && time.Since(this.lastPing) >= progressInterval
	if emit {
		this.lastPing = time.Now()
	}
	this.mu.Unlock()

	if done {
		atomic.StoreInt32(&this.stop, 1)
	}
	if emit {
		this.progress(found, atomic.LoadInt64(&this.visited))
	}
	return done
}

// progressInterval is how often the search may report progress.
const progressInterval = 120 * time.Millisecond

// remainingReachableFrom reports whether every edge still needing a traversal
// can be reached from node without crossing an already-used edge.
//
// This is the pruning the naive search lacked. Once the unused edges split into
// groups no longer joined to each other, no continuation can cover them all, so
// the whole subtree is dead and is abandoned before it is explored. It only
// discards branches that provably have no completions, so the set of solutions
// found is unchanged.
func (this *Puzzle) remainingReachableFrom(node uint16) bool {
	remaining := 0
	for _, e := range this.Edges {
		remaining += int(e.Count)
	}
	if remaining == 0 {
		return true
	}

	seen := map[uint16]bool{node: true}
	queue := []uint16{node}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for i := range this.Edges {
			e := &this.Edges[i]
			if e.Count == 0 {
				continue
			}
			if e.PointA == cur && !seen[e.PointB] {
				seen[e.PointB] = true
				queue = append(queue, e.PointB)
			}
			if e.PointB == cur && !seen[e.PointA] {
				seen[e.PointA] = true
				queue = append(queue, e.PointA)
			}
		}
	}

	for _, e := range this.Edges {
		if e.Count == 0 {
			continue
		}
		if !seen[e.PointA] || !seen[e.PointB] {
			return false
		}
	}
	return true
}

// boundedStorer wraps a SolutionHandler and stops the search once ctl is done.
type boundedStorer struct {
	inner SolutionHandler
	ctl   *solveControl
}

func (this *boundedStorer) handleNewSolutionFound(path *[]uint16) {
	if this.ctl.stopping() {
		return
	}
	this.inner.handleNewSolutionFound(path)
	this.ctl.offer()
}

func findSolutions(puzzle *Puzzle, starting *uint16, path []uint16, solution_handler SolutionHandler, level uint16, wg *sync.WaitGroup, ctl *solveControl) {
	if level == 1 {
		defer wg.Done()
	}
	if ctl.stopping() {
		return
	}
	atomic.AddInt64(&ctl.visited, 1)

	path = append(path, *starting)
	possible_edges := puzzle.listPossibleEdgesToVisit(starting)

	if len(possible_edges) == 0 {
		if puzzle.isSolved() {
			solution_handler.handleNewSolutionFound(&path)
		}
		return
	}

	for k := range possible_edges {
		previous_count := puzzle.count
		edge, err := puzzle.getEdge(starting, &possible_edges[k])
		if err != nil {
			continue
		}
		previous_edgecount := edge.Count
		puzzle.visitEdge(edge)

		// Abandon this branch unless the edges still owed can all be reached
		// from where the walk now stands.
		if puzzle.remainingReachableFrom(possible_edges[k]) {
			findSolutions(puzzle, &possible_edges[k], path, solution_handler, level+1, wg, ctl)
		}
		edge.Count = previous_edgecount
		puzzle.count = previous_count

		if ctl.stopping() {
			return
		}
	}
}

// SolveLimited returns at most limit solutions. A limit <= 0 means unlimited.
// Stopping early does not change the solutions found before the limit was hit.
func SolveLimited(puzzle *Puzzle, limit int) *Solutions {
	solutions, _ := solveBounded(puzzle, solveOptions{limit: int64(limit)})
	return solutions
}

// SolveWithin returns whatever it found before the deadline, which is a partial
// answer rather than an empty one for searches too large to finish.
func SolveWithin(puzzle *Puzzle, limit int, budget time.Duration) (*Solutions, bool) {
	solutions, stats := solveBounded(puzzle, solveOptions{
		limit:    int64(limit),
		deadline: time.Now().Add(budget),
	})
	return solutions, stats.TimedOut
}

// SolveStats describes how a bounded search ended.
type SolveStats struct {
	Found      int           // solutions returned
	TimedOut   bool          // the deadline stopped the search
	Cancelled  bool          // the caller's cancel stopped the search
	Limited    bool          // the solution limit was reached
	Visited    int64         // search nodes explored
	Elapsed    time.Duration // wall clock spent searching
	Exhaustive bool          // the whole search space was covered
}

// SolveWith runs a search that reports progress and can be cancelled. progress,
// if non-nil, is called from inside the search as solutions arrive; it must not
// block. Closing cancel, if non-nil, stops the search at the next node visited.
//
// This is what the web UI uses: a big puzzle keeps streaming progress to the
// browser and the user can stop it instead of waiting on a blocked request.
func SolveWith(puzzle *Puzzle, limit int, budget time.Duration, progress func(SolveStats), cancel <-chan struct{}) (*Solutions, SolveStats) {
	var deadline time.Time
	if budget > 0 {
		deadline = time.Now().Add(budget)
	}
	opts := solveOptions{limit: int64(limit), deadline: deadline, progress: progress, cancel: cancel}

	started := time.Now()
	solutions, stats := solveBounded(puzzle, opts)
	stats.Elapsed = time.Since(started)
	return solutions, stats
}

type solveOptions struct {
	limit    int64
	deadline time.Time
	progress func(SolveStats)
	cancel   <-chan struct{}
}

func solveBounded(puzzle *Puzzle, opts solveOptions) (*Solutions, SolveStats) {
	starting_points := puzzle.listStartingPoints()
	arr_solution_storer := make([]solutionStorer, len(starting_points))
	var wg sync.WaitGroup
	ctl := &solveControl{limit: opts.limit, deadline: opts.deadline}
	if opts.progress != nil {
		ctl.progress = func(found, visited int64) {
			opts.progress(SolveStats{Found: int(found), Visited: visited})
		}
	}

	// Bridge the caller's cancel channel onto the shared stop flag the recursion
	// already checks, so cancellation costs one goroutine rather than one
	// select per search node.
	var cancelDone chan struct{}
	if opts.cancel != nil {
		cancelDone = make(chan struct{})
		go func() {
			select {
			case <-opts.cancel:
				ctl.cancel()
			case <-cancelDone:
			}
		}()
	}

	for k := range starting_points {
		arr_solution_storer[k] = *newSolutionStorer()
		wg.Add(1)
		path := make([]uint16, 0)
		pc := puzzle.Copy()
		storer := &boundedStorer{inner: &arr_solution_storer[k], ctl: ctl}
		go findSolutions(&pc, &starting_points[k], path, storer, 1, &wg, ctl)
	}

	wg.Wait()
	if cancelDone != nil {
		close(cancelDone)
	}

	to_return := make(Solutions, 0)
	for _, solutions := range arr_solution_storer {
		for _, solution := range solutions.solutions {
			to_return = append(to_return, *solution)
		}
	}

	return &to_return, SolveStats{
		Found:      len(to_return),
		TimedOut:   ctl.timedOut(),
		Cancelled:  ctl.cancelled(),
		Limited:    ctl.limitReached(),
		Visited:    atomic.LoadInt64(&ctl.visited),
		Exhaustive: !ctl.cancelled() && !ctl.timedOut() && !ctl.limitReached(),
	}
}

func Solve(puzzle *Puzzle) *Solutions {
	solutions, _ := solveBounded(puzzle, solveOptions{})
	return solutions
}

type Solutions []Solution

func (this *Solutions) Print(printer SolutionPrinter) error {
	printer.Print(this)
	return nil
}

func GetNumberOfSolutions(puzzle *Puzzle) int {
	count, _ := CountWith(puzzle, 0, nil)
	return count
}

// CountWith counts solutions, optionally stopping at a budget or when cancel is
// closed. The stats say whether the count is complete, which matters because a
// large puzzle's count cannot always be finished.
func CountWith(puzzle *Puzzle, budget time.Duration, cancel <-chan struct{}) (int, SolveStats) {
	return CountWithProgress(puzzle, budget, cancel, nil)
}

// CountWithProgress is CountWith plus a progress callback, so a long count can
// report how many walks and branches it has seen instead of appearing hung.
func CountWithProgress(puzzle *Puzzle, budget time.Duration, cancel <-chan struct{}, progress func(SolveStats)) (int, SolveStats) {
	var deadline time.Time
	if budget > 0 {
		deadline = time.Now().Add(budget)
	}
	starting_points := puzzle.listStartingPoints()
	arr_solutions_count := make([]solutionCounter, len(starting_points))
	var wg sync.WaitGroup
	ctl := &solveControl{deadline: deadline}
	if progress != nil {
		ctl.progress = func(found, visited int64) {
			progress(SolveStats{Found: int(found), Visited: visited})
		}
	}

	var cancelDone chan struct{}
	if cancel != nil {
		cancelDone = make(chan struct{})
		go func() {
			select {
			case <-cancel:
				ctl.cancel()
			case <-cancelDone:
			}
		}()
	}

	started := time.Now()
	for k := range starting_points {
		arr_solutions_count[k] = *newSolutionCounter()
		wg.Add(1)
		path := make([]uint16, 0)
		pc := puzzle.Copy()
		counter := &boundedStorer{inner: &arr_solutions_count[k], ctl: ctl}
		go findSolutions(&pc, &starting_points[k], path, counter, 1, &wg, ctl)
	}

	wg.Wait()
	if cancelDone != nil {
		close(cancelDone)
	}

	to_return := 0
	for _, solutions := range arr_solutions_count {
		to_return = to_return + solutions.count_solutions
	}

	return to_return, SolveStats{
		Found:      to_return,
		TimedOut:   ctl.timedOut(),
		Cancelled:  ctl.cancelled(),
		Visited:    atomic.LoadInt64(&ctl.visited),
		Elapsed:    time.Since(started),
		Exhaustive: !ctl.cancelled() && !ctl.timedOut(),
	}
}

func removeDuplicates(s []uint16) []uint16 {
	seen := make(map[uint16]struct{}, len(s))
	result := make([]uint16, 0, len(s))
	for _, v := range s {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
