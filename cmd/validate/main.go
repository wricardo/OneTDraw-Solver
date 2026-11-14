// Command validate checks every map and its reference solution. It is the CLI
// behind `make validate` and exits non-zero when anything is wrong.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wricardo/OneTDraw-Solver/validate"
)

var (
	mapsDir      = flag.String("maps-dir", "maps", "Directory containing map JSON files")
	solutionsDir = flag.String("solutions-dir", "solutions", "Directory containing reference solutions ('' to skip)")
	writeMissing = flag.Bool("write", false, "Write a reference solution for any map that has none")
)

func main() {
	flag.Parse()
	if *writeMissing {
		if err := writeSolutions(*mapsDir, *solutionsDir); err != nil {
			fmt.Fprintf(os.Stderr, "validate: %v\n", err)
			os.Exit(1)
		}
	}
	rep, err := validate.Dir(*mapsDir, *solutionsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate: %v\n", err)
		os.Exit(1)
	}
	for _, m := range rep.Maps {
		fmt.Printf("%-22s nodes=%-3d edges=%-3d steps=%-3d %-10s par=%s\n",
			m.ID, len(m.Nodes), len(m.Edges), m.StepCount, m.Analysis.Verdict, parLabel(m.Par))
	}
	if !rep.OK() {
		for _, issue := range rep.Issues {
			fmt.Fprintf(os.Stderr, "ERROR %s\n", issue)
		}
		fmt.Fprintf(os.Stderr, "validate: %d problem(s) in %d map(s)\n", len(rep.Issues), rep.Checked)
		os.Exit(1)
	}
	fmt.Printf("validate: %d map(s) OK\n", rep.Checked)
}

// parLabel renders par, distinguishing "unreachable" from a real length.
func parLabel(par int) string {
	if par < 0 {
		return "unreachable"
	}
	return fmt.Sprint(par)
}

func writeSolutions(mapsDir, solutionsDir string) error {
	rep, err := validate.Dir(mapsDir, "")
	if err != nil {
		return err
	}
	for _, m := range rep.Maps {
		path := solutionsDir + "/" + m.ID + ".json"
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if m.Par < 0 {
			continue
		}
		if err := validate.WriteSolution(m, path); err != nil {
			return fmt.Errorf("write solution for %s: %w", m.ID, err)
		}
		fmt.Printf("wrote %s\n", path)
	}
	return nil
}
