package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/wricardo/OneTDraw-Solver/solver"
)

func validateFilename(filename string) error {
	cleaned := filepath.Clean(filename)
	if strings.Contains(cleaned, "..") || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("invalid filename: %s", filename)
	}
	if !strings.HasSuffix(cleaned, ".json") {
		return fmt.Errorf("only .json files allowed")
	}
	return nil
}

func mapRoutes() {
	goweb.Map("/", func(c context.Context) error {
		return goweb.Respond.WithRedirect(c, "/static/ui2.html")
	})

	goweb.Map("/puzzles", func(c context.Context) error {
		jsonBlob, err := os.ReadFile("puzzles.json")
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Could not read puzzles.json"))
		}
		type puzzle struct {
			Name     string
			JsonFile string
		}
		var puzzles []puzzle
		err = json.Unmarshal(jsonBlob, &puzzles)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
		}
		return goweb.API.RespondWithData(c, puzzles)
	})
	goweb.Map("/puzzle/solve/{filename}", func(c context.Context) error {
		filenameParam := c.PathParams().Get("filename").Str()
		if err := validateFilename(filenameParam + ".json"); err != nil {
			return goweb.Respond.With(c, 400, []byte("ERROR: "+err.Error()))
		}
		filename := fmt.Sprintf("puzzles/%s.json", filenameParam)
		puzzle, err := createPuzzleByFilename(&filename)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Could not load puzzle"))
		}
		solution := solver.Solve(puzzle)
		return goweb.API.RespondWithData(c, solution)
	})
	goweb.Map("/puzzle/get_points/{filename}", func(c context.Context) error {
		filenameParam := c.PathParams().Get("filename").Str()
		if err := validateFilename(filenameParam + ".json"); err != nil {
			return goweb.Respond.With(c, 400, []byte("ERROR: "+err.Error()))
		}
		filename := fmt.Sprintf("puzzles/%s.json", filenameParam)
		jsonBlob, err := os.ReadFile(filename)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Could not read puzzle file"))
		}
		type puzzle struct {
			Points interface{}
			Edges  interface{}
		}
		var p puzzle
		err = json.Unmarshal(jsonBlob, &p)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
		}
		return goweb.API.RespondWithData(c, p)
	})

}
