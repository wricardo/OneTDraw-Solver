package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wricardo/OneTDraw-Solver/game/config"
	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/game/session"
	"github.com/wricardo/OneTDraw-Solver/transport/websocket"
)

const triMapJSON = `{
  "id": "tri",
  "name": "Triangle",
  "description": "A closed triangle.",
  "difficulty": "easy",
  "Points": [{"Point":1,"Level":1},{"Point":2,"Level":1},{"Point":3,"Level":1}],
  "Edges": [
    {"PointA":1,"PointB":2},
    {"PointA":2,"PointB":3},
    {"PointA":3,"PointB":1}
  ]
}`

// chainMapJSON is an open trail: it must start at node 1 or 3.
const chainMapJSON = `{
  "id": "chain",
  "name": "Chain",
  "difficulty": "easy",
  "Edges": [
    {"PointA":1,"PointB":2},
    {"PointA":2,"PointB":3}
  ]
}`

func newServer(t *testing.T, extraOptions ...func(*Options)) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tri.json"), []byte(triMapJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chain.json"), []byte(chainMapJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	maps, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	sessions := session.NewManager(nil, 0)
	hub := websocket.NewHub()
	svc := service.New(maps, sessions, hub, service.Options{})
	srv, err := NewServer(svc, hub, Options{
		Introspection: true, Playground: true, UIDirs: []string{dir},
		// The same template main embeds, so the guide's shape is under test.
		LLMSTemplate: readLLMSTemplate(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range extraOptions {
		fn(&srv.opts)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return ts
}

// gql posts one GraphQL document and returns the decoded envelope.
type envelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message    string                 `json:"message"`
		Extensions map[string]interface{} `json:"extensions"`
	} `json:"errors"`
}

func gql(t *testing.T, ts *httptest.Server, query string) envelope {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/graphql", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var env envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatalf("decode %q: %v", string(blob), err)
	}
	return env
}

// gqlAdmin posts one GraphQL document with an X-Admin-Key header.
func gqlAdmin(t *testing.T, ts *httptest.Server, key, query string) envelope {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", ts.URL+"/graphql", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", key)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	blob, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var env envelope
	if err := json.Unmarshal(blob, &env); err != nil {
		t.Fatalf("decode %q: %v", string(blob), err)
	}
	return env
}

// readLLMSTemplate loads the guide template main embeds, so the API tests cover
// the same document the server ships.
func readLLMSTemplate(t *testing.T) string {
	t.Helper()
	blob, err := os.ReadFile(".." + string(filepath.Separator) + "llms.txt.tmpl")
	if err != nil {
		t.Fatalf("read llms.txt.tmpl: %v", err)
	}
	return string(blob)
}

// mustData fails when the operation errored and decodes the data into out.
func mustData(t *testing.T, env envelope, out interface{}) {
	t.Helper()
	if len(env.Errors) > 0 {
		t.Fatalf("graphql errors: %+v", env.Errors)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func TestHealthz(t *testing.T) {
	ts := newServer(t)
	res, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
}

func TestMapsQueryExposesTheGraph(t *testing.T) {
	ts := newServer(t)
	env := gql(t, ts, `{ maps { id name nodeCount edgeCount stepCount parMoves analysis { verdict oddNodes startNodes message } edges { index a b count oneWay } nodes { id x y } } }`)
	var out struct {
		Maps []struct {
			ID       string `json:"id"`
			NodeCnt  int    `json:"nodeCount"`
			EdgeCnt  int    `json:"edgeCount"`
			StepCnt  int    `json:"stepCount"`
			ParMoves *int   `json:"parMoves"`
			Analysis struct {
				Verdict    string `json:"verdict"`
				OddNodes   []int  `json:"oddNodes"`
				StartNodes []int  `json:"startNodes"`
				Message    string `json:"message"`
			} `json:"analysis"`
			Edges []struct {
				Index  int  `json:"index"`
				A      int  `json:"a"`
				B      int  `json:"b"`
				Count  int  `json:"count"`
				OneWay bool `json:"oneWay"`
			} `json:"edges"`
			Nodes []struct {
				ID int `json:"id"`
				X  int `json:"x"`
				Y  int `json:"y"`
			} `json:"nodes"`
		} `json:"maps"`
	}
	mustData(t, env, &out)
	if len(out.Maps) != 2 {
		t.Fatalf("maps = %d, want 2", len(out.Maps))
	}
	// Maps are sorted by id: chain then tri.
	chain, tri := out.Maps[0], out.Maps[1]
	if chain.ID != "chain" || tri.ID != "tri" {
		t.Fatalf("order = %s,%s, want chain,tri", chain.ID, tri.ID)
	}
	if tri.Analysis.Verdict != "CIRCUIT" {
		t.Errorf("tri verdict = %s, want CIRCUIT", tri.Analysis.Verdict)
	}
	if tri.ParMoves == nil || *tri.ParMoves != 3 {
		t.Errorf("tri parMoves = %v, want 3", tri.ParMoves)
	}
	if len(tri.Edges) != 3 || tri.Edges[0].Count != 1 {
		t.Errorf("tri edges = %+v", tri.Edges)
	}
	if len(tri.Nodes) != 3 {
		t.Errorf("tri nodes = %d, want 3", len(tri.Nodes))
	}
	// Every node must carry a layout position, and they must not all coincide:
	// a collapsed layout would draw the triangle as one segment.
	seen := map[string]bool{}
	for _, n := range tri.Nodes {
		seen[jsonNumber(n.X)+","+jsonNumber(n.Y)] = true
	}
	if len(seen) != 3 {
		t.Errorf("tri nodes share layout positions: %+v", tri.Nodes)
	}
	if chain.Analysis.Verdict != "TRAIL" {
		t.Errorf("chain verdict = %s, want TRAIL", chain.Analysis.Verdict)
	}
	if len(chain.Analysis.OddNodes) != 2 {
		t.Errorf("chain odd nodes = %v, want two", chain.Analysis.OddNodes)
	}
}

// jsonNumber renders n into a GraphQL document.
func jsonNumber(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestMapSingle(t *testing.T) {
	ts := newServer(t)
	env := gql(t, ts, `{ map(id: "tri") { id name stepCount } }`)
	var out struct {
		Map *struct {
			ID        string `json:"id"`
			StepCount int    `json:"stepCount"`
		} `json:"map"`
	}
	mustData(t, env, &out)
	if out.Map == nil || out.Map.StepCount != 3 {
		t.Fatalf("map = %+v", out.Map)
	}

	// An unknown map is null, not an error, so a client can tell "missing".
	env = gql(t, ts, `{ map(id: "nope") { id } }`)
	var missing struct {
		Map *struct {
			ID string `json:"id"`
		} `json:"map"`
	}
	mustData(t, env, &missing)
	if missing.Map != nil {
		t.Errorf("unknown map returned %+v, want null", missing.Map)
	}
}

// createSession starts a game and returns its id.
func createSession(t *testing.T, ts *httptest.Server, mapID string) string {
	t.Helper()
	env := gql(t, ts, `mutation { createSession(mapID: "`+mapID+`") { id } }`)
	var out struct {
		CreateSession struct {
			ID string `json:"id"`
		} `json:"createSession"`
	}
	mustData(t, env, &out)
	if out.CreateSession.ID == "" {
		t.Fatal("createSession returned no id")
	}
	return out.CreateSession.ID
}

func TestPlayThroughGraphQL(t *testing.T) {
	ts := newServer(t)
	id := createSession(t, ts, "tri")

	// Start at node 1, then draw 1-2, 2-3, 3-1.
	env := gql(t, ts, `mutation { start(sessionID: "`+id+`", node: 1) { state { pos status moves } lastEvent { kind blocked } } }`)
	var started struct {
		Start struct {
			State struct {
				Pos    *int   `json:"pos"`
				Status string `json:"status"`
				Moves  int    `json:"moves"`
			} `json:"state"`
			LastEvent struct {
				Kind string `json:"kind"`
			} `json:"lastEvent"`
		} `json:"start"`
	}
	mustData(t, env, &started)
	if started.Start.State.Pos == nil || *started.Start.State.Pos != 1 {
		t.Fatalf("pos = %v, want 1", started.Start.State.Pos)
	}
	if started.Start.LastEvent.Kind != "START" {
		t.Errorf("kind = %s, want START", started.Start.LastEvent.Kind)
	}

	env = gql(t, ts, `mutation { bulkMove(sessionID: "`+id+`", nodes: [2, 3, 1]) { state { pos status moves remaining } lastEvent { blocked } attempts bestMoves } }`)
	var moved struct {
		BulkMove struct {
			State struct {
				Pos       *int   `json:"pos"`
				Status    string `json:"status"`
				Moves     int    `json:"moves"`
				Remaining int    `json:"remaining"`
			} `json:"state"`
			Attempts  int  `json:"attempts"`
			BestMoves *int `json:"bestMoves"`
		} `json:"bulkMove"`
	}
	mustData(t, env, &moved)
	if moved.BulkMove.State.Status != "WON" {
		t.Fatalf("status = %s, want WON", moved.BulkMove.State.Status)
	}
	if moved.BulkMove.State.Remaining != 0 {
		t.Errorf("remaining = %d, want 0", moved.BulkMove.State.Remaining)
	}
	if moved.BulkMove.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", moved.BulkMove.Attempts)
	}
}

func TestBlockedMoveIsNotAnError(t *testing.T) {
	ts := newServer(t)
	id := createSession(t, ts, "chain")
	gql(t, ts, `mutation { start(sessionID: "`+id+`", node: 1) { state { pos } } }`)

	env := gql(t, ts, `mutation { move(sessionID: "`+id+`", node: 9) { state { moves } lastEvent { blocked reason } } }`)
	var out struct {
		Move struct {
			State struct {
				Moves int `json:"moves"`
			} `json:"state"`
			LastEvent struct {
				Blocked bool   `json:"blocked"`
				Reason  string `json:"reason"`
			} `json:"lastEvent"`
		} `json:"move"`
	}
	mustData(t, env, &out)
	if !out.Move.LastEvent.Blocked {
		t.Fatal("expected a blocked step")
	}
	if out.Move.LastEvent.Reason == "" {
		t.Error("a blocked step should explain itself")
	}
	if out.Move.State.Moves != 0 {
		t.Errorf("moves = %d, want 0", out.Move.State.Moves)
	}
}

func TestMoveBeforeStartReturnsCode(t *testing.T) {
	ts := newServer(t)
	id := createSession(t, ts, "tri")
	env := gql(t, ts, `mutation { move(sessionID: "`+id+`", node: 2) { state { moves } } }`)
	if len(env.Errors) == 0 {
		t.Fatal("expected an error")
	}
	code, _ := env.Errors[0].Extensions["code"].(string)
	if code != "INVALID_ARGUMENT" {
		t.Errorf("code = %q, want INVALID_ARGUMENT", code)
	}
}

func TestUnknownSessionReturnsNotFoundCode(t *testing.T) {
	ts := newServer(t)
	env := gql(t, ts, `mutation { reset(sessionID: "missing") { id } }`)
	if len(env.Errors) == 0 {
		t.Fatal("expected an error")
	}
	code, _ := env.Errors[0].Extensions["code"].(string)
	if code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

func TestSessionTerminalCode(t *testing.T) {
	ts := newServer(t)
	id := createSession(t, ts, "tri")
	gql(t, ts, `mutation { start(sessionID: "`+id+`", node: 1) { state { pos } } }`)
	gql(t, ts, `mutation { bulkMove(sessionID: "`+id+`", nodes: [2, 3, 1]) { id } }`)

	env := gql(t, ts, `mutation { move(sessionID: "`+id+`", node: 2) { state { moves } } }`)
	if len(env.Errors) == 0 {
		t.Fatal("moving after a win should be an error")
	}
	code, _ := env.Errors[0].Extensions["code"].(string)
	if code != "SESSION_TERMINAL" {
		t.Errorf("code = %q, want SESSION_TERMINAL", code)
	}

	// Reset makes it playable again.
	var out struct {
		Reset struct {
			State struct {
				Status string `json:"status"`
			} `json:"state"`
		} `json:"reset"`
	}
	mustData(t, gql(t, ts, `mutation { reset(sessionID: "`+id+`") { state { status } } }`), &out)
	if out.Reset.State.Status != "READY" {
		t.Errorf("status after reset = %s, want READY", out.Reset.State.Status)
	}
}

func TestSolveAndHint(t *testing.T) {
	ts := newServer(t)
	id := createSession(t, ts, "tri")

	var hint struct {
		Hint struct {
			Start int   `json:"start"`
			Steps []int `json:"steps"`
			Found bool  `json:"found"`
		} `json:"hint"`
	}
	mustData(t, gql(t, ts, `query { hint(sessionID: "`+id+`") { start steps found } }`), &hint)
	if !hint.Hint.Found || len(hint.Hint.Steps) != 3 {
		t.Fatalf("hint = %+v", hint.Hint)
	}

	var solved struct {
		Solve struct {
			Session struct {
				LastPlan *struct {
					Start int   `json:"start"`
					Found bool  `json:"found"`
					Steps []int `json:"steps"`
				} `json:"lastPlan"`
			} `json:"session"`
		} `json:"solve"`
	}
	_ = solved
	env := gql(t, ts, `mutation { solve(sessionID: "`+id+`") { lastPlan { start steps found } } }`)
	var out struct {
		Solve struct {
			LastPlan struct {
				Start int   `json:"start"`
				Steps []int `json:"steps"`
				Found bool  `json:"found"`
			} `json:"lastPlan"`
		} `json:"solve"`
	}
	mustData(t, env, &out)
	if !out.Solve.LastPlan.Found || len(out.Solve.LastPlan.Steps) != 3 {
		t.Fatalf("plan = %+v", out.Solve.LastPlan)
	}

	// Playing the plan wins.
	gql(t, ts, `mutation { start(sessionID: "`+id+`", node: `+jsonNumber(out.Solve.LastPlan.Start)+`) { state { pos } } }`)
	steps, _ := json.Marshal(out.Solve.LastPlan.Steps)
	var done struct {
		BulkMove struct {
			State struct {
				Status string `json:"status"`
			} `json:"state"`
		} `json:"bulkMove"`
	}
	mustData(t, gql(t, ts, `mutation { bulkMove(sessionID: "`+id+`", nodes: `+string(steps)+`) { state { status } } }`), &done)
	if done.BulkMove.State.Status != "WON" {
		t.Errorf("status after playing the plan = %s, want WON", done.BulkMove.State.Status)
	}
}

func TestSessionsListAndDelete(t *testing.T) {
	ts := newServer(t)
	a := createSession(t, ts, "tri")
	b := createSession(t, ts, "chain")

	var list struct {
		Sessions []struct {
			ID    string `json:"id"`
			MapID string `json:"mapId"`
		} `json:"sessions"`
	}
	mustData(t, gql(t, ts, `{ sessions(limit: 10) { id mapId } }`), &list)
	if len(list.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(list.Sessions))
	}

	var del struct {
		DeleteSession bool `json:"deleteSession"`
	}
	mustData(t, gql(t, ts, `mutation { deleteSession(id: "`+a+`") }`), &del)
	if !del.DeleteSession {
		t.Error("deleteSession returned false")
	}

	mustData(t, gql(t, ts, `{ sessions(limit: 10) { id } }`), &list)
	if len(list.Sessions) != 1 || list.Sessions[0].ID != b {
		t.Errorf("after delete sessions = %+v, want just b", list.Sessions)
	}

	// Deleting again reports false rather than an error.
	mustData(t, gql(t, ts, `mutation { deleteSession(id: "`+a+`") }`), &del)
	if del.DeleteSession {
		t.Error("deleting a missing session returned true")
	}
}

func TestAdminGate(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "secret")
	t.Setenv("ALLOW_UNAUTHENTICATED_ADMIN", "")

	ts := newServer(t)
	// No key: refused with FORBIDDEN.
	env := gql(t, ts, `{ adminMap(id: "tri") { id } }`)
	if len(env.Errors) == 0 {
		t.Fatal("adminMap without a key should be refused")
	}
	code, _ := env.Errors[0].Extensions["code"].(string)
	if code != "FORBIDDEN" {
		t.Errorf("code = %q, want FORBIDDEN", code)
	}

	// With the right key the full definition comes back.
	body, _ := json.Marshal(map[string]string{"query": `{ adminMap(id: "tri") { id config { points { point level } edges { pointA pointB count } } } }`})
	req, _ := http.NewRequest("POST", ts.URL+"/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Key", "secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		AdminMap *struct {
			ID     string `json:"id"`
			Config struct {
				Points []struct {
					Point int `json:"point"`
				} `json:"points"`
				Edges []struct {
					PointA int `json:"pointA"`
					Count  int `json:"count"`
				} `json:"edges"`
			} `json:"config"`
		} `json:"adminMap"`
	}
	blob, _ := io.ReadAll(res.Body)
	var envBody struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(blob, &envBody); err != nil {
		t.Fatalf("decode: %v (%s)", err, string(blob))
	}
	if err := json.Unmarshal(envBody.Data, &out); err != nil {
		t.Fatal(err)
	}
	if out.AdminMap == nil || len(out.AdminMap.Config.Edges) != 3 {
		t.Fatalf("adminMap = %+v", out.AdminMap)
	}
}

func TestMapValidationThroughGraphQL(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "secret")
	ts := newServer(t)

	// An invalid map reports issues rather than failing the request. The mutation
	// is admin-gated, so it needs the key.
	env := gqlAdmin(t, ts, "secret", `mutation { validateMap(map: { id: "bad id", name: "X", edges: [{pointA: 1, pointB: 1}] }) { valid issues { message } solvable } }`)
	var out struct {
		ValidateMap struct {
			Valid  bool `json:"valid"`
			Issues []struct {
				Message string `json:"message"`
			} `json:"issues"`
			Solvable bool `json:"solvable"`
		} `json:"validateMap"`
	}
	mustData(t, env, &out)
	if out.ValidateMap.Valid {
		t.Fatal("an invalid map was reported valid")
	}
	if len(out.ValidateMap.Issues) == 0 {
		t.Error("expected issues to be listed")
	}
}

func TestLLMSGuideRenders(t *testing.T) {
	ts := newServer(t)
	res, err := http.Get(ts.URL + "/llms.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	blob, _ := io.ReadAll(res.Body)
	text := string(blob)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if !strings.Contains(text, "/graphql") {
		t.Error("the guide does not mention the GraphQL endpoint")
	}
	// The base URL must be the host the client actually used.
	if !strings.Contains(text, ts.URL) {
		t.Errorf("the guide does not carry the request host %s", ts.URL)
	}
}

func TestSPAFallback(t *testing.T) {
	ts := newServer(t)
	// A client route without an index.html still needs the 200 shell; an unknown
	// path must report 404 so crawlers see a real status.
	res, err := http.Get(ts.URL + "/play/abc")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 && res.StatusCode != 200 {
		t.Errorf("unexpected status %d", res.StatusCode)
	}
}

func TestBodyCap(t *testing.T) {
	ts := newServer(t)
	huge := strings.Repeat("a", MaxRequestBodyBytes+1024)
	res, err := http.Post(ts.URL+"/graphql", "application/json", strings.NewReader(huge))
	if err != nil {
		// A server that closes the connection early is also an acceptable
		// rejection of the oversized body.
		return
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		blob, _ := io.ReadAll(res.Body)
		// A 200 is only acceptable if the payload was refused as a GraphQL error.
		if !strings.Contains(string(blob), "error") && !strings.Contains(string(blob), "too large") {
			t.Errorf("an oversized body was accepted: %s", string(blob))
		}
	}
}
