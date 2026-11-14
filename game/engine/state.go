package engine

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Status is the lifecycle of one trail.
type Status string

const (
	// StatusReady: the trail has not been started.
	StatusReady Status = "READY"
	// StatusPlaying: the trail is under way.
	StatusPlaying Status = "PLAYING"
	// StatusWon: every edge has been drawn.
	StatusWon Status = "WON"
	// StatusLost: the trail is stuck with edges left to draw.
	StatusLost Status = "LOST"
)

// Terminal reports whether the status is WON or LOST.
func (s Status) Terminal() bool { return s == StatusWon || s == StatusLost }

// LossReason explains a LOST status.
type LossReason string

const (
	LossNone LossReason = ""
	// LossDeadEnd: the current node has no usable edge while edges remain.
	LossDeadEnd LossReason = "DEAD_END"
)

// StepKind describes how an attempt changed the trail.
type StepKind string

const (
	// StepNone: the attempt was blocked; nothing changed.
	StepNone StepKind = "NONE"
	// StepDraw: an edge was traversed and its count decremented.
	StepDraw StepKind = "DRAW"
	// StepStart: the first node of the trail was placed.
	StepStart StepKind = "START"
)

// State is the mutable state of one trail.
type State struct {
	// Pos is the current node; -1 until the trail is started.
	Pos        int        `json:"pos"`
	Status     Status     `json:"status"`
	LossReason LossReason `json:"loss_reason,omitempty"`
	// Moves counts drawn edges (blocked attempts are free).
	Moves int `json:"moves"`
	// Blocked counts rejected attempts.
	Blocked int `json:"blocked"`
	// Remaining is how many traversals are still owed.
	Remaining int `json:"remaining"`
	// Path lists the nodes of the trail in order; empty until started.
	Path []int `json:"path"`

	used  []int // remaining traversals per edge, index parallel to Map.Edges
	steps []int // edge index drawn by each move, parallel to Path[1:]
}

// NewState returns a fresh, unstarted state for m.
func NewState(m *Map) State {
	s := State{Pos: -1, Status: StatusReady, Remaining: m.StepCount, Path: []int{}}
	s.used = make([]int, len(m.Edges))
	for i, e := range m.Edges {
		s.used[i] = e.Count
	}
	return s
}

// Clone deep-copies the state.
func (s State) Clone() State {
	s.Path = append([]int(nil), s.Path...)
	s.used = append([]int(nil), s.used...)
	s.steps = append([]int(nil), s.steps...)
	return s
}

// ErrTerminal is returned when a move is attempted on a finished trail.
var ErrTerminal = errors.New("trail is already finished")

// ErrNotStarted is returned by Travel before the trail has a first node.
var ErrNotStarted = errors.New("trail has not been started")

// StepEvent describes one attempted step.
type StepEvent struct {
	// Move is the drawn-move index after this attempt (unchanged when blocked).
	Move int `json:"move"`
	// Node is the node the player tried to reach.
	Node int `json:"node"`
	From int `json:"from"` // -1 before the trail starts
	To   int `json:"to"`   // final position; equals From when blocked
	// Kind is NONE, DRAW or START.
	Kind StepKind `json:"kind"`
	// Edge is the edge index traversed, or -1.
	Edge int `json:"edge"`
	// Blocked is true when nothing was drawn.
	Blocked bool `json:"blocked"`
	// Reason explains a blocked attempt. Empty when the step succeeded.
	Reason     string     `json:"reason,omitempty"`
	Status     Status     `json:"status"`
	LossReason LossReason `json:"loss_reason,omitempty"`
}

// Start places the first node of the trail. Any node with edges works. Placing
// the first node is not a drawn move.
func (s *State) Start(m *Map, node int) (StepEvent, error) {
	if s.Status.Terminal() {
		return StepEvent{}, ErrTerminal
	}
	ev := StepEvent{Move: s.Moves, Node: node, From: -1, To: s.Pos, Edge: -1, Kind: StepNone}
	if s.Status != StatusReady {
		ev.Blocked = true
		ev.Reason = "the trail is already started"
		ev.Status = s.Status
		s.Blocked++
		return ev, nil
	}
	if !m.NodeExists(node) {
		ev.Blocked = true
		ev.Reason = fmt.Sprintf("no node %d", node)
		ev.Status = s.Status
		s.Blocked++
		return ev, nil
	}
	if len(m.incidentEdges(node)) == 0 {
		ev.Blocked = true
		ev.Reason = "that node has no edges"
		ev.Status = s.Status
		s.Blocked++
		return ev, nil
	}
	s.Pos = node
	s.Path = append(s.Path, node)
	s.Status = StatusPlaying
	ev.Kind = StepStart
	ev.To = node
	ev.Status = s.Status
	return ev, nil
}

// Travel attempts to walk from the current node to node to. A blocked attempt
// (no such edge, that copy already drawn, or a one-way edge walked backwards)
// changes nothing but is counted in Blocked.
func (s *State) Travel(m *Map, to int) (StepEvent, error) {
	if s.Status.Terminal() {
		return StepEvent{}, ErrTerminal
	}
	if s.Status == StatusReady {
		return StepEvent{}, ErrNotStarted
	}
	ev := StepEvent{Move: s.Moves, Node: to, From: s.Pos, To: s.Pos, Edge: -1, Kind: StepNone}

	if !m.NodeExists(to) {
		return s.block(ev, fmt.Sprintf("no node %d", to)), nil
	}
	edge, reason := s.pickEdge(m, s.Pos, to)
	if edge < 0 {
		return s.block(ev, reason), nil
	}

	s.used[edge]--
	s.Remaining--
	s.Moves++
	s.Pos = to
	s.Path = append(s.Path, to)
	s.steps = append(s.steps, edge)
	ev.Kind = StepDraw
	ev.Edge = edge
	ev.To = to
	ev.Move = s.Moves

	switch {
	case s.Remaining == 0:
		s.Status = StatusWon
	case !s.hasMove(m, s.Pos):
		s.Status = StatusLost
		s.LossReason = LossDeadEnd
	}
	ev.Status = s.Status
	ev.LossReason = s.LossReason
	return ev, nil
}

func (s *State) block(ev StepEvent, reason string) StepEvent {
	ev.Blocked = true
	ev.Reason = reason
	ev.Status = s.Status
	s.Blocked++
	return ev
}

// pickEdge returns the index of an edge usable from -> to, or -1 and why not.
// Among duplicate edges the lowest index with a traversal left wins, so the
// choice is deterministic.
func (s *State) pickEdge(m *Map, from, to int) (int, string) {
	sawEdge := false
	sawReversed := false
	for _, ei := range m.incidentEdges(from) {
		e := m.Edges[ei]
		if e.A != to && e.B != to {
			continue
		}
		if !e.Allows(from, to) {
			sawReversed = true
			continue
		}
		sawEdge = true
		if s.used[ei] > 0 {
			return ei, ""
		}
	}
	switch {
	case sawReversed:
		return -1, "that edge is one-way the other way"
	case sawEdge:
		return -1, "that edge is already fully drawn"
	default:
		return -1, fmt.Sprintf("nodes %d and %d are not joined", from, to)
	}
}

// hasMove reports whether node has any unused edge it may leave.
func (s *State) hasMove(m *Map, node int) bool {
	for _, ei := range m.incidentEdges(node) {
		if s.used[ei] == 0 {
			continue
		}
		e := m.Edges[ei]
		if !e.Directed() || e.Dir.From == node {
			return true
		}
	}
	return false
}

// Undo takes back the most recent drawn edge and clears a lost status. It
// reports whether there was anything to undo.
func (s *State) Undo(m *Map) (StepEvent, bool) {
	if len(s.Path) < 2 || len(s.steps) == 0 {
		return StepEvent{}, false
	}
	prev := s.Path[len(s.Path)-1]
	edge := s.steps[len(s.steps)-1]
	s.steps = s.steps[:len(s.steps)-1]
	s.Path = s.Path[:len(s.Path)-1]
	s.used[edge]++
	s.Remaining++
	s.Moves--
	if s.Moves < 0 {
		s.Moves = 0
	}
	s.Pos = s.Path[len(s.Path)-1]
	s.Status = StatusPlaying
	s.LossReason = LossNone
	return StepEvent{
		Move: s.Moves, Node: prev, From: prev, To: s.Pos, Kind: StepDraw, Edge: edge, Status: s.Status,
	}, true
}

// Usage returns the traversals still owed per edge, so a client can grey out
// edges that are used up.
func (s State) Usage() []int { return append([]int(nil), s.used...) }

// PlanResult is a completion plan from a state.
type PlanResult struct {
	// Start is the node the trail must begin at. It equals the state's current
	// position when the trail is already under way, and is the chosen first node
	// when planning a trail that has not been started.
	Start int
	// Steps are the nodes to travel to, in order, starting from Start.
	Steps []int
	// Found is false when no completion could be proven within the budget.
	Found bool
}

// Path returns the full node trail of a plan that starts a fresh trail: the
// start followed by every step.
func (p PlanResult) Path() []int {
	if !p.Found {
		return []int{}
	}
	return append([]int{p.Start}, p.Steps...)
}

// FullPathFrom returns the whole trail this plan yields when continued from an
// in-progress state: the nodes already drawn, followed by the remaining steps.
//
// Plan.Path is only the plan itself, so verifying a mid-trail plan against the
// map needs this, which is why both exist.
func (p PlanResult) FullPathFrom(s State) []int {
	if !p.Found {
		return nil
	}
	if s.Status == StatusReady {
		return p.Path()
	}
	out := append([]int(nil), s.Path...)
	return append(out, p.Steps...)
}

// defaultPlanBudget bounds a Plan call so a pathological map cannot hang a
// request. The bundled maps plan in a few hundred milliseconds, and the hardest
// needs about two seconds under the race detector, so the cap is generous
// rather than tight: it exists to bound a bad case, not to time a good one.
const defaultPlanBudget = 15 * time.Second

// Plan searches for a completion of the trail from the current state. When the
// trail has not been started it also chooses a legal start node.
//
// Every completion draws exactly the edges still owed, so all completions have
// the same length; Plan returns the first one it proves.
func Plan(m *Map, s State) (PlanResult, error) {
	return planWithin(m, s, defaultPlanBudget)
}

func planWithin(m *Map, s State, budget time.Duration) (PlanResult, error) {
	deadline := time.Now().Add(budget)
	switch s.Status {
	case StatusWon:
		return PlanResult{Found: true, Start: s.Pos, Steps: []int{}}, nil
	case StatusLost:
		return PlanResult{}, nil
	}
	st := s.Clone()
	// failed memoises subtrees already proven dead, keyed by position plus the
	// remaining traversals of every edge. It is shared across start attempts:
	// the same subproblem reached from a different opening is the same problem.
	failed := make(map[string]struct{}, 4096)

	if st.Status == StatusReady {
		for _, start := range m.startCandidates() {
			if time.Now().After(deadline) {
				return PlanResult{}, nil
			}
			trial := st.Clone()
			trial.Pos = start
			trial.Path = []int{start}
			trial.Status = StatusPlaying
			if path, ok := dfsPlan(m, &trial, make([]int, 0, trial.Remaining), deadline, failed); ok {
				return PlanResult{Found: true, Start: start, Steps: path}, nil
			}
		}
		return PlanResult{}, nil
	}

	if path, ok := dfsPlan(m, &st, make([]int, 0, st.Remaining), deadline, failed); ok {
		return PlanResult{Found: true, Start: st.Pos, Steps: path}, nil
	}
	return PlanResult{}, nil
}

// startCandidates lists the nodes Plan tries first: the parity-allowed starts
// when the map is undirected, else every node with an edge.
func (m *Map) startCandidates() []int {
	if !m.Analysis.Directional && len(m.Analysis.StartNodes) > 0 {
		return m.Analysis.StartNodes
	}
	var out []int
	for _, n := range m.Nodes {
		if len(m.incidentEdges(n.ID)) > 0 {
			out = append(out, n.ID)
		}
	}
	sort.Ints(out)
	return out
}

// dfsPlan is the depth-first search behind Plan. It returns the nodes to travel
// to, in order, once every owed traversal is drawn.
//
// It mutates s in place and undoes each move on the way out: allocating a state
// clone per branch made the search on the largest bundled map take seconds,
// whereas the apply/recurse/undo pattern costs one edge counter update.
func dfsPlan(m *Map, s *State, path []int, deadline time.Time, failed map[string]struct{}) ([]int, bool) {
	if s.Remaining == 0 {
		return append([]int(nil), path...), true
	}
	if time.Now().After(deadline) {
		return nil, false
	}
	key := s.memoKey()
	if _, dead := failed[key]; dead {
		return nil, false
	}
	if !s.feasible(m) {
		failed[key] = struct{}{}
		return nil, false
	}
	for _, next := range s.orderedMoves(m) {
		undo := s.applyMove(m, next)
		if undo == nil {
			continue
		}
		if s.Status == StatusLost {
			s.undoMove(undo)
			continue
		}
		path = append(path, next)
		out, ok := dfsPlan(m, s, path, deadline, failed)
		path = path[:len(path)-1]
		s.undoMove(undo)
		if ok {
			return out, true
		}
	}
	failed[key] = struct{}{}
	return nil, false
}

// moveUndo records what applyMove changed, so it can be reverted exactly.
type moveUndo struct {
	edge   int
	from   int
	prevSt Status
	prevLR LossReason
}

// applyMove draws edge to next in place, returning a token for undoMove, or nil
// when the move is not legal. It is the allocation-free equivalent of
// State.Travel.
func (s *State) applyMove(m *Map, to int) *moveUndo {
	edge, _ := s.pickEdge(m, s.Pos, to)
	if edge < 0 {
		return nil
	}
	u := &moveUndo{edge: edge, from: s.Pos, prevSt: s.Status, prevLR: s.LossReason}
	s.used[edge]--
	s.Remaining--
	s.Moves++
	s.Pos = to
	s.Status = StatusPlaying
	s.LossReason = LossNone
	// A walk whose last edge leaves no way on is dead; recording it here means
	// the caller can skip it without a second feasibility pass.
	if s.Remaining > 0 && !s.hasMove(m, s.Pos) {
		s.Status = StatusLost
		s.LossReason = LossDeadEnd
	}
	return u
}

// undoMove reverts applyMove exactly. It only ever rewinds a search branch, so
// it restores the recorded counters rather than recomputing them.
func (s *State) undoMove(u *moveUndo) {
	s.used[u.edge]++
	s.Remaining++
	s.Moves--
	s.Pos = u.from
	s.Status = u.prevSt
	s.LossReason = u.prevLR
}

// memoKey identifies a search state: the current node plus the traversals still
// owed on every edge.
func (s *State) memoKey() string {
	buf := make([]byte, 0, len(s.used)+8)
	buf = append(buf, byte(s.Pos), byte(s.Pos>>8), byte(s.Pos>>16), byte(s.Pos>>24))
	for _, left := range s.used {
		buf = append(buf, byte(left))
	}
	return string(buf)
}

// orderedMoves lists the nodes reachable by one drawn move from the current
// node. Warnsdorff ordering: the neighbour that leaves the fewest ways out of
// itself is tried first, so forced corridors are taken before open rooms and
// the walk does not strand edges behind it. Ties break on node id.
func (s *State) orderedMoves(m *Map) []int {
	type cand struct {
		node int
		deg  int
	}
	var cands []cand
	seen := map[int]bool{}
	for _, ei := range m.incidentEdges(s.Pos) {
		if s.used[ei] == 0 {
			continue
		}
		e := m.Edges[ei]
		if e.Directed() && e.Dir.From != s.Pos {
			continue
		}
		other := e.A
		if other == s.Pos {
			other = e.B
		}
		if seen[other] {
			continue
		}
		seen[other] = true
		cands = append(cands, cand{node: other, deg: s.exitCount(m, other)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].deg != cands[j].deg {
			return cands[i].deg < cands[j].deg
		}
		return cands[i].node < cands[j].node
	})
	out := make([]int, len(cands))
	for i, c := range cands {
		out[i] = c.node
	}
	return out
}

// exitCount is how many usable edges leave node, counting traversals.
func (s *State) exitCount(m *Map, node int) int {
	n := 0
	for _, ei := range m.incidentEdges(node) {
		if s.used[ei] == 0 {
			continue
		}
		e := m.Edges[ei]
		if e.Directed() && e.Dir.From != node {
			continue
		}
		n += s.used[ei]
	}
	return n
}

// feasible reports whether the edges still owed could possibly be drawn from
// the current position. It combines two sound necessary conditions:
//
//   - connectivity: every owed edge, and the departure end of every owed
//     one-way edge, must be reachable from here;
//   - parity, when only undirected edges remain: a walk starting here that
//     covers them all leaves at most two odd-degree nodes, and if there are
//     two, this node is one of them.
//
// Failing either proves the branch is dead, so pruning it cannot lose a
// solution.
func (s *State) feasible(m *Map) bool {
	if !s.reachableAll(m) {
		return false
	}
	return s.parityOK(m)
}

// parityOK applies the Euler parity/balance test to the edges still owed.
//
// With only undirected edges left, this is the classic degree argument: a walk
// covering every remaining edge from s.Pos leaves zero or two odd-degree nodes,
// and if two, s.Pos is one of them.
//
// With only one-way edges left, the degree argument does not hold, so the test
// uses the directed balance condition instead (see balanceOK).
//
// When both kinds remain there is no sound static test (an undirected edge can
// absorb any imbalance), so nothing is pruned.
//
// Either way, failing the test proves the branch is dead, so pruning it cannot
// lose a solution.
func (s *State) parityOK(m *Map) bool {
	haveDirected, haveUndirected := false, false
	for ei, left := range s.used {
		if left == 0 {
			continue
		}
		if m.Edges[ei].Directed() {
			haveDirected = true
		} else {
			haveUndirected = true
		}
	}
	switch {
	case haveDirected && haveUndirected:
		return true
	case haveDirected:
		return s.balanceOK(m)
	}

	deg := make([]int, len(m.Nodes))
	for ei, left := range s.used {
		if left == 0 {
			continue
		}
		e := m.Edges[ei]
		deg[m.nodeIdx[e.A]] += left
		deg[m.nodeIdx[e.B]] += left
	}
	odd := 0
	posOdd := false
	for i, d := range deg {
		if d%2 == 1 {
			odd++
			if m.Nodes[i].ID == s.Pos {
				posOdd = true
			}
		}
	}
	switch {
	case odd == 0:
		return true
	case odd == 2:
		return posOdd
	default:
		return false
	}
}

// balanceOK is the directed Euler-trail balance test: a single trail from s.Pos
// over only one-way edges needs out-in of 0 at s.Pos (a closed tour, so no node
// short by one) or exactly 1 (an open trail ending at the single node short by
// one), and 0 everywhere else with no more than one node short.
func (s *State) balanceOK(m *Map) bool {
	bal := make([]int, len(m.Nodes))
	for ei, left := range s.used {
		if left == 0 {
			continue
		}
		e := m.Edges[ei]
		bal[m.nodeIdx[e.Dir.From]] += left
		bal[m.nodeIdx[e.Dir.To]] -= left
	}
	posIdx := m.nodeIdx[s.Pos]
	if bal[posIdx] < 0 || bal[posIdx] > 1 {
		return false
	}
	negOnes := 0
	for i, b := range bal {
		if i == posIdx {
			continue
		}
		switch {
		case b == -1:
			negOnes++
		case b == 0:
		default:
			return false
		}
	}
	if negOnes > 1 {
		return false
	}
	if bal[posIdx] == 1 && negOnes != 1 {
		return false
	}
	return true
}

// reachableAll reports whether every edge still owed can be reached from the
// current node, following only edges a walk could still leave by.
func (s *State) reachableAll(m *Map) bool {
	seen := make([]bool, len(m.Nodes))
	start := m.nodeIdx[s.Pos]
	seen[start] = true
	queue := []int{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		curID := m.Nodes[cur].ID
		for _, ei := range m.adj[cur] {
			if s.used[ei] == 0 {
				continue
			}
			e := m.Edges[ei]
			// A one-way edge entered from its target cannot be left again.
			if e.Directed() && e.Dir.To == curID {
				continue
			}
			other := e.A
			if other == curID {
				other = e.B
			}
			oi, ok := m.nodeIdx[other]
			if !ok || seen[oi] {
				continue
			}
			seen[oi] = true
			queue = append(queue, oi)
		}
	}
	for ei, left := range s.used {
		if left == 0 {
			continue
		}
		e := m.Edges[ei]
		if e.Directed() {
			if !seen[m.nodeIdx[e.Dir.From]] {
				return false
			}
			continue
		}
		if !seen[m.nodeIdx[e.A]] && !seen[m.nodeIdx[e.B]] {
			return false
		}
	}
	return true
}

// Hint returns the next node to travel to on a completion, and whether one was
// found.
func Hint(m *Map, s State) (int, bool) {
	plan, err := Plan(m, s)
	if err != nil || !plan.Found || len(plan.Steps) == 0 {
		return -1, false
	}
	return plan.Steps[0], true
}

// StepsFromPath converts a full node trail into the moves a caller would send,
// i.e. the trail without its first (starting) node.
func StepsFromPath(path []int) []int {
	if len(path) < 2 {
		return []int{}
	}
	return append([]int(nil), path[1:]...)
}

// ValidSolution reports whether path covers every edge of m exactly Count
// times, respecting directions. Map validation uses it to check golden
// solutions.
func ValidSolution(m *Map, path []int) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}
	s := NewState(m)
	ev, err := s.Start(m, path[0])
	if err != nil {
		return err
	}
	if ev.Blocked {
		return fmt.Errorf("cannot start at node %d: %s", path[0], ev.Reason)
	}
	for i := 1; i < len(path); i++ {
		ev, err := s.Travel(m, path[i])
		if err != nil {
			return err
		}
		if ev.Blocked {
			return fmt.Errorf("step %d: %s", i, ev.Reason)
		}
	}
	if s.Status != StatusWon {
		return fmt.Errorf("path leaves %d traversals undrawn", s.Remaining)
	}
	return nil
}
