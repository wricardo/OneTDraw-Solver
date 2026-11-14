// Package validate checks that every shipped map parses, that its graph is
// sound, and that a reference solution really draws every edge exactly as many
// times as the map demands.
//
// Keeping this separate from the engine lets `make validate` gate a release on
// data quality without the server having to trust its own inputs at runtime.
package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

// Issue is one problem found in one map.
type Issue struct {
	Map     string `json:"map"`
	Message string `json:"message"`
}

func (i Issue) String() string { return fmt.Sprintf("%s: %s", i.Map, i.Message) }

// Report is the outcome of validating a maps directory.
type Report struct {
	// Checked counts maps that parsed and were inspected.
	Checked int
	// Maps lists the parsed maps by id.
	Maps []*engine.Map
	// Issues lists every problem found, empty when everything is sound.
	Issues []Issue
}

// OK reports whether the whole report is clean.
func (r Report) OK() bool { return len(r.Issues) == 0 }

// Dir validates every map in mapsDir and, when solutionsDir is non-empty,
// checks each map's reference solution in solutionsDir/<id>.json.
//
// A solution file holds {"moves": [...]} where moves are the nodes to travel to
// after the map's own start; the start node is chosen by the file's optional
// "start" field, or else by the solver for a map whose trail may start
// anywhere.
func Dir(mapsDir, solutionsDir string) (Report, error) {
	var rep Report
	entries, err := os.ReadDir(mapsDir)
	if err != nil {
		return rep, fmt.Errorf("read maps dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		id := strings.TrimSuffix(name, ".json")
		path := filepath.Join(mapsDir, name)
		blob, err := os.ReadFile(path)
		if err != nil {
			rep.Issues = append(rep.Issues, Issue{Map: id, Message: err.Error()})
			continue
		}
		var cfg engine.MapConfig
		if err := json.Unmarshal(blob, &cfg); err != nil {
			rep.Issues = append(rep.Issues, Issue{Map: id, Message: "invalid JSON: " + err.Error()})
			continue
		}
		if cfg.ID != "" && cfg.ID != id {
			rep.Issues = append(rep.Issues, Issue{Map: id, Message: fmt.Sprintf("map id %q does not match file name", cfg.ID)})
		}
		cfg.ID = id
		m, err := engine.NewMap(cfg)
		if err != nil {
			rep.Issues = append(rep.Issues, Issue{Map: id, Message: err.Error()})
			continue
		}
		rep.Checked++
		rep.Maps = append(rep.Maps, m)

		rep.Issues = append(rep.Issues, checkMap(m)...)

		if solutionsDir != "" {
			rep.Issues = append(rep.Issues, checkSolution(m, filepath.Join(solutionsDir, id+".json"))...)
		}
	}
	return rep, nil
}

// checkMap applies the invariants that do not need a solution.
func checkMap(m *engine.Map) []Issue {
	var issues []Issue
	add := func(format string, args ...any) {
		issues = append(issues, Issue{Map: m.ID, Message: fmt.Sprintf(format, args...)})
	}
	if m.StepCount == 0 {
		add("no traversals required: the map has no usable edges")
	}
	if m.Analysis.Verdict == engine.VerdictImpossible {
		// An impossible map is legal to ship as a puzzle (jose is one) but the
		// par must stay unset so the UI does not advertise a solution.
		if m.Par >= 0 {
			add("verdict is IMPOSSIBLE but par is %d", m.Par)
		}
	} else if m.Par < 0 {
		add("no complete trail found even though the verdict is %s", m.Analysis.Verdict)
	} else if m.Par != m.StepCount {
		add("par %d does not equal the step count %d", m.Par, m.StepCount)
	}
	if m.MaxMoves < 1 {
		add("max_moves must be at least 1")
	}
	return issues
}

// solutionFile is the on-disk form of a reference solution.
type solutionFile struct {
	// Start is the node the trail begins at. Zero is ambiguous with "unset", so
	// it is a pointer.
	Start *int  `json:"start,omitempty"`
	Moves []int `json:"moves"`
}

// checkSolution confirms the stored reference solution draws every edge. A
// missing solutions directory is not an error: not every map ships one.
func checkSolution(m *engine.Map, path string) []Issue {
	add := func(format string, args ...any) []Issue {
		return []Issue{{Map: m.ID, Message: fmt.Sprintf(format, args...)}}
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return add("read solution: %v", err)
	}
	var sf solutionFile
	if err := json.Unmarshal(blob, &sf); err != nil {
		return add("invalid solution JSON: %v", err)
	}
	if len(sf.Moves) == 0 {
		return add("solution has no moves")
	}

	var pathNodes []int
	if sf.Start != nil {
		pathNodes = append([]int{*sf.Start}, sf.Moves...)
	} else {
		// No start recorded: ask the solver for a trail that matches the moves,
		// which for every bundled map is simply to prefix the first move's
		// predecessor. Trying each parity-legal start covers that.
		pathNodes = []int{sf.Moves[0]}
		found := false
		for _, start := range m.Analysis.StartNodes {
			trial := append([]int{start}, sf.Moves...)
			if engine.ValidSolution(m, trial) == nil {
				pathNodes = trial
				found = true
				break
			}
		}
		if !found {
			return add("solution does not cover every edge from any allowed start")
		}
	}
	if err := engine.ValidSolution(m, pathNodes); err != nil {
		return add("solution invalid: %v", err)
	}
	return nil
}

// WriteSolution is a helper for the map-authoring workflow: it solves m and
// writes a reference solution to path.
func WriteSolution(m *engine.Map, path string) error {
	plan, err := engine.Plan(m, engine.NewState(m))
	if err != nil {
		return err
	}
	if !plan.Found {
		return fmt.Errorf("map %s has no solution to record", m.ID)
	}
	sf := solutionFile{Start: &plan.Start, Moves: plan.Steps}
	if sf.Moves == nil {
		sf.Moves = []int{}
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
