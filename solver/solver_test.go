package solver

import (
	"encoding/json"
	. "launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestRemoveDuplicates(c *C) {
	original := []uint16{1, 1, 1, 1}
	expected := []uint16{1}
	result := removeDuplicates(original)
	c.Check(result, DeepEquals, expected)

	original = []uint16{1, 1, 1, 2, 1}
	expected = []uint16{1, 2}
	result = removeDuplicates(original)
	c.Check(result, DeepEquals, expected)

	original = []uint16{1, 2, 3}
	expected = []uint16{1, 2, 3}
	result = removeDuplicates(original)
	c.Check(result, DeepEquals, expected)
}

func (s *MySuite) TestVisitEdge(c *C) {
	var expected uint16

	e := Edge{PointA: 1, PointB: 2, Count: 1}

	expected = 1
	c.Check(e.Count, Equals, expected)

	e.visit()

	expected = 0
	c.Check(e.Count, Equals, expected)
}

func (s *MySuite) TestSolveRegularTriange(c *C) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)

	solutions := Solve(p)
	c.Check(len(*solutions), Equals, 6)

	tmp, _ := json.Marshal(*solutions)
	encoded := string(tmp)

	c.Check(encoded, Equals, "[[1,2,3,1],[1,3,2,1],[2,1,3,2],[2,3,1,2],[3,2,1,3],[3,1,2,3]]")
}

func (s *MySuite) TestGetCountSolutionsRegularTriange(c *C) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)

	solutions := GetNumberOfSolutions(p)
	c.Check(solutions, Equals, 6)
}

func (s *MySuite) TestSolveDirectional(c *C) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1, Direction: Direction{From: 1, To: 2, Unidirectional: true}}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)

	solutions := Solve(p)
	c.Check(len(*solutions), Equals, 3)

	tmp, _ := json.Marshal(*solutions)
	encoded := string(tmp)

	c.Check(encoded, Equals, "[[1,2,3,1],[2,3,1,2],[3,1,2,3]]")
}

func (s *MySuite) BenchmarkLogic(c *C) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	for i := 0; i < c.N; i++ {
		p := NewPuzzle(edges)
		_ = Solve(p)
	}
}
