package main

import (
	"io"
	"os"
	"testing"
)

// writePuzzle writes a puzzle JSON file and returns its path plus a cleanup.
func writePuzzle(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "puzzle*.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	blob, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(blob)
}

const trianglePuzzle = `{
  "Edges": [
    {"PointA": 1, "PointB": 2, "Count": 1},
    {"PointA": 2, "PointB": 3, "Count": 1},
    {"PointA": 3, "PointB": 1, "Count": 1}
  ]
}`

// TestSolveFileCountOnly checks that the CLI mode still counts a classic
// puzzle's walks, which is the behaviour the original tool shipped.
func TestSolveFileCountOnly(t *testing.T) {
	path := writePuzzle(t, trianglePuzzle)

	// Point the flag vars at local values; solveFile reads them.
	oldCount, oldOut := *countOnly, *output
	t.Cleanup(func() {
		*countOnly, *output = oldCount, oldOut
	})
	*countOnly = true
	*output = "json"

	got := captureStdout(t, func() { solveFile(path) })
	// A triangle drawn in one stroke has one distinct picture in each of two
	// rotation directions times two directions per start, so six walks.
	if got != "6\n" {
		t.Errorf("stdout = %q, want %q", got, "6\n")
	}
}

// TestSolveFilePrintsSolutionsInCleanForm checks the default output shape.
func TestSolveFileCleanOutput(t *testing.T) {
	path := writePuzzle(t, trianglePuzzle)

	oldCount, oldOut := *countOnly, *output
	t.Cleanup(func() {
		*countOnly, *output = oldCount, oldOut
	})
	*countOnly = false
	*output = "clean"

	got := captureStdout(t, func() { solveFile(path) })
	if got == "" {
		t.Fatal("clean output printed nothing")
	}
	// Each walk is printed as "a - b - c" on its own line.
	if lines := countLines(got); lines != 6 {
		t.Errorf("printed %d walks, want 6", lines)
	}
}

// TestSolveFileJSONOutput checks that -output json emits a JSON array.
func TestSolveFileJSONOutput(t *testing.T) {
	path := writePuzzle(t, trianglePuzzle)

	oldCount, oldOut := *countOnly, *output
	t.Cleanup(func() {
		*countOnly, *output = oldCount, oldOut
	})
	*countOnly = false
	*output = "json"

	got := captureStdout(t, func() { solveFile(path) })
	if len(got) == 0 || got[0] != '[' {
		t.Errorf("json output = %q, want a JSON array", got)
	}
}

func countLines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

// TestShippedPuzzlesStillLoad guards the CLI mode against the bundle being
// deleted or renamed out from under it.
func TestShippedPuzzlesStillLoad(t *testing.T) {
	entries, err := os.ReadDir("puzzles")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("puzzles/ is empty")
	}
}

func BenchmarkSolveFileCountOnly(b *testing.B) {
	path := writePuzzle(&testing.T{}, trianglePuzzle)
	oldCount := *countOnly
	defer func() { *countOnly = oldCount }()
	*countOnly = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// The analysis line goes to stderr; stdout is redirected so the
		// benchmark's own output stays readable.
		old := os.Stdout
		devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err == nil {
			os.Stdout = devnull
		}
		solveFile(path) //nolint:errcheck // the CLI exits on error
		os.Stdout = old
		if devnull != nil {
			devnull.Close()
		}
	}
}
