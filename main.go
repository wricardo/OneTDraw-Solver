package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/wricardo/OneTDraw-Solver/solver"
)

var count_only *bool
var output *string

func main() {
	var maxprocs = flag.Bool("maxprocs", true, "Pass false to NOT use all CPU cores available")
	var puzzle_file_path = flag.String("solve", "", "File path to puzzle to solve")
	count_only = flag.Bool("count_only", false, "Pass true to display only the count of possible solutions")
	output = flag.String("output", "clean", "Format of the output. [clean,json]")
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

func solveFile(puzzle_file_path string) {
	puzzle, err := createPuzzleByFilename(&puzzle_file_path)
	if err != nil {
		log.Fatalf("Error loading puzzle: %v", err)
	}
	if *count_only {
		fmt.Println(solver.GetNumberOfSolutions(puzzle))
	} else {
		solutions := solver.Solve(puzzle)
		solutions.Print(getPrinter())
	}
}

func setupWebServer() {
	ws := Webserver{Address: ":8090"}
	ws.init()
}
