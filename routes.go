package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/wricardo/OneTDraw-Solver/solver"
	"io/ioutil"
)

func mapRoutes() {
	goweb.Map("/", func(c context.Context) error {
		return goweb.Respond.WithRedirect(c, "/static/ui2.html")
	})

	goweb.Map("/puzzles", func(c context.Context) error {
		jsonBlob, _ := ioutil.ReadFile("puzzles.json")
		type puzzle struct {
			Name     string
			JsonFile string
		}
		var puzzles []puzzle
		err := json.Unmarshal(jsonBlob, &puzzles)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
		}
		return goweb.API.RespondWithData(c, puzzles)
	})
	goweb.Map("/puzzle/solve/{filename}", func(c context.Context) error {
		filename := fmt.Sprintf("puzzles/%s.json", c.PathParams().Get("filename").Str())
		puzzle, _ := createPuzzleByFilename(&filename)
		solution := solver.Solve(puzzle)
		return goweb.API.RespondWithData(c, solution)
	})
	goweb.Map("/puzzle/get_points/{filename}", func(c context.Context) error {
		filename := fmt.Sprintf("puzzles/%s.json", c.PathParams().Get("filename").Str())
		jsonBlob, _ := ioutil.ReadFile(filename)
		type puzzle struct {
			Points interface{}
			Edges  interface{}
		}
		var p puzzle
		err := json.Unmarshal(jsonBlob, &p)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
		}
		return goweb.API.RespondWithData(c, p)
	})

}
