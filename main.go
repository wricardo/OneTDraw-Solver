package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"runtime"
	"star/solver"
)

var count_only *bool;

func main() {
	var maxprocs = flag.Bool("maxprocs", true, "Pass false to NOT use all CPU cores available")
	count_only = flag.Bool("count_only", false, "Pass true to display only the count of possible solutions")
	var puzzle_file_path = flag.String("solve", "", "File path to puzzle to solve")
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

func createPuzzleByFilename(puzzle_file_path *string) *solver.Puzzle {
	file_content, _ := ioutil.ReadFile(*puzzle_file_path)

	var puzzle solver.Puzzle
	err := json.Unmarshal(file_content, &puzzle)
	if err != nil {
		fmt.Println("Invalid JSON. Error:", err)
	}
	return &puzzle
}

func solveFile(puzzle_file_path string) {
	puzzle := createPuzzleByFilename(&puzzle_file_path)
	if *count_only {
		fmt.Println(solver.GetNumberOfSolutions(solver.NewPuzzle(puzzle.Edges)))
	} else {
		solutions := solver.Solve(solver.NewPuzzle(puzzle.Edges))
		fmt.Println(*solutions)
	}
}

func setupWebServer() {
	ws := Webserver{Address: ":8090"}
	ws.init()
}
