package solver

import "os"

// readPuzzleFile reads a puzzle fixture. Kept in one place so tests can load
// the bundled puzzles by name.
func readPuzzleFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
