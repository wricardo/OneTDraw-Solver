package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/wricardo/OneTDraw-Solver/solver"
)

var count_only *bool
var output *string
var budget *time.Duration

func main() {
	var maxprocs = flag.Bool("maxprocs", true, "Pass false to NOT use all CPU cores available")
	var puzzle_file_path = flag.String("solve", "", "File path to puzzle to solve")
	count_only = flag.Bool("count_only", false, "Pass true to display only the count of possible solutions")
	output = flag.String("output", "clean", "Format of the output. [clean,json]")
	budget = flag.Duration("budget", 0, "Stop after this long and print what was found, e.g. -budget 10s (0 = no limit)")
	flag.Parse()

	if *maxprocs == true {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	if len(*puzzle_file_path) > 0 {
		solveFile(*puzzle_file_path)
	} else {
		setupWebServer()
	}
}

func createPuzzleByFilename(puzzle_file_path *string) (*solver.Puzzle, error) {
	file_content, err := os.ReadFile(*puzzle_file_path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", *puzzle_file_path, err)
	}
	return solver.NewPuzzleFromBytes(file_content)
}

func getPrinter() solver.SolutionPrinter {
	if *output == "json" {
		return solver.JsonPrinter{}
	} else {
		return solver.CleanPrinter{}
	}
}

// budgetOrZero returns the CLI budget, tolerating a nil flag so the helper stays
// callable from tests that only initialise the flags they care about.
func budgetOrZero() time.Duration {
	if budget == nil {
		return 0
	}
	return *budget
}

func solveFile(puzzle_file_path string) {
	puzzle, err := createPuzzleByFilename(&puzzle_file_path)
	if err != nil {
		log.Fatalf("Error loading puzzle: %v", err)
	}

	// The analysis goes to stderr so it never corrupts the machine-readable
	// stdout that -count_only and -output json produce.
	fmt.Fprintf(os.Stderr, "%s\n", solver.Analyze(puzzle))

	if *count_only {
		count, stats := solver.CountWith(puzzle, budgetOrZero(), nil)
		if stats.Exhaustive {
			fmt.Println(count)
			return
		}
		fmt.Fprintf(os.Stderr, "Count incomplete after %v: at least %d walks (%d branches)\n",
			stats.Elapsed.Round(time.Millisecond), count, stats.Visited)
		fmt.Println(count)
		return
	}

	solutions, stats := solver.SolveWith(puzzle, 0, budgetOrZero(), nil, nil)
	if !stats.Exhaustive {
		fmt.Fprintf(os.Stderr, "Search incomplete after %v: %d walks found, %d branches explored\n",
			stats.Elapsed.Round(time.Millisecond), len(*solutions), stats.Visited)
	}
	solutions.Print(getPrinter())
}

func setupWebServer() {
	ws := Webserver{Address: ":8090"}
	ws.init()
}
