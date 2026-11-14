package solver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Degree is one node's total required edge traversals, counting each edge
// once per unit of Count for both of its endpoints.
type Degree struct {
	Node   uint16
	Degree int
}

// Analysis is a cheap pre-flight over a puzzle: whether a traversal can exist
// at all, where it must start, and how big the search space is likely to be.
type Analysis struct {
	NodeCount   int
	EdgeCount   int
	StepCount   int // total single edge traversals required
	Degrees     []Degree
	OddVertices []uint16 // nodes with an odd degree, ascending
	Connected   bool
	Directional bool
	Verdict     string // "circuit", "trail", "impossible" or "directed"
}

// Analyze inspects a puzzle without searching for solutions.
//
// The parity test is the classic Euler criterion: a single trail covering every
// edge exactly Count times exists only if the graph is connected and has zero
// odd-degree vertices (a closed tour) or exactly two (an open trail starting at
// one of them). Unidirectional edges break the undirected argument, so those
// puzzles report "directed" and carry no verdict.
func Analyze(puzzle *Puzzle) Analysis {
	a := Analysis{EdgeCount: len(puzzle.Edges)}

	deg := make(map[uint16]int, len(puzzle.Edges)*2)
	parent := make(map[uint16]uint16, len(puzzle.Edges)*2)
	var find func(uint16) uint16
	find = func(x uint16) uint16 {
		p, ok := parent[x]
		if !ok || p == x {
			parent[x] = x
			return x
		}
		root := find(p)
		parent[x] = root
		return root
	}
	union := func(x, y uint16) {
		rx, ry := find(x), find(y)
		if rx != ry {
			parent[rx] = ry
		}
	}

	for _, e := range puzzle.Edges {
		n := int(e.Count)
		deg[e.PointA] += n
		deg[e.PointB] += n
		a.StepCount += n
		if e.Direction.Unidirectional {
			a.Directional = true
		}
		if n > 0 {
			union(e.PointA, e.PointB)
		}
	}
	a.NodeCount = len(deg)

	for node, d := range deg {
		a.Degrees = append(a.Degrees, Degree{Node: node, Degree: d})
		if d%2 == 1 {
			a.OddVertices = append(a.OddVertices, node)
		}
	}
	sort.Slice(a.Degrees, func(i, j int) bool { return a.Degrees[i].Node < a.Degrees[j].Node })
	sort.Slice(a.OddVertices, func(i, j int) bool { return a.OddVertices[i] < a.OddVertices[j] })

	a.Connected = true
	rootSet := make(map[uint16]struct{}, len(deg))
	for node, d := range deg {
		if d == 0 {
			continue
		}
		rootSet[find(node)] = struct{}{}
	}
	if len(rootSet) > 1 {
		a.Connected = false
	}

	switch {
	case !a.Connected || a.StepCount == 0:
		a.Verdict = "impossible"
	case a.Directional:
		a.Verdict = "directed"
	case len(a.OddVertices) == 0:
		a.Verdict = "circuit"
	case len(a.OddVertices) == 2:
		a.Verdict = "trail"
	default:
		a.Verdict = "impossible"
	}
	return a
}

// StartNodes returns the nodes a solution can begin at according to Analysis.
// For a closed tour every node works, so all of them are returned.
func (this Analysis) StartNodes() []uint16 {
	if this.Verdict == "trail" {
		return append([]uint16(nil), this.OddVertices...)
	}
	nodes := make([]uint16, 0, len(this.Degrees))
	for _, d := range this.Degrees {
		if d.Degree > 0 {
			nodes = append(nodes, d.Node)
		}
	}
	return nodes
}

// String is a short human sentence describing the verdict, used by the CLI.
func (this Analysis) String() string {
	switch this.Verdict {
	case "circuit":
		return fmt.Sprintf("closed tour: every start point works (%d edge traversals)", this.StepCount)
	case "trail":
		return fmt.Sprintf("open trail: must start at %s (%d edge traversals)", joinNodes(this.OddVertices), this.StepCount)
	case "directed":
		return fmt.Sprintf("directed edges present (%d edge traversals); start point depends on direction", this.StepCount)
	case "impossible":
		if !this.Connected {
			return "impossible: the puzzle has disconnected edges"
		}
		return fmt.Sprintf("impossible: %d odd-degree points (%s)", len(this.OddVertices), joinNodes(this.OddVertices))
	}
	return ""
}

func joinNodes(nodes []uint16) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = strconv.Itoa(int(n))
	}
	return strings.Join(parts, ", ")
}

// CanonicalKey collapses solutions that draw the same picture. Two walks are
// the same drawing when one is the other traversed backwards, and for a closed
// walk any rotation of it is the same drawing too.
func CanonicalKey(s Solution) string {
	best := keyOf(s)
	if k := keyOf(reversed(s)); k < best {
		best = k
	}
	if len(s) > 1 && s[0] == s[len(s)-1] {
		best = minClosedKey(s, best)
		best = minClosedKey(reversed(s), best)
	}
	return best
}

func minClosedKey(s Solution, best string) string {
	ring := s[:len(s)-1] // drop the repeated final node
	n := len(ring)
	buf := make([]uint16, 0, n)
	for shift := 1; shift < n; shift++ {
		buf = buf[:0]
		buf = append(buf, ring[shift:]...)
		buf = append(buf, ring[:shift]...)
		buf = append(buf, buf[0])
		if k := keyOf(buf); k < best {
			best = k
		}
	}
	return best
}

func reversed(s Solution) Solution {
	out := make(Solution, len(s))
	for i := range s {
		out[i] = s[len(s)-1-i]
	}
	return out
}

func keyOf(s Solution) string {
	var b strings.Builder
	b.Grow(len(s) * 4)
	for i, node := range s {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(int(node)))
	}
	return b.String()
}

// UniqueSolutions keeps the first walk that draws each distinct picture.
func UniqueSolutions(solutions *Solutions) *Solutions {
	indices := UniqueIndices(solutions)
	out := make(Solutions, 0, len(indices))
	for _, i := range indices {
		out = append(out, (*solutions)[i])
	}
	return &out
}

// UniqueIndices reports the positions of the first walk for each distinct
// drawing, which lets a caller send the walks once and refer to them by index.
func UniqueIndices(solutions *Solutions) []int {
	seen := make(map[string]struct{}, len(*solutions))
	out := make([]int, 0, len(*solutions))
	for i, s := range *solutions {
		k := CanonicalKey(s)
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, i)
	}
	return out
}
