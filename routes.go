package main

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"io/ioutil"
	"star/solver"
)

func mapRoutes() {
	goweb.Map("/", func(c context.Context) error {
		return goweb.Respond.WithRedirect(c, "/static/ui2.html")
		//return goweb.Respond.With(c, 200, []byte("Welcome to the Goweb example app - see the terminal for instructions."))
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
		fmt.Println(filename)
		puzzle := createPuzzleByFilename(&filename)
		solution := solver.Solve(solver.NewPuzzle(puzzle.Edges))
		return goweb.API.RespondWithData(c, solution)
		//return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
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
		//return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
	})

}
