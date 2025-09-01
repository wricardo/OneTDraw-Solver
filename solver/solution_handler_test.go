package solver

import (
	"testing"
)

func TestNewSolutionStorer(t *testing.T) {
	storer := newSolutionStorer()
	if storer == nil {
		t.Error("Expected storer to be created")
	}
	if len(storer.solutions) != 0 {
		t.Errorf("Expected 0 solutions initially, got %d", len(storer.solutions))
	}
}

func TestSolutionStorerHandleNewSolutionFound(t *testing.T) {
	storer := newSolutionStorer()
	
	path := []uint16{1, 2, 3}
	storer.handleNewSolutionFound(&path)
	
	if len(storer.solutions) != 1 {
		t.Errorf("Expected 1 solution, got %d", len(storer.solutions))
	}
	if len(*storer.solutions[0]) != 3 {
		t.Errorf("Expected solution length 3, got %d", len(*storer.solutions[0]))
	}
	if (*storer.solutions[0])[0] != 1 {
		t.Errorf("Expected first element 1, got %d", (*storer.solutions[0])[0])
	}
	if (*storer.solutions[0])[1] != 2 {
		t.Errorf("Expected second element 2, got %d", (*storer.solutions[0])[1])
	}
	if (*storer.solutions[0])[2] != 3 {
		t.Errorf("Expected third element 3, got %d", (*storer.solutions[0])[2])
	}
}

func TestSolutionStorerMultipleSolutions(t *testing.T) {
	storer := newSolutionStorer()
	
	path1 := []uint16{1, 2, 3}
	path2 := []uint16{3, 2, 1}
	
	storer.handleNewSolutionFound(&path1)
	storer.handleNewSolutionFound(&path2)
	
	if len(storer.solutions) != 2 {
		t.Errorf("Expected 2 solutions, got %d", len(storer.solutions))
	}
	if len(*storer.solutions[0]) != 3 {
		t.Errorf("Expected first solution length 3, got %d", len(*storer.solutions[0]))
	}
	if len(*storer.solutions[1]) != 3 {
		t.Errorf("Expected second solution length 3, got %d", len(*storer.solutions[1]))
	}
}

func TestSolutionStorerDeepCopy(t *testing.T) {
	storer := newSolutionStorer()
	
	path := []uint16{1, 2, 3}
	storer.handleNewSolutionFound(&path)
	
	// Modify original path
	path[0] = 999
	
	// Stored solution should be unchanged
	if (*storer.solutions[0])[0] != 1 {
		t.Errorf("Expected stored solution to be unchanged, got %d", (*storer.solutions[0])[0])
	}
}

func TestNewSolutionCounter(t *testing.T) {
	counter := newSolutionCounter()
	if counter == nil {
		t.Error("Expected counter to be created")
	}
	if counter.count_solutions != 0 {
		t.Errorf("Expected count 0 initially, got %d", counter.count_solutions)
	}
}

func TestSolutionCounterHandleNewSolutionFound(t *testing.T) {
	counter := newSolutionCounter()
	
	path := []uint16{1, 2, 3}
	counter.handleNewSolutionFound(&path)
	
	if counter.count_solutions != 1 {
		t.Errorf("Expected count 1, got %d", counter.count_solutions)
	}
}

func TestSolutionCounterMultipleSolutions(t *testing.T) {
	counter := newSolutionCounter()
	
	path1 := []uint16{1, 2, 3}
	path2 := []uint16{3, 2, 1}
	path3 := []uint16{2, 1, 3}
	
	counter.handleNewSolutionFound(&path1)
	counter.handleNewSolutionFound(&path2)
	counter.handleNewSolutionFound(&path3)
	
	if counter.count_solutions != 3 {
		t.Errorf("Expected count 3, got %d", counter.count_solutions)
	}
}

func TestSolutionCounterIgnoresPathContent(t *testing.T) {
	counter := newSolutionCounter()
	
	// Test that counter doesn't care about path content, just counts calls
	emptyPath := []uint16{}
	longPath := []uint16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	
	counter.handleNewSolutionFound(&emptyPath)
	counter.handleNewSolutionFound(&longPath)
	
	if counter.count_solutions != 2 {
		t.Errorf("Expected count 2, got %d", counter.count_solutions)
	}
}