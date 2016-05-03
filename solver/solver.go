package solver

import (
	"encoding/json"
	"sync"
)

var waiting_group sync.WaitGroup

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
	var sp []uint16
	for _, edge := range this.Edges {
		sp = append(sp, edge.PointA)
		sp = append(sp, edge.PointB)
	}
	sp = removeDuplicates(sp)
	return sp
}

func (this *Puzzle) getEdge(e1 *uint16, e2 *uint16) *Edge {
	for k, edge := range this.Edges {
		if edge.PointA == *e1 && edge.PointB == *e2 {
			return &this.Edges[k]
		} else if edge.PointA == *e2 && edge.PointB == *e1 {
			return &this.Edges[k]
		}
	}
	return &this.Edges[0]
}

func (this *Puzzle) visitEdge(edge *Edge) {
	this.count = this.count - 1
	edge.visit()
}

func (this *Puzzle) listPossibleEdgesToVisit(from *uint16) []uint16 {
	var r []uint16
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

func findSolutions(puzzle *Puzzle, starting *uint16, path []uint16, solution_handler SolutionHandler, level uint16) {
	path = append(path, *starting)
	possible_edges := puzzle.listPossibleEdgesToVisit(starting)

	if len(possible_edges) == 0 {
		if puzzle.isSolved() {
			solution_handler.handleNewSolutionFound(&path)
		}
	} else {
		for k, _ := range possible_edges {
			previous_count := puzzle.count
			edge := puzzle.getEdge(starting, &possible_edges[k])
			previous_edgecount := edge.Count
			puzzle.visitEdge(edge)

			findSolutions(puzzle, &possible_edges[k], path, solution_handler, level+1)
			edge.Count = previous_edgecount
			puzzle.count = previous_count
		}

		if level == 1 {
			waiting_group.Done()
		}
	}
}

func Solve(puzzle *Puzzle) *Solutions {
	starting_points := puzzle.listStartingPoints()
	arr_solution_storer := make([]solutionStorer, len(starting_points))

	for k, _ := range starting_points {
		arr_solution_storer[k] = *newSolutionStorer()
		waiting_group.Add(1)
		path := make([]uint16, 0)
		pc := puzzle.Copy()
		go findSolutions(&pc, &starting_points[k], path, &arr_solution_storer[k], 1)
	}

	waiting_group.Wait()
	to_return := make(Solutions, 0)
	for _, solutions := range arr_solution_storer {
		for _, solution := range solutions.solutions {
			to_return = append(to_return, *solution)
		}
	}

	return &to_return
}

type Solutions []Solution

func (this *Solutions) Print(printer SolutionPrinter) error {
	printer.Print(this)
	return nil
}

func GetNumberOfSolutions(puzzle *Puzzle) int {
	starting_points := puzzle.listStartingPoints()
	arr_solutions_count := make([]solutionCounter, len(starting_points))

	for k, _ := range starting_points {
		arr_solutions_count[k] = *newSolutionCounter()
		waiting_group.Add(1)
		path := make([]uint16, 0)
		pc := puzzle.Copy()
		go findSolutions(&pc, &starting_points[k], path, &arr_solutions_count[k], 1)
	}

	waiting_group.Wait()
	to_return := 0
	for _, solutions := range arr_solutions_count {
		to_return = to_return + solutions.count_solutions
	}

	return to_return
}

func removeDuplicates(s []uint16) []uint16 {
	m := map[uint16]bool{}
	for _, v := range s {
		if _, seen := m[v]; !seen {
			s[len(m)] = v
			m[v] = true
		}
	}
	s = s[:len(m)]
	return s
}
