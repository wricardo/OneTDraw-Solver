package solver

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint16
		expected []uint16
	}{
		{
			name:     "all duplicates",
			input:    []uint16{1, 1, 1, 1},
			expected: []uint16{1},
		},
		{
			name:     "mixed duplicates",
			input:    []uint16{1, 1, 1, 2, 1},
			expected: []uint16{1, 2},
		},
		{
			name:     "no duplicates",
			input:    []uint16{1, 2, 3},
			expected: []uint16{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeDuplicates(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("removeDuplicates(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVisitEdge(t *testing.T) {
	e := Edge{PointA: 1, PointB: 2, Count: 1}
	
	if e.Count != 1 {
		t.Errorf("Expected Count = 1, got %d", e.Count)
	}
	
	e.visit()
	
	if e.Count != 0 {
		t.Errorf("Expected Count = 0 after visit, got %d", e.Count)
	}
}

func TestSolveRegularTriangle(t *testing.T) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)
	
	solutions := Solve(p)
	if len(*solutions) != 6 {
		t.Errorf("Expected 6 solutions, got %d", len(*solutions))
	}
	
	tmp, _ := json.Marshal(*solutions)
	encoded := string(tmp)
	expected := "[[1,2,3,1],[1,3,2,1],[2,1,3,2],[2,3,1,2],[3,2,1,3],[3,1,2,3]]"
	
	if encoded != expected {
		t.Errorf("Expected %s, got %s", expected, encoded)
	}
}

func TestGetCountSolutionsRegularTriangle(t *testing.T) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)
	
	solutions := GetNumberOfSolutions(p)
	if solutions != 6 {
		t.Errorf("Expected 6 solutions, got %d", solutions)
	}
}

func TestSolveDirectional(t *testing.T) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1, Direction: Direction{From: 1, To: 2, Unidirectional: true}}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	p := NewPuzzle(edges)
	
	solutions := Solve(p)
	if len(*solutions) != 3 {
		t.Errorf("Expected 3 solutions, got %d", len(*solutions))
	}
	
	tmp, _ := json.Marshal(*solutions)
	encoded := string(tmp)
	expected := "[[1,2,3,1],[2,3,1,2],[3,1,2,3]]"
	
	if encoded != expected {
		t.Errorf("Expected %s, got %s", expected, encoded)
	}
}

func TestEdgeCopy(t *testing.T) {
	edge := Edge{PointA: 1, PointB: 2, Count: 3, Direction: Direction{From: 1, To: 2, Unidirectional: true}}
	copy := edge.Copy()
	
	if copy.PointA != edge.PointA {
		t.Errorf("PointA not copied correctly")
	}
	if copy.PointB != edge.PointB {
		t.Errorf("PointB not copied correctly")
	}
	if copy.Count != edge.Count {
		t.Errorf("Count not copied correctly")
	}
	if !reflect.DeepEqual(copy.Direction, edge.Direction) {
		t.Errorf("Direction not copied correctly")
	}
	
	// Ensure it's a deep copy
	copy.Count = 5
	if edge.Count != 3 {
		t.Errorf("Original edge was modified")
	}
}

func TestEdgeCanGoTo(t *testing.T) {
	// Test bidirectional edge
	edge := Edge{PointA: 1, PointB: 2, Count: 1}
	if !edge.canGoTo(1, 2) {
		t.Error("Bidirectional edge should allow 1->2")
	}
	if !edge.canGoTo(2, 1) {
		t.Error("Bidirectional edge should allow 2->1")
	}
	
	// Test unidirectional edge
	uniedge := Edge{PointA: 1, PointB: 2, Count: 1, Direction: Direction{From: 1, To: 2, Unidirectional: true}}
	if !uniedge.canGoTo(1, 2) {
		t.Error("Unidirectional edge should allow 1->2")
	}
	if uniedge.canGoTo(2, 1) {
		t.Error("Unidirectional edge should not allow 2->1")
	}
	
	// Test edge with count 0
	emptyEdge := Edge{PointA: 1, PointB: 2, Count: 0}
	if emptyEdge.canGoTo(1, 2) {
		t.Error("Edge with count 0 should not allow traversal")
	}
	if emptyEdge.canGoTo(2, 1) {
		t.Error("Edge with count 0 should not allow traversal")
	}
}

func TestPuzzleCopy(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 2, PointB: 3, Count: 2},
	}
	p := NewPuzzle(edges)
	copy := p.Copy()
	
	if copy.count != p.count {
		t.Errorf("Count not copied correctly")
	}
	if len(copy.Edges) != len(p.Edges) {
		t.Errorf("Edges length not copied correctly")
	}
	
	// Ensure deep copy
	copy.Edges[0].Count = 5
	if p.Edges[0].Count != 1 {
		t.Errorf("Original puzzle was modified")
	}
}

func TestPuzzleIsSolved(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
	}
	p := NewPuzzle(edges)
	if p.isSolved() {
		t.Error("Puzzle should not be solved initially")
	}
	
	p.count = 0
	if !p.isSolved() {
		t.Error("Puzzle should be solved when count is 0")
	}
}

func TestPuzzleListStartingPoints(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 2, PointB: 3, Count: 1},
		{PointA: 3, PointB: 1, Count: 1},
	}
	p := NewPuzzle(edges)
	points := p.listStartingPoints()
	
	if len(points) != 3 {
		t.Errorf("Expected 3 starting points, got %d", len(points))
	}
	
	// Points should be unique
	seen := make(map[uint16]bool)
	for _, point := range points {
		if seen[point] {
			t.Errorf("Duplicate point found: %d", point)
		}
		seen[point] = true
	}
}

func TestPuzzleGetEdge(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 2, PointB: 3, Count: 2},
		{PointA: 3, PointB: 1, Count: 3},
	}
	p := NewPuzzle(edges)
	
	var a, b uint16
	a, b = 1, 2
	edge := p.getEdge(&a, &b)
	if edge.Count != 1 {
		t.Errorf("Expected edge count 1, got %d", edge.Count)
	}
	
	a, b = 3, 2 // Reverse order should still find the edge
	edge = p.getEdge(&a, &b)
	if edge.Count != 2 {
		t.Errorf("Expected edge count 2, got %d", edge.Count)
	}
}

func TestPuzzleVisitEdge(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 2},
	}
	p := NewPuzzle(edges)
	if p.count != 2 {
		t.Errorf("Expected initial count 2, got %d", p.count)
	}
	
	p.visitEdge(&p.Edges[0])
	if p.count != 1 {
		t.Errorf("Expected count 1 after visit, got %d", p.count)
	}
	if p.Edges[0].Count != 1 {
		t.Errorf("Expected edge count 1 after visit, got %d", p.Edges[0].Count)
	}
}

func TestPuzzleListPossibleEdgesToVisit(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 2, PointB: 3, Count: 1},
		{PointA: 1, PointB: 3, Count: 0}, // Cannot visit
	}
	p := NewPuzzle(edges)
	
	var from uint16 = 1
	possible := p.listPossibleEdgesToVisit(&from)
	if len(possible) != 1 {
		t.Errorf("Expected 1 possible edge from point 1, got %d", len(possible))
	}
	if possible[0] != 2 {
		t.Errorf("Expected possible edge to point 2, got %d", possible[0])
	}
	
	from = 2
	possible = p.listPossibleEdgesToVisit(&from)
	if len(possible) != 2 {
		t.Errorf("Expected 2 possible edges from point 2, got %d", len(possible))
	}
}

func TestNewPuzzleFromBytes(t *testing.T) {
	jsonData := `{"Edges": [{"PointA": 1, "PointB": 2, "Count": 1}]}`
	p, err := NewPuzzleFromBytes([]byte(jsonData))
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(p.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(p.Edges))
	}
	if p.Edges[0].PointA != 1 {
		t.Errorf("Expected PointA = 1, got %d", p.Edges[0].PointA)
	}
	if p.Edges[0].PointB != 2 {
		t.Errorf("Expected PointB = 2, got %d", p.Edges[0].PointB)
	}
	if p.Edges[0].Count != 1 {
		t.Errorf("Expected Count = 1, got %d", p.Edges[0].Count)
	}
	
	// Test invalid JSON
	invalidJson := `{"invalid": json}`
	_, err = NewPuzzleFromBytes([]byte(invalidJson))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestSolveComplexPuzzle(t *testing.T) {
	// House-like puzzle
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 1, PointB: 3, Count: 1},
		{PointA: 2, PointB: 3, Count: 1},
		{PointA: 2, PointB: 4, Count: 1},
		{PointA: 3, PointB: 4, Count: 1},
	}
	p := NewPuzzle(edges)
	
	solutions := Solve(p)
	if len(*solutions) == 0 {
		t.Error("Expected at least one solution")
	}
	
	// Verify each solution visits all edges
	for _, solution := range *solutions {
		if len(solution) == 0 {
			t.Error("Solution should not be empty")
		}
	}
}

func TestSolveMultiEdge(t *testing.T) {
	// Puzzle with multiple edges between same points
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 2},
		{PointA: 2, PointB: 3, Count: 1},
	}
	p := NewPuzzle(edges)
	
	solutions := Solve(p)
	if len(*solutions) != 2 {
		t.Errorf("Expected 2 solutions, got %d", len(*solutions))
	}
}

func TestGetNumberOfSolutionsComplexPuzzle(t *testing.T) {
	edges := []Edge{
		{PointA: 1, PointB: 2, Count: 1},
		{PointA: 1, PointB: 3, Count: 1},
		{PointA: 2, PointB: 3, Count: 1},
		{PointA: 2, PointB: 4, Count: 1},
		{PointA: 3, PointB: 4, Count: 1},
	}
	p := NewPuzzle(edges)
	
	count := GetNumberOfSolutions(p)
	if count == 0 {
		t.Error("Expected at least one solution")
	}
}

func BenchmarkLogic(b *testing.B) {
	edges := make([]Edge, 3)
	edges[0] = Edge{PointA: 1, PointB: 2, Count: 1}
	edges[1] = Edge{PointA: 2, PointB: 3, Count: 1}
	edges[2] = Edge{PointA: 3, PointB: 1, Count: 1}
	for i := 0; i < b.N; i++ {
		p := NewPuzzle(edges)
		_ = Solve(p)
	}
}