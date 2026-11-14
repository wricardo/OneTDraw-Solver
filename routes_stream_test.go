package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/goweb"
)

// newTestServer starts the real legacy route table on a test server, so these
// tests exercise the same handlers the browser talks to. The default handler is
// reset first so each test gets a clean route table rather than accumulating
// maps, then legacyHandler installs the routes.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	goweb.SetDefaultHttpHandler(nil)
	srv := httptest.NewServer(legacyHandler())
	t.Cleanup(srv.Close)
	return srv
}

func TestPuzzlesRouteIncludesAnalysis(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/puzzles")
	if err != nil {
		t.Fatalf("get /puzzles: %v", err)
	}
	defer res.Body.Close()

	var puzzles []puzzleSummary
	if err := json.NewDecoder(res.Body).Decode(&puzzles); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(puzzles) == 0 {
		t.Fatal("expected puzzles")
	}
	for _, p := range puzzles {
		if p.JsonFile == "square.json" {
			continue // legacy format, has no analyseable puzzle body
		}
		if p.StepCount == 0 {
			t.Errorf("%s: expected a step count, got 0", p.Name)
		}
		if p.Verdict == "" {
			t.Errorf("%s: expected a verdict", p.Name)
		}
	}
}

// The stream must deliver a result line even for a large puzzle, rather than a
// truncated body or an error, and must say whether the search was exhaustive.
func TestStreamRouteReturnsResult(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/puzzle/stream/house?limit=20000&budgetMs=10000")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer res.Body.Close()

	if got := res.Header.Get("Content-Type"); !strings.Contains(got, "ndjson") {
		t.Errorf("expected ndjson content type, got %q", got)
	}

	var result *streamResult
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for scanner.Scan() {
		var probe struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &probe); err != nil {
			t.Fatalf("bad line %q: %v", scanner.Text(), err)
		}
		if probe.Event == "result" {
			var r streamResult
			if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			result = &r
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if result == nil {
		t.Fatal("stream produced no result line")
	}
	if result.Found != 88 {
		t.Errorf("expected 88 walks, got %d", result.Found)
	}
	if result.Unique != 44 {
		t.Errorf("expected 44 distinct routes, got %d", result.Unique)
	}
	if !result.Exhaustive {
		t.Error("expected the search to be exhaustive")
	}
	if len(result.UniqueIndex) != result.Unique {
		t.Errorf("uniqueIndex length %d does not match unique %d", len(result.UniqueIndex), result.Unique)
	}
}

// A stop request must end the search and still deliver what was found, which is
// the whole point of the session handshake.
func TestStopRouteEndsStreamAndKeepsResults(t *testing.T) {
	srv := newTestServer(t)

	// No limit and no budget: only the stop request can end this.
	res, err := http.Get(srv.URL + "/puzzle/stream/level54?limit=0&budgetMs=0&session=test-session")
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	defer res.Body.Close()

	stopped := make(chan struct{})
	go func() {
		time.Sleep(500 * time.Millisecond)
		resp, err := http.Post(srv.URL+"/puzzle/stop/test-session", "text/plain", nil)
		if err == nil {
			resp.Body.Close()
		}
		close(stopped)
	}()

	done := make(chan *streamResult, 1)
	go func() {
		var result *streamResult
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 0, 1<<20), 32<<20)
		for scanner.Scan() {
			var probe struct {
				Event string `json:"event"`
			}
			if json.Unmarshal(scanner.Bytes(), &probe) != nil {
				continue
			}
			if probe.Event == "result" {
				var r streamResult
				if json.Unmarshal(scanner.Bytes(), &r) == nil {
					result = &r
				}
			}
		}
		done <- result
	}()

	select {
	case result := <-done:
		if result == nil {
			t.Fatal("no result line after stopping")
		}
		if !result.Cancelled {
			t.Error("expected the result to report cancellation")
		}
		if result.Found == 0 {
			t.Error("expected the partial results to be kept")
		}
		if result.Exhaustive {
			t.Error("a stopped search cannot be exhaustive")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("stream did not finish after a stop request")
	}
	<-stopped
}

func TestStopRouteUnknownSession(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Post(srv.URL+"/puzzle/stop/nope", "text/plain", nil)
	if err != nil {
		t.Fatalf("post stop: %v", err)
	}
	defer res.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["stopped"] != false {
		t.Errorf("expected stopped=false for an unknown session, got %v", body["stopped"])
	}
}

// A budgeted count of a puzzle too large to finish must say the number is a
// lower bound rather than presenting it as the total.
func TestCountRouteReportsIncomplete(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/puzzle/count/level54?budgetMs=300")
	if err != nil {
		t.Fatalf("get count: %v", err)
	}
	defer res.Body.Close()

	var result countResult
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	found := false
	for scanner.Scan() {
		var probe struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(scanner.Bytes(), &probe) != nil {
			continue
		}
		if probe.Event == "result" {
			if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("count stream produced no result line")
	}
	if result.Complete {
		t.Error("expected an incomplete count")
	}
	if result.Count == 0 {
		t.Error("expected a partial count")
	}
}

func TestCountRouteComplete(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/puzzle/count/level57?budgetMs=30000")
	if err != nil {
		t.Fatalf("get count: %v", err)
	}
	defer res.Body.Close()

	var result countResult
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for scanner.Scan() {
		var probe struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(scanner.Bytes(), &probe) != nil {
			continue
		}
		if probe.Event == "result" {
			if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
				t.Fatalf("decode result: %v", err)
			}
		}
	}
	if !result.Complete {
		t.Error("expected a complete count")
	}
	if result.Count != 90624 {
		t.Errorf("expected 90624, got %d", result.Count)
	}
}

func TestPuzzleDataRoute(t *testing.T) {
	srv := newTestServer(t)
	res, err := http.Get(srv.URL + "/puzzle/data/level53")
	if err != nil {
		t.Fatalf("get data: %v", err)
	}
	defer res.Body.Close()

	var data puzzleData
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.File != "level53" {
		t.Errorf("expected file level53, got %q", data.File)
	}
	if len(data.Edges) == 0 {
		t.Error("expected edges")
	}
	if len(data.Points) == 0 {
		t.Error("expected points")
	}
	if data.Analysis.StepCount == 0 {
		t.Error("expected a step count")
	}
}

// Puzzles that omit Points entirely, and the legacy list-only format, must be
// rejected or handled without panicking.
func TestRoutesRejectBadInput(t *testing.T) {
	srv := newTestServer(t)

	cases := []string{
		"/puzzle/data/nope",
		"/puzzle/stream/nope",
		"/puzzle/count/nope",
	}
	for _, path := range cases {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: expected 400 or 404, got %d", path, res.StatusCode)
		}
	}
}

// A traversal attempt must never return file contents. The status code is not
// the point (the router may normalise the path before the handler sees it) —
// leaking a system file is.
func TestRoutesDoNotLeakFiles(t *testing.T) {
	srv := newTestServer(t)

	cases := []string{
		"/puzzle/data/..%2F..%2Fetc%2Fpasswd",
		"/puzzle/data/%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"/puzzle/stream/..%2F..%2Fetc%2Fpasswd",
		"/puzzle/data/..%2f..%2fgo.mod",
	}
	for _, path := range cases {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		for _, leak := range []string{"root:", "module github.com"} {
			if strings.Contains(string(body), leak) {
				t.Fatalf("%s leaked file contents: %q", path, string(body)[:min(200, len(body))])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// The solver's own UI must stay reachable and must not live under static/, which
// `npm run build:static` wipes. This is a regression guard: the frontend build
// deleted static/index.html once, taking the original UI with it.
func TestLegacySolverUIIsServed(t *testing.T) {
	srv := newTestServer(t)
	for _, path := range []string{"/legacy-static/index.html", "/legacy-static/ui2.html"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		blob, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s status = %d, want 200", path, res.StatusCode)
			continue
		}
		if len(blob) < 1000 {
			t.Errorf("%s is suspiciously small (%d bytes)", path, len(blob))
		}
	}

	// static/ belongs to the SvelteKit build, so its index.html must be the SPA
	// shell, not a copy of the original solver page. (The build wipes static/,
	// which is exactly why the legacy UI was moved out of it.)
	blob, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("read static/index.html: %v", err)
	}
	if strings.Contains(string(blob), "puzzle/stream") {
		t.Error("static/index.html is the original solver page, not the SPA shell")
	}
	legacy, err := os.ReadFile("legacy-static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(legacy), "puzzle/stream") {
		t.Error("legacy-static/index.html is not the original solver UI")
	}
}
