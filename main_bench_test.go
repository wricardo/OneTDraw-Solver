package main

import (
	"os"
	"testing"

	"github.com/wricardo/OneTDraw-Solver/solver"
)

func BenchmarkCreatePuzzleByFilenameOptimized(b *testing.B) {
	tmpfile, err := os.CreateTemp("", "puzzle*.json")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	puzzleJSON := `{
		"Edges": [
			{"PointA": 1, "PointB": 2, "Count": 1},
			{"PointA": 2, "PointB": 3, "Count": 1},
			{"PointA": 3, "PointB": 1, "Count": 1}
		]
	}`
	if _, err := tmpfile.Write([]byte(puzzleJSON)); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	filename := tmpfile.Name()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = createPuzzleByFilename(&filename)
	}
}

func BenchmarkSolveFileCountOnly(b *testing.B) {
	tmpfile, err := os.CreateTemp("", "puzzle*.json")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	puzzleJSON := `{
		"Edges": [
			{"PointA": 1, "PointB": 2, "Count": 1},
			{"PointA": 2, "PointB": 3, "Count": 1},
			{"PointA": 3, "PointB": 1, "Count": 1}
		]
	}`
	if _, err := tmpfile.Write([]byte(puzzleJSON)); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	count_only = new(bool)
	*count_only = true

	filename := tmpfile.Name()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		puzzle, _ := createPuzzleByFilename(&filename)
		_ = solver.GetNumberOfSolutions(puzzle)
	}
}

func BenchmarkGetPrinter(b *testing.B) {
	output = new(string)
	*output = "json"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getPrinter()
	}
}