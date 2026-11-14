//go:build ignore

// convert_puzzles copies puzzles/*.json into maps/*.json in the game's map
// format. The two formats share field names, so this only adds the metadata a
// map needs (id, name, difficulty) and skips legacy files that are not graphs.
//
// Run it with:
//
//	go run scripts/convert_puzzles.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

// catalog maps a puzzle file base name to its display metadata.
var catalog = map[string]struct {
	Name        string
	Description string
	Difficulty  string
}{
	"regular_triangle":     {"Regular Triangle", "A triangle drawn without lifting the pen. A gentle first puzzle.", "easy"},
	"directional_triangle": {"Directional Triangle", "The triangle again, but one side is one-way: find the corner it must start from.", "easy"},
	"littlet":              {"Little T", "A small T shape with a doubled edge and a one-way climb. A tidy warm-up.", "easy"},
	"jamaican_flag":        {"Jamaican Flag", "A flag drawn in one line: two triangles across a pair of diagonals.", "easy"},
	"jose":                 {"Jose", "A two-storey figure with four odd corners. Impossible without a repeat: can you prove it?", "medium"},
	"house":                {"House", "The classic house: a square under a triangular roof, drawn with one stroke from the bottom corner.", "medium"},
	"level53":              {"Level 53", "Four floors joined by long corridors, with two one-way drops near the top.", "hard"},
	"level54":              {"Level 54", "A tall ladder with doubled rungs and two one-way descents. The trickiest bundled map.", "hard"},
	"level55":              {"Level 55", "A ring of rooms with one-way doors that force the order you visit them.", "hard"},
	"level56":              {"Level 56", "A compact web of crossings where the doubled edges are the only way through.", "hard"},
	"level57":              {"Level 57", "A wide grid with four one-way links deciding which corner you start from.", "hard"},
}

func main() {
	files, err := filepath.Glob("puzzles/*.json")
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(files)
	if err := os.MkdirAll("maps", 0o755); err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".json")
		meta, ok := catalog[base]
		if !ok {
			log.Printf("skip %s: not in the catalog", f)
			continue
		}
		blob, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		var cfg engine.MapConfig
		if err := json.Unmarshal(blob, &cfg); err != nil {
			log.Printf("skip %s: %v", f, err)
			continue
		}
		cfg.ID = base
		cfg.Name = meta.Name
		cfg.Description = meta.Description
		cfg.Difficulty = meta.Difficulty
		cfg.MaxMoves = 0 // NewMap derives it from the map's step count.

		m, err := engine.NewMap(cfg)
		if err != nil {
			log.Fatalf("%s: %v", f, err)
		}
		out := m.Config()
		out.MaxMoves = 0
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		data = append(data, '\n')
		dest := filepath.Join("maps", base+".json")
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%-22s nodes=%-3d edges=%-3d steps=%-3d par=%d\n", base, len(m.Nodes), len(m.Edges), m.StepCount, m.Par)
	}
}
