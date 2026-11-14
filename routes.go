package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/goweb"
	"github.com/stretchr/goweb/context"
	"github.com/wricardo/OneTDraw-Solver/solver"
)

// defaultSolveBudget bounds how long a streaming solve keeps searching before
// it reports what it has. Enumeration alone cannot finish puzzles whose walk
// space is astronomically large (level54), so a bounded search that streams
// results and can be stopped is more useful than one that never answers.
// Callers may override it with ?budgetMs=.
const (
	defaultSolveBudget = 8 * time.Second
	maxSolveBudget     = 2 * time.Minute
	defaultSolveLimit  = 20000
	maxSolveLimit      = 50000
	solveStreamTick    = 150 * time.Millisecond
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

type puzzleSummary struct {
	Name      string `json:"Name"`
	JsonFile  string `json:"JsonFile"`
	NodeCount int    `json:"nodeCount"`
	EdgeCount int    `json:"edgeCount"`
	StepCount int    `json:"stepCount"`
	Verdict   string `json:"verdict"`
}

type puzzlePoint struct {
	Point int      `json:"Point"`
	Level int      `json:"Level"`
	Hide  flexBool `json:"Hide"`
}

// flexBool accepts true/false or 1/0, because the bundled puzzles use both
// spellings for the Hide flag that marks layout-only spacer nodes.
type flexBool bool

func (this *flexBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(string(data), `"`) {
	case "true", "1":
		*this = true
	case "false", "0", "", "null":
		*this = false
	default:
		return fmt.Errorf("invalid boolean value %s", string(data))
	}
	return nil
}

type puzzleFile struct {
	Points []puzzlePoint `json:"Points"`
	Edges  []solver.Edge `json:"Edges"`
}

// puzzleData is everything the UI needs to render one puzzle before solving.
type puzzleData struct {
	File     string           `json:"file"`
	Points   []puzzlePoint    `json:"points"`
	Edges    []solver.Edge    `json:"edges"`
	Analysis analysisResponse `json:"analysis"`
}

type analysisResponse struct {
	NodeCount   int             `json:"nodeCount"`
	EdgeCount   int             `json:"edgeCount"`
	StepCount   int             `json:"stepCount"`
	Verdict     string          `json:"verdict"`
	Connected   bool            `json:"connected"`
	Directional bool            `json:"directional"`
	OddVertices []uint16        `json:"oddVertices"`
	Degrees     []solver.Degree `json:"degrees"`
	Message     string          `json:"message"`
}

type solutionResponse struct {
	Solutions   solver.Solutions `json:"solutions"`
	UniqueIndex []int            `json:"uniqueIndex"` // indices into Solutions, one per distinct drawing
	Found       int              `json:"found"`
	Unique      int              `json:"unique"`
	Truncated   bool             `json:"truncated"`
	TimedOut    bool             `json:"timedOut"`
	ElapsedMS   int64            `json:"elapsedMs"`
	Limit       int              `json:"limit"`
}

func loadPuzzleFile(name string) (*puzzleFile, error) {
	if err := validateFilename(name + ".json"); err != nil {
		return nil, err
	}
	blob, err := os.ReadFile(fmt.Sprintf("puzzles/%s.json", name))
	if err != nil {
		return nil, fmt.Errorf("could not read puzzle file: %w", err)
	}
	var pf puzzleFile
	if err := json.Unmarshal(blob, &pf); err != nil {
		return nil, fmt.Errorf("invalid puzzle JSON: %w", err)
	}
	seen := make(map[int]bool, len(pf.Points))
	for _, p := range pf.Points {
		seen[p.Point] = true
	}
	for _, e := range pf.Edges {
		for _, n := range []uint16{e.PointA, e.PointB} {
			if !seen[int(n)] {
				// Puzzles may omit Points entirely; give stray nodes level 0 so
				// the client still has something to lay out.
				pf.Points = append(pf.Points, puzzlePoint{Point: int(n), Level: 0})
				seen[int(n)] = true
			}
		}
	}
	return &pf, nil
}

func toSolverPuzzle(pf *puzzleFile) *solver.Puzzle {
	return solver.NewPuzzle(pf.Edges)
}

func analysisResponseFrom(a solver.Analysis) analysisResponse {
	odd := a.OddVertices
	if odd == nil {
		odd = []uint16{}
	}
	degrees := a.Degrees
	if degrees == nil {
		degrees = []solver.Degree{}
	}
	return analysisResponse{
		NodeCount:   a.NodeCount,
		EdgeCount:   a.EdgeCount,
		StepCount:   a.StepCount,
		Verdict:     a.Verdict,
		Connected:   a.Connected,
		Directional: a.Directional,
		OddVertices: odd,
		Degrees:     degrees,
		Message:     a.String(),
	}
}

func intQuery(c context.Context, key string, fallback int) int {
	raw := c.HttpRequest().URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func mapRoutes() {
	goweb.Map("/", func(c context.Context) error {
		return goweb.Respond.WithRedirect(c, "/static/index.html")
	})

	goweb.Map("/puzzles", func(c context.Context) error {
		jsonBlob, err := os.ReadFile("puzzles.json")
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Could not read puzzles.json"))
		}
		var puzzles []puzzleSummary
		if err := json.Unmarshal(jsonBlob, &puzzles); err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: Invalid JSON"))
		}
		// Decorate each entry with its size and a solvability verdict, so the
		// list conveys something before the user clicks.
		for i := range puzzles {
			name := strings.TrimSuffix(puzzles[i].JsonFile, ".json")
			pf, err := loadPuzzleFile(name)
			if err != nil {
				continue
			}
			a := solver.Analyze(toSolverPuzzle(pf))
			puzzles[i].NodeCount = a.NodeCount
			puzzles[i].EdgeCount = a.EdgeCount
			puzzles[i].StepCount = a.StepCount
			puzzles[i].Verdict = a.Verdict
		}
		return goweb.API.RespondWithData(c, puzzles)
	})

	// /puzzle/data/{name} returns the puzzle geometry plus a cheap analysis.
	goweb.Map("/puzzle/data/{filename}", func(c context.Context) error {
		name := c.PathParams().Get("filename").Str()
		pf, err := loadPuzzleFile(name)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: "+err.Error()))
		}
		data := puzzleData{
			File:     name,
			Points:   pf.Points,
			Edges:    pf.Edges,
			Analysis: analysisResponseFrom(solver.Analyze(toSolverPuzzle(pf))),
		}
		return goweb.API.RespondWithData(c, data)
	})

	goweb.Map("/puzzle/solve/{filename}", func(c context.Context) error {
		name := c.PathParams().Get("filename").Str()
		pf, err := loadPuzzleFile(name)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: "+err.Error()))
		}

		limit := clampInt(intQuery(c, "limit", defaultSolveLimit), 0, maxSolveLimit)
		budget := time.Duration(clampInt(intQuery(c, "budgetMs", int(defaultSolveBudget/time.Millisecond)), 0, int(maxSolveBudget/time.Millisecond))) * time.Millisecond

		started := time.Now()
		solutions, stats := solver.SolveWith(toSolverPuzzle(pf), limit, budget, nil, nil)
		elapsed := time.Since(started)

		uniqueIndex := solver.UniqueIndices(solutions)
		resp := solutionResponse{
			Solutions:   *solutions,
			UniqueIndex: uniqueIndex,
			Found:       len(*solutions),
			Unique:      len(uniqueIndex),
			Truncated:   !stats.Exhaustive,
			TimedOut:    stats.TimedOut,
			ElapsedMS:   elapsed.Milliseconds(),
			Limit:       limit,
		}
		return goweb.API.RespondWithData(c, resp)
	})

	// /puzzle/stream/{name} streams the search as newline-delimited JSON events:
	// "progress" while it runs, then "result" with the solutions found so far.
	// Closing the connection stops the search, which is what lets the UI offer a
	// Stop button instead of making the user wait out a search they don't want.
	goweb.Map("/puzzle/stream/{filename}", func(c context.Context) error {
		name := c.PathParams().Get("filename").Str()
		pf, err := loadPuzzleFile(name)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: "+err.Error()))
		}

		limit := clampInt(intQuery(c, "limit", defaultSolveLimit), 0, maxSolveLimit)
		budget := time.Duration(clampInt(intQuery(c, "budgetMs", int(defaultSolveBudget/time.Millisecond)), 0, int(maxSolveBudget/time.Millisecond))) * time.Millisecond

		// A client-supplied id lets Stop request a graceful finish over a second
		// request, so the stream stays open long enough to deliver the solutions
		// found so far. Aborting the stream instead would discard them.
		session := c.HttpRequest().URL.Query().Get("session")
		stop := make(chan struct{})
		if session != "" {
			registerSolveSession(session, stop)
			defer unregisterSolveSession(session)
		}

		w := newNDJSONStream(c)
		defer w.close()

		// A ticker keeps a heartbeat flowing while the search explores branches
		// that have not produced a solution yet, so the client can distinguish
		// "working" from "hung". It is joined before the handler returns: a
		// flush after that point writes to a recycled ResponseWriter and panics.
		progress := func(stats solver.SolveStats) {
			w.write(streamProgress{Event: "progress", Found: stats.Found, Visited: stats.Visited})
		}
		w.startHeartbeat()

		go cancelOnDisconnect(c, w.done, stop)

		solutions, stats := solver.SolveWith(toSolverPuzzle(pf), limit, budget, progress, stop)
		uniqueIndex := solver.UniqueIndices(solutions)

		w.write(streamResult{
			Event:       "result",
			Solutions:   *solutions,
			UniqueIndex: uniqueIndex,
			Found:       len(*solutions),
			Unique:      len(uniqueIndex),
			Exhaustive:  stats.Exhaustive,
			TimedOut:    stats.TimedOut,
			Cancelled:   stats.Cancelled,
			Limited:     stats.Limited,
			Visited:     stats.Visited,
			ElapsedMS:   stats.Elapsed.Milliseconds(),
			Limit:       limit,
		})
		return nil
	})

	// /puzzle/count/{name} counts exhaustively. Counting is usually the slower
	// operation, so it streams progress the same way and stops if the client
	// disconnects.
	goweb.Map("/puzzle/count/{filename}", func(c context.Context) error {
		name := c.PathParams().Get("filename").Str()
		pf, err := loadPuzzleFile(name)
		if err != nil {
			return goweb.Respond.With(c, 404, []byte("ERROR: "+err.Error()))
		}
		budget := time.Duration(clampInt(intQuery(c, "budgetMs", int(maxSolveBudget/time.Millisecond)), 0, int(maxSolveBudget/time.Millisecond))) * time.Millisecond

		session := c.HttpRequest().URL.Query().Get("session")
		stop := make(chan struct{})
		if session != "" {
			registerSolveSession(session, stop)
			defer unregisterSolveSession(session)
		}

		w := newNDJSONStream(c)
		defer w.close()
		w.startHeartbeat()

		go cancelOnDisconnect(c, w.done, stop)

		progress := func(stats solver.SolveStats) {
			w.write(streamProgress{Event: "progress", Found: stats.Found, Visited: stats.Visited})
		}

		count, stats := solver.CountWithProgress(toSolverPuzzle(pf), budget, stop, progress)
		w.write(countResult{
			Event:     "result",
			Count:     count,
			Complete:  stats.Exhaustive,
			TimedOut:  stats.TimedOut,
			Cancelled: stats.Cancelled,
			Visited:   stats.Visited,
			ElapsedMS: stats.Elapsed.Milliseconds(),
		})
		return nil
	})

	// /puzzle/stop/{session} asks a running stream to finish early and still
	// report what it found. Without it, the only way to stop a search is to drop
	// the connection, which also throws away the results.
	goweb.Map("/puzzle/stop/{session}", func(c context.Context) error {
		session := c.PathParams().Get("session").Str()
		if session == "" {
			return goweb.Respond.With(c, 400, []byte("ERROR: missing session"))
		}
		if requestStop(session) {
			return goweb.API.RespondWithData(c, map[string]interface{}{"stopped": true})
		}
		return goweb.API.RespondWithData(c, map[string]interface{}{"stopped": false})
	})
}

// solveSessions tracks in-flight streams so a stop request can reach them.
var solveSessions = struct {
	mu   sync.Mutex
	byID map[string]chan struct{}
}{byID: make(map[string]chan struct{})}

func registerSolveSession(id string, stop chan struct{}) {
	solveSessions.mu.Lock()
	solveSessions.byID[id] = stop
	solveSessions.mu.Unlock()
}

func unregisterSolveSession(id string) {
	solveSessions.mu.Lock()
	delete(solveSessions.byID, id)
	solveSessions.mu.Unlock()
}

// requestStop asks the named session to finish. It reports whether the session
// existed, so the caller can tell "stopped" from "already finished".
func requestStop(id string) bool {
	solveSessions.mu.Lock()
	stop, ok := solveSessions.byID[id]
	solveSessions.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
	return true
}

// ndjsonStream writes newline-delimited JSON to a streaming HTTP response.
//
// It exists to make the shutdown order safe. The heartbeat goroutine and the
// request handler both write, and the ResponseWriter must not be touched once
// the handler returns, so close() marks the stream dead under the same mutex a
// write takes, then joins the heartbeat before the handler can return.
type ndjsonStream struct {
	mu       sync.Mutex
	enc      *json.Encoder
	flusher  http.Flusher
	canFlush bool
	closed   bool
	failed   bool

	done      chan struct{}
	hbStarted bool
	hbWG      sync.WaitGroup
}

func newNDJSONStream(c context.Context) *ndjsonStream {
	w := c.HttpResponseWriter()
	flusher, canFlush := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	// A progress feed is worthless if a proxy buffers it until the end.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	if canFlush {
		flusher.Flush()
	}
	return &ndjsonStream{
		enc:      json.NewEncoder(w),
		flusher:  flusher,
		canFlush: canFlush,
		done:     make(chan struct{}),
	}
}

// write sends one event. Failures are remembered rather than returned, because
// the only cause is a client that went away, which the search already reacts to.
func (this *ndjsonStream) write(v interface{}) {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.closed || this.failed {
		return
	}
	if err := this.enc.Encode(v); err != nil {
		this.failed = true
		return
	}
	if this.canFlush {
		this.flusher.Flush()
	}
}

// startHeartbeat keeps a keep-alive line flowing on a fixed cadence.
func (this *ndjsonStream) startHeartbeat() {
	this.hbStarted = true
	this.hbWG.Add(1)
	go func() {
		defer this.hbWG.Done()
		ticker := time.NewTicker(solveStreamTick)
		defer ticker.Stop()
		for {
			select {
			case <-this.done:
				return
			case <-ticker.C:
				this.write(streamProgress{Event: "heartbeat"})
			}
		}
	}()
}

// close stops the heartbeat, waits for it, and makes later writes no-ops. It is
// idempotent so it can be deferred safely.
func (this *ndjsonStream) close() {
	this.mu.Lock()
	if this.closed {
		this.mu.Unlock()
		return
	}
	this.closed = true
	this.mu.Unlock()

	if this.hbStarted {
		close(this.done)
		this.hbWG.Wait()
	}
}

// cancelOnDisconnect closes stop when the client's connection goes away, so an
// abandoned search does not keep burning CPU.
func cancelOnDisconnect(c context.Context, done <-chan struct{}, stop chan struct{}) {
	select {
	case <-c.HttpRequest().Context().Done():
	case <-done:
	}
	select {
	case <-stop:
	default:
		close(stop)
	}
}

// countResult is the final line of the count feed.
type countResult struct {
	Event     string `json:"event"`
	Count     int    `json:"count"`
	Complete  bool   `json:"complete"`
	TimedOut  bool   `json:"timedOut"`
	Cancelled bool   `json:"cancelled"`
	Visited   int64  `json:"visited"`
	ElapsedMS int64  `json:"elapsedMs"`
}

// streamProgress is a heartbeat/progress line of the NDJSON feed. It is a
// separate type from streamResult so that zero values (0 ms elapsed, 0
// solutions) survive encoding instead of being dropped by omitempty.
type streamProgress struct {
	Event   string `json:"event"`
	Found   int    `json:"found"`
	Visited int64  `json:"visited"`
}

// streamResult is the final line of the feed.
type streamResult struct {
	Event       string           `json:"event"`
	Solutions   solver.Solutions `json:"solutions"`
	UniqueIndex []int            `json:"uniqueIndex"`
	Found       int              `json:"found"`
	Unique      int              `json:"unique"`
	Exhaustive  bool             `json:"exhaustive"`
	TimedOut    bool             `json:"timedOut"`
	Cancelled   bool             `json:"cancelled"`
	Limited     bool             `json:"limited"`
	Visited     int64            `json:"visited"`
	ElapsedMS   int64            `json:"elapsedMs"`
	Limit       int              `json:"limit"`
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// solveWithin runs the search with a hard wall-clock budget so a slow puzzle
// yields partial results instead of a dropped connection.
