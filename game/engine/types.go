// Package engine holds the pure One T Draw rules: the graph a map describes,
// the walk state, and the feasibility logic that powers hints and auto-solve.
//
// A map is a graph. The player draws one continuous trail that traverses every
// edge exactly as many times as its Count says. Stepping onto an already
// exhausted edge, or onto a node that is not a neighbour, is a blocked attempt
// and costs nothing. Running out of usable edges while edges remain is a dead
// end; covering the last edge wins.
package engine

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Vec is a board position in layout units. The board box is 0..LayoutSize on
// both axes so the UI can scale it to any viewport.
type Vec struct{ X, Y int }

// LayoutSize is the width and height of the layout coordinate space.
const LayoutSize = 1000

// Direction constrains which way an edge may be traversed. An edge without one
// is undirected.
type Direction struct {
	From           int  `json:"From"`
	To             int  `json:"To"`
	Unidirectional bool `json:"Unidirectional"`
}

// flexBool accepts true/false or 1/0. The bundled puzzle files use both
// spellings for the Hide flag, which marks layout-only spacer nodes.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(string(data), `"`) {
	case "true", "1":
		*b = true
	case "false", "0", "", "null":
		*b = false
	default:
		return fmt.Errorf("invalid boolean value %s", string(data))
	}
	return nil
}

func (b flexBool) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("true"), nil
	}
	return []byte("false"), nil
}

// PointConfig is one node of a map file. Level only affects layout: nodes of
// the same Level are drawn on one band.
type PointConfig struct {
	Point int      `json:"Point"`
	Level int      `json:"Level,omitempty"`
	Hide  flexBool `json:"Hide,omitempty"`
}

// Hidden reports whether the point is a layout-only spacer.
func (p PointConfig) Hidden() bool { return bool(p.Hide) }

// EdgeConfig is one edge of a map file. Count defaults to 1.
type EdgeConfig struct {
	PointA    int        `json:"PointA"`
	PointB    int        `json:"PointB"`
	Count     int        `json:"Count,omitempty"`
	Direction *Direction `json:"Direction,omitempty"`
}

// MapConfig is the JSON representation of a map (maps/*.json).
//
// The field names match the original puzzle files, so every bundled puzzle is
// a valid map without conversion.
type MapConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Difficulty  string        `json:"difficulty,omitempty"`
	Points      []PointConfig `json:"Points,omitempty"`
	Edges       []EdgeConfig  `json:"Edges"`
	MaxMoves    int           `json:"max_moves,omitempty"`
}

// Edge is a parsed edge. Count is the number of times it must be traversed.
type Edge struct {
	A, B  int
	Count int
	Dir   *Direction
}

// Directed reports whether the edge can only be traversed from A to B.
func (e Edge) Directed() bool { return e.Dir != nil && e.Dir.Unidirectional }

// Allows reports whether a traversal from -> to is legal for this edge,
// ignoring whether it has traversals left.
//
// From and to must be the edge's two distinct endpoints, in either order; a
// step from a node to itself is never a traversal. (Without the from != to
// check, an edge incident to a node would accept a "step" from that node to
// itself, which would let a tampered path replay.)
func (e Edge) Allows(from, to int) bool {
	if from == to {
		return false
	}
	if !((e.A == from && e.B == to) || (e.A == to && e.B == from)) {
		return false
	}
	if e.Directed() {
		// A one-way edge only ever goes Dir.From -> Dir.To.
		return e.Dir.From == from && e.Dir.To == to
	}
	return true
}

// Node is a parsed node with its layout position.
type Node struct {
	ID    int
	Level int
	Pos   Vec
}

// Verdict is the pre-flight Euler classification of a map.
type Verdict string

const (
	// VerdictCircuit: a closed tour exists, so any node can start.
	VerdictCircuit Verdict = "CIRCUIT"
	// VerdictTrail: an open trail exists; it must start at an odd node.
	VerdictTrail Verdict = "TRAIL"
	// VerdictImpossible: no single trail can cover every edge.
	VerdictImpossible Verdict = "IMPOSSIBLE"
	// VerdictDirected: directed edges are present, so the undirected parity
	// argument does not apply; solvability is decided by search.
	VerdictDirected Verdict = "DIRECTED"
)

// Analysis is a cheap pre-flight over a map: whether a full trail can exist at
// all and where it may start.
type Analysis struct {
	NodeCount   int
	EdgeCount   int
	StepCount   int // total traversals required
	Connected   bool
	Directional bool
	OddNodes    []int // nodes with odd remaining degree, ascending
	StartNodes  []int // nodes a full trail may begin at, ascending
	Verdict     Verdict
}

// Message is the one-line human description of the verdict.
func (a Analysis) Message() string {
	switch a.Verdict {
	case VerdictCircuit:
		return fmt.Sprintf("closed tour: every node works as a start (%d traversals)", a.StepCount)
	case VerdictTrail:
		return fmt.Sprintf("open trail: must start at %s (%d traversals)", joinInts(a.OddNodes), a.StepCount)
	case VerdictDirected:
		return fmt.Sprintf("directed edges present (%d traversals); the start point depends on direction", a.StepCount)
	}
	if !a.Connected {
		return "impossible: the edge groups are not connected"
	}
	return fmt.Sprintf("impossible: %d odd-degree nodes (%s)", len(a.OddNodes), joinInts(a.OddNodes))
}

func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}

// parSolveBudget bounds the one-off existence search done at load time, so a
// pathological map file cannot stall startup. The bundled maps solve in well
// under a second; the hardest needs about two seconds under the race detector.
const parSolveBudget = 20 * time.Second

// Limits and defaults.
const (
	MinCount     = 1
	MaxEdgeCount = 100 // traversals one edge may require
	MaxEdges     = 2000
	MaxNodes     = 2000
	MaxMaxMoves  = 100000
)

var idRe = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// ValidID reports whether id is a legal map id (also a safe file name).
func ValidID(id string) bool { return idRe.MatchString(id) }

// Map is a parsed, validated map.
type Map struct {
	ID          string
	Name        string
	Description string
	Difficulty  string
	MaxMoves    int

	points []PointConfig // as loaded, including hidden spacers

	Nodes   []Node
	Edges   []Edge
	LevelOf map[int]int

	nodeIdx map[int]int // node id -> index in Nodes
	adj     [][]int     // node index -> edge indices
	indeg   []int
	outdeg  []int

	StepCount int
	Width     int
	Height    int

	Analysis Analysis
	// Par is the fewest moves a full trail needs, which is StepCount when the
	// map is solvable and -1 when it is not.
	Par int
}

// ValidationError aggregates structural problems of a map.
type ValidationError struct{ Issues []string }

func (e *ValidationError) Error() string { return strings.Join(e.Issues, "; ") }

// NewMap validates cfg (applying defaults) and builds a Map.
func NewMap(cfg MapConfig) (*Map, error) {
	var issues []string
	add := func(format string, args ...any) { issues = append(issues, fmt.Sprintf(format, args...)) }

	if !ValidID(cfg.ID) {
		add("id %q must match %s", cfg.ID, idRe.String())
	}
	if strings.TrimSpace(cfg.Name) == "" {
		add("name is required")
	}
	if len(cfg.Edges) == 0 {
		add("a map needs at least one edge")
	}
	if len(cfg.Edges) > MaxEdges {
		add("too many edges (%d, max %d)", len(cfg.Edges), MaxEdges)
	}

	m := &Map{
		ID: cfg.ID, Name: cfg.Name, Description: cfg.Description, Difficulty: cfg.Difficulty,
		MaxMoves: cfg.MaxMoves, Par: -1,
		points:  append([]PointConfig(nil), cfg.Points...),
		LevelOf: map[int]int{},
		nodeIdx: map[int]int{},
	}
	if m.Difficulty == "" {
		m.Difficulty = "easy"
	}

	// Collect the node universe: everything mentioned by a point or an edge.
	levels := map[int]int{}
	for _, p := range cfg.Points {
		levels[p.Point] = p.Level
	}
	seen := map[int]bool{}
	for _, e := range cfg.Edges {
		seen[e.PointA] = true
		seen[e.PointB] = true
	}
	for id := range seen {
		if _, ok := levels[id]; !ok {
			levels[id] = 0
		}
	}
	hidden := map[int]bool{}
	for _, p := range cfg.Points {
		if p.Hidden() {
			hidden[p.Point] = true
		}
	}
	for id := range seen {
		if hidden[id] {
			add("node %d is marked hidden but has edges", id)
		}
	}

	ids := make([]int, 0, len(levels))
	for id := range levels {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) > MaxNodes {
		return nil, &ValidationError{Issues: append(issues, fmt.Sprintf("too many nodes (%d, max %d)", len(ids), MaxNodes))}
	}
	m.Nodes = make([]Node, 0, len(levels))
	for _, id := range ids {
		m.nodeIdx[id] = len(m.Nodes)
		m.LevelOf[id] = levels[id]
		m.Nodes = append(m.Nodes, Node{ID: id, Level: levels[id]})
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}

	m.adj = make([][]int, len(m.Nodes))
	m.indeg = make([]int, len(m.Nodes))
	m.outdeg = make([]int, len(m.Nodes))
	for i, e := range cfg.Edges {
		if _, ok := m.nodeIdx[e.PointA]; !ok {
			add("edge %d: unknown point %d", i, e.PointA)
			continue
		}
		if _, ok := m.nodeIdx[e.PointB]; !ok {
			add("edge %d: unknown point %d", i, e.PointB)
			continue
		}
		if e.PointA == e.PointB {
			add("edge %d joins point %d to itself", i, e.PointA)
			continue
		}
		count := e.Count
		if count == 0 {
			count = 1
		}
		if count < 0 || count > MaxEdgeCount {
			add("edge %d: count must be %d..%d, got %d", i, MinCount, MaxEdgeCount, count)
			continue
		}
		edge := Edge{A: e.PointA, B: e.PointB, Count: count, Dir: e.Direction}
		if edge.Directed() {
			if !(edge.Dir.From == edge.A && edge.Dir.To == edge.B) && !(edge.Dir.From == edge.B && edge.Dir.To == edge.A) {
				add("edge %d: direction %d->%d does not match its endpoints %d-%d", i, edge.Dir.From, edge.Dir.To, edge.A, edge.B)
				continue
			}
		}
		idx := len(m.Edges)
		m.Edges = append(m.Edges, edge)
		m.adj[m.nodeIdx[e.PointA]] = append(m.adj[m.nodeIdx[e.PointA]], idx)
		m.adj[m.nodeIdx[e.PointB]] = append(m.adj[m.nodeIdx[e.PointB]], idx)
		m.StepCount += count
		if edge.Directed() {
			m.outdeg[m.nodeIdx[edge.Dir.From]] += count
			m.indeg[m.nodeIdx[edge.Dir.To]] += count
		} else {
			m.outdeg[m.nodeIdx[e.PointA]] += count
			m.indeg[m.nodeIdx[e.PointA]] += count
			m.outdeg[m.nodeIdx[e.PointB]] += count
			m.indeg[m.nodeIdx[e.PointB]] += count
		}
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}

	if m.MaxMoves == 0 {
		m.MaxMoves = m.StepCount
	}
	if m.MaxMoves < 1 || m.MaxMoves > MaxMaxMoves {
		add("max_moves must be 1..%d, got %d", MaxMaxMoves, m.MaxMoves)
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Issues: issues}
	}

	m.computeLayout()
	m.Analysis = analyze(m)
	// Par is the length of a complete trail. Every complete trail draws the
	// same edges, so it is StepCount whenever one exists; the search only has
	// to decide existence. It is skipped when the parity test already proves
	// the map unsolvable, and bounded so loading a pathological map cannot hang
	// startup. m.Par stays -1 when it is unknown or unreachable.
	if m.Analysis.Verdict != VerdictImpossible {
		if plan, err := planWithin(m, NewState(m), parSolveBudget); err == nil && plan.Found {
			m.Par = m.StepCount
		}
	}
	return m, nil
}

// Config returns the JSON form of the map, for round-tripping through the
// editor.
func (m *Map) Config() MapConfig {
	edges := make([]EdgeConfig, len(m.Edges))
	for i, e := range m.Edges {
		edges[i] = EdgeConfig{PointA: e.A, PointB: e.B, Count: e.Count}
		if e.Dir != nil {
			d := *e.Dir
			edges[i].Direction = &d
		}
	}
	return MapConfig{
		ID: m.ID, Name: m.Name, Description: m.Description, Difficulty: m.Difficulty,
		Points: append([]PointConfig(nil), m.points...), Edges: edges, MaxMoves: m.MaxMoves,
	}
}

// Clone deep-copies the map. It never re-runs the solver: Par and Analysis are
// carried over as they are, because a map's graph is immutable once parsed.
func (m *Map) Clone() *Map {
	c := *m
	c.Nodes = append([]Node(nil), m.Nodes...)
	c.Edges = append([]Edge(nil), m.Edges...)
	c.points = append([]PointConfig(nil), m.points...)
	c.LevelOf = make(map[int]int, len(m.LevelOf))
	for k, v := range m.LevelOf {
		c.LevelOf[k] = v
	}
	c.nodeIdx = make(map[int]int, len(m.nodeIdx))
	for k, v := range m.nodeIdx {
		c.nodeIdx[k] = v
	}
	c.adj = make([][]int, len(m.adj))
	for i := range m.adj {
		c.adj[i] = append([]int(nil), m.adj[i]...)
	}
	c.indeg = append([]int(nil), m.indeg...)
	c.outdeg = append([]int(nil), m.outdeg...)
	c.Analysis.OddNodes = append([]int(nil), m.Analysis.OddNodes...)
	c.Analysis.StartNodes = append([]int(nil), m.Analysis.StartNodes...)
	return &c
}

// LevelCount is the number of distinct layout levels.
func (m *Map) LevelCount() int {
	seen := map[int]bool{}
	for _, n := range m.Nodes {
		seen[n.Level] = true
	}
	return len(seen)
}

// NodeExists reports whether id is a node of the map.
func (m *Map) NodeExists(id int) bool {
	_, ok := m.nodeIdx[id]
	return ok
}

// Node returns the node with the given id.
func (m *Map) Node(id int) (Node, bool) {
	i, ok := m.nodeIdx[id]
	if !ok {
		return Node{}, false
	}
	return m.Nodes[i], true
}

// incidentEdges returns the edge indices touching node id.
func (m *Map) incidentEdges(id int) []int {
	i, ok := m.nodeIdx[id]
	if !ok {
		return nil
	}
	return m.adj[i]
}

// computeLayout assigns every node a position in the normalized layout box.
//
// The map file's Level values are a hint, not a geometry: several nodes share a
// level, and some maps carry no levels at all. Placing each level on one row
// draws those nodes in a straight line, which for a three-node triangle
// collapses the figure into a single segment. So the levels seed a force
// relaxation: springs pull joined nodes together, repulsion pushes every pair
// apart, and a weak attraction to the seed row keeps the overall bands. The
// result is deterministic (no randomness, fixed iteration count), so a map looks
// the same on every load and in every client.
func (m *Map) computeLayout() {
	byLevel := map[int][]int{}
	for _, n := range m.Nodes {
		byLevel[n.Level] = append(byLevel[n.Level], n.ID)
	}
	levels := make([]int, 0, len(byLevel))
	for l := range byLevel {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		sort.Ints(byLevel[l])
	}

	const margin = 90
	span := LayoutSize - 2*margin
	bands := len(levels)
	m.Width = LayoutSize
	m.Height = LayoutSize
	n := len(m.Nodes)
	if n == 0 {
		return
	}

	// Seed: rows per level. A single band (or no levels at all, which the
	// directional triangle has) is seeded on a circle instead, so every node
	// starts with a distinct direction from the centre.
	seedX := make([]float64, n)
	seedY := make([]float64, n)
	circle := bands <= 1
	for bi, l := range levels {
		ids := byLevel[l]
		y := float64(LayoutSize) / 2
		if bands > 1 {
			y = float64(margin + bi*span/(bands-1))
		}
		for i, id := range ids {
			ni := m.nodeIdx[id]
			seedY[ni] = y
			if circle {
				// Radius shrinks as the node count grows so a big single-level map
				// still fits the box.
				r := float64(span) / 2 * 0.85
				angle := 2 * math.Pi * float64(i) / float64(len(ids))
				seedX[ni] = float64(LayoutSize)/2 + r*math.Cos(angle)
				seedY[ni] = float64(LayoutSize)/2 + r*math.Sin(angle)
				continue
			}
			x := float64(LayoutSize) / 2
			if len(ids) > 1 {
				x = float64(margin + i*span/(len(ids)-1))
			}
			seedX[ni] = x
		}
	}

	relaxLayout(m, seedX, seedY, circle)
}

// relaxLayout runs a fixed number of deterministic spring/repulsion iterations
// from the seed positions, then writes the result into the nodes.
func relaxLayout(m *Map, seedX, seedY []float64, circle bool) {
	const (
		margin     = 90
		iterations = 320
		idealLen   = 300.0
		repulsion  = 900000.0
		springK    = 1.4
		rowPull    = 0.02
		cooling    = 0.985
	)
	n := len(m.Nodes)
	x := append([]float64(nil), seedX...)
	y := append([]float64(nil), seedY...)

	// Degenerate layouts (several nodes in exactly the same spot) would never
	// separate under pure forces, because the repulsion direction is undefined.
	// A tiny deterministic nudge keyed by index breaks the tie.
	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			if math.Abs(x[i]-x[j]) < 1 && math.Abs(y[i]-y[j]) < 1 {
				x[i] += 3 * float64(i+1)
				y[i] += 2 * float64(i+1)
			}
		}
	}

	dx := make([]float64, n)
	dy := make([]float64, n)
	step := 1.0
	for it := 0; it < iterations; it++ {
		for i := range dx {
			dx[i], dy[i] = 0, 0
		}

		// Repulsion: every pair pushes every pair apart.
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				vx := x[i] - x[j]
				vy := y[i] - y[j]
				d2 := vx*vx + vy*vy
				if d2 < 1 {
					d2 = 1
				}
				d := math.Sqrt(d2)
				f := repulsion / d2
				ux, uy := vx/d, vy/d
				dx[i] += f * ux
				dy[i] += f * uy
				dx[j] -= f * ux
				dy[j] -= f * uy
			}
		}

		// Springs: joined nodes pull together towards idealLen.
		for _, e := range m.Edges {
			i, j := m.nodeIdx[e.A], m.nodeIdx[e.B]
			vx := x[j] - x[i]
			vy := y[j] - y[i]
			d := math.Hypot(vx, vy)
			if d < 1 {
				d = 1
			}
			// A doubled edge pulls twice as hard, so a pair joined twice sits closer.
			f := springK * float64(e.Count) * (d - idealLen) / idealLen
			ux, uy := vx/d, vy/d
			dx[i] += f * ux * 40
			dy[i] += f * uy * 40
			dx[j] -= f * ux * 40
			dy[j] -= f * uy * 40
		}

		// Weak row pull keeps the file's levels legible as bands.
		if !circle {
			for i := 0; i < n; i++ {
				dy[i] += (seedY[i] - y[i]) * rowPull * 40
			}
		}

		// Apply, then clamp into the padded box.
		for i := 0; i < n; i++ {
			mag := math.Hypot(dx[i], dy[i])
			if mag > 40 {
				dx[i] *= 40 / mag
				dy[i] *= 40 / mag
			}
			x[i] += dx[i] * step * 0.05
			y[i] += dy[i] * step * 0.05
			x[i] = math.Max(margin, math.Min(LayoutSize-margin, x[i]))
			y[i] = math.Max(margin, math.Min(LayoutSize-margin, y[i]))
		}
		step *= cooling
	}

	for i := range m.Nodes {
		m.Nodes[i].Pos = Vec{X: int(math.Round(x[i])), Y: int(math.Round(y[i]))}
	}
}

// analyze computes the pre-flight Euler classification of a map.
func analyze(m *Map) Analysis {
	a := Analysis{
		NodeCount:   len(m.Nodes),
		EdgeCount:   len(m.Edges),
		StepCount:   m.StepCount,
		Directional: false,
	}
	deg := make([]int, len(m.Nodes))
	for _, e := range m.Edges {
		if e.Directed() {
			a.Directional = true
		}
		deg[m.nodeIdx[e.A]] += e.Count
		deg[m.nodeIdx[e.B]] += e.Count
	}
	a.Connected = m.edgesConnected(func(int) bool { return true })

	for i, d := range deg {
		if d%2 == 1 {
			a.OddNodes = append(a.OddNodes, m.Nodes[i].ID)
		}
	}
	sort.Ints(a.OddNodes)

	switch {
	case !a.Connected || m.StepCount == 0:
		a.Verdict = VerdictImpossible
	case a.Directional:
		a.Verdict = VerdictDirected
	case len(a.OddNodes) == 0:
		a.Verdict = VerdictCircuit
	case len(a.OddNodes) == 2:
		a.Verdict = VerdictTrail
	default:
		a.Verdict = VerdictImpossible
	}

	switch a.Verdict {
	case VerdictTrail:
		a.StartNodes = append([]int(nil), a.OddNodes...)
	case VerdictImpossible:
		a.StartNodes = []int{}
	default:
		for i, d := range deg {
			if d > 0 {
				a.StartNodes = append(a.StartNodes, m.Nodes[i].ID)
			}
		}
	}
	if a.StartNodes == nil {
		a.StartNodes = []int{}
	}
	if a.OddNodes == nil {
		a.OddNodes = []int{}
	}
	return a
}

// edgesConnected reports whether every edge whose include returns true lies in
// one connected component (viewed as an undirected graph).
func (m *Map) edgesConnected(include func(edgeIndex int) bool) bool {
	parent := make([]int, len(m.Nodes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	active := false
	for i, e := range m.Edges {
		if !include(i) {
			continue
		}
		active = true
		union(m.nodeIdx[e.A], m.nodeIdx[e.B])
	}
	if !active {
		return true
	}
	root := -1
	for i, e := range m.Edges {
		if !include(i) {
			continue
		}
		r := find(m.nodeIdx[e.A])
		if root == -1 {
			root = r
		} else if r != root {
			return false
		}
	}
	return true
}

// NewPointConfig builds a PointConfig, so callers outside the package (the
// GraphQL conversion) do not need to set the unexported flexBool.
func NewPointConfig(point, level int, hidden bool) PointConfig {
	return PointConfig{Point: point, Level: level, Hide: flexBool(hidden)}
}

// Now reports the current time. It exists so the API layer has one seam for
// timestamps without importing the time package at every call site.
func Now() time.Time { return time.Now() }
