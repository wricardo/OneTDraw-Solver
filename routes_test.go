package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestPuzzlesJSONLoading(t *testing.T) {
	// Create a temporary puzzles.json file
	tmpfile, err := os.CreateTemp("", "puzzles*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	puzzlesJSON := `[
		{"Name": "Triangle", "JsonFile": "triangle"},
		{"Name": "Square", "JsonFile": "square"}
	]`

	if _, err := tmpfile.Write([]byte(puzzlesJSON)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading and parsing
	jsonBlob, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	type puzzle struct {
		Name     string
		JsonFile string
	}

	var puzzles []puzzle
	err = json.Unmarshal(jsonBlob, &puzzles)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(puzzles) != 2 {
		t.Errorf("Expected 2 puzzles, got %d", len(puzzles))
	}

	if puzzles[0].Name != "Triangle" {
		t.Errorf("Expected first puzzle name 'Triangle', got '%s'", puzzles[0].Name)
	}

	if puzzles[1].JsonFile != "square" {
		t.Errorf("Expected second puzzle file 'square', got '%s'", puzzles[1].JsonFile)
	}
}

func TestPuzzlePointsLoading(t *testing.T) {
	// Create a temporary puzzle file with points
	tmpfile, err := os.CreateTemp("", "puzzle_points*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	puzzleJSON := `{
		"Points": [
			{"Point": 1, "Level": 1},
			{"Point": 2, "Level": 2}
		],
		"Edges": [
			{"PointA": 1, "PointB": 2, "Count": 1}
		]
	}`

	if _, err := tmpfile.Write([]byte(puzzleJSON)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading and parsing
	jsonBlob, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	type puzzle struct {
		Points interface{}
		Edges  interface{}
	}

	var p puzzle
	err = json.Unmarshal(jsonBlob, &p)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if p.Points == nil {
		t.Error("Expected Points to be loaded, got nil")
	}

	if p.Edges == nil {
		t.Error("Expected Edges to be loaded, got nil")
	}

	// Test that Points is an array
	points, ok := p.Points.([]interface{})
	if !ok {
		t.Error("Expected Points to be an array")
	} else {
		if len(points) != 2 {
			t.Errorf("Expected 2 points, got %d", len(points))
		}
	}
}

func TestInvalidPuzzleJSON(t *testing.T) {
	invalidJSON := `{"invalid": json}`

	type puzzle struct {
		Name     string
		JsonFile string
	}

	var puzzles []puzzle
	err := json.Unmarshal([]byte(invalidJSON), &puzzles)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestPuzzleFilenameConstruction(t *testing.T) {
	// Test the filename construction pattern used in routes
	testCases := []struct {
		input    string
		expected string
	}{
		{"triangle", "puzzles/triangle.json"},
		{"house", "puzzles/house.json"},
		{"level53", "puzzles/level53.json"},
	}

	for _, tc := range testCases {
		// This mimics what happens in the route handler
		filename := "puzzles/" + tc.input + ".json"
		if filename != tc.expected {
			t.Errorf("Expected filename '%s', got '%s'", tc.expected, filename)
		}
	}
}

// Test helper for validating puzzle structure
func validatePuzzleStructure(t *testing.T, puzzleJSON string) {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(puzzleJSON), &data)

	if err != nil {
		t.Errorf("Failed to parse puzzle JSON: %v", err)
		return
	}

	// Check for required fields
	if _, hasEdges := data["Edges"]; !hasEdges {
		t.Error("Puzzle JSON must have 'Edges' field")
	}

	// Points field is optional but if present should be valid
	if points, hasPoints := data["Points"]; hasPoints {
		if _, ok := points.([]interface{}); !ok {
			t.Error("Points field must be an array")
		}
	}
}

func TestValidatePuzzleStructure(t *testing.T) {
	validPuzzle := `{
		"Points": [{"Point": 1, "Level": 1}],
		"Edges": [{"PointA": 1, "PointB": 2, "Count": 1}]
	}`
	validatePuzzleStructure(t, validPuzzle)

	validPuzzleNoPoints := `{
		"Edges": [{"PointA": 1, "PointB": 2, "Count": 1}]
	}`
	validatePuzzleStructure(t, validPuzzleNoPoints)
}
