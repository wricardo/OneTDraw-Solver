package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

// ---- fakes ----------------------------------------------------------------

// fakeMaps is an in-memory MapStore.
type fakeMaps struct {
	mu   sync.Mutex
	maps map[string]*engine.Map
}

func newFakeMaps(cfgs ...engine.MapConfig) *fakeMaps {
	f := &fakeMaps{maps: map[string]*engine.Map{}}
	for _, cfg := range cfgs {
		m, err := engine.NewMap(cfg)
		if err != nil {
			panic(err)
		}
		f.maps[m.ID] = m
	}
	return f
}

func (f *fakeMaps) List() []*engine.Map {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*engine.Map, 0, len(f.maps))
	for _, m := range f.maps {
		out = append(out, m.Clone())
	}
	return out
}

func (f *fakeMaps) Get(id string) (*engine.Map, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.maps[id]
	if !ok {
		return nil, Errorf(CodeNotFound, "map %q not found", id)
	}
	return m.Clone(), nil
}

func (f *fakeMaps) Save(cfg engine.MapConfig) (*engine.Map, error) {
	m, err := engine.NewMap(cfg)
	if err != nil {
		return nil, Errorf(CodeInvalidArgument, "%v", err)
	}
	f.mu.Lock()
	f.maps[m.ID] = m
	f.mu.Unlock()
	return m.Clone(), nil
}

func (f *fakeMaps) Create(cfg engine.MapConfig) (*engine.Map, error) {
	f.mu.Lock()
	_, exists := f.maps[cfg.ID]
	f.mu.Unlock()
	if exists {
		return nil, Errorf(CodeInvalidArgument, "map %q already exists", cfg.ID)
	}
	return f.Save(cfg)
}

func (f *fakeMaps) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.maps[id]; !ok {
		return Errorf(CodeNotFound, "map %q not found", id)
	}
	delete(f.maps, id)
	return nil
}

// fakeSessions is an in-memory SessionStore that records persist calls.
type fakeSessions struct {
	mu       sync.Mutex
	sessions map[string]*Session
	persists int
	max      int
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{sessions: map[string]*Session{}, max: 100}
}

func (f *fakeSessions) Put(s *Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.sessions[s.ID]; ok {
		return errors.New("exists")
	}
	if len(f.sessions) >= f.max {
		return Errorf(CodeLimit, "limit")
	}
	f.sessions[s.ID] = s
	return nil
}

func (f *fakeSessions) Get(id string) (*Session, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[id]
	return s, ok
}

func (f *fakeSessions) List() []*Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*Session, 0, len(f.sessions))
	for _, s := range f.sessions {
		out = append(out, s)
	}
	return out
}

func (f *fakeSessions) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.sessions[id]; !ok {
		return Errorf(CodeNotFound, "session %q not found", id)
	}
	delete(f.sessions, id)
	return nil
}

func (f *fakeSessions) Persist(s *Session) error {
	f.mu.Lock()
	f.persists++
	f.mu.Unlock()
	return nil
}

// recorder captures broadcasts.
type recorder struct {
	mu    sync.Mutex
	snaps []*Session
}

func (r *recorder) Broadcast(s *Session) {
	r.mu.Lock()
	r.snaps = append(r.snaps, s)
	r.mu.Unlock()
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.snaps)
}

func (r *recorder) last() *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.snaps) == 0 {
		return nil
	}
	return r.snaps[len(r.snaps)-1]
}

// ---- fixtures -------------------------------------------------------------

// triMap is a closed triangle: any node may start and three moves win.
func triMap() engine.MapConfig {
	return engine.MapConfig{
		ID: "tri", Name: "Triangle",
		Points: []engine.PointConfig{
			engine.NewPointConfig(1, 1, false), engine.NewPointConfig(2, 1, false), engine.NewPointConfig(3, 1, false),
		},
		Edges: []engine.EdgeConfig{
			{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}, {PointA: 3, PointB: 1},
		},
	}
}

// chainMap is an open path 1-2-3: it must start at 1 or 3.
func chainMap() engine.MapConfig {
	return engine.MapConfig{
		ID: "chain", Name: "Chain",
		Edges: []engine.EdgeConfig{{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}},
	}
}

func newEnv(t *testing.T) (GameService, *fakeSessions, *recorder) {
	t.Helper()
	sessions := newFakeSessions()
	rec := &recorder{}
	svc := New(newFakeMaps(triMap(), chainMap()), sessions, rec, Options{})
	return svc, sessions, rec
}

// ---- session lifecycle ----------------------------------------------------

func TestCreateSession(t *testing.T) {
	svc, _, rec := newEnv(t)
	s, err := svc.CreateSession("tri", "  Ada  ")
	if err != nil {
		t.Fatal(err)
	}
	if s.DisplayName != "Ada" {
		t.Errorf("display name = %q, want it trimmed to %q", s.DisplayName, "Ada")
	}
	if s.State.Status != engine.StatusReady {
		t.Errorf("status = %s, want READY", s.State.Status)
	}
	if s.State.Pos != -1 {
		t.Errorf("pos = %d, want -1 before starting", s.State.Pos)
	}
	if s.Seq != 1 {
		t.Errorf("seq = %d, want 1", s.Seq)
	}
	if s.Map.ID != "tri" {
		t.Errorf("map id = %q, want tri", s.Map.ID)
	}
	if rec.count() != 1 {
		t.Errorf("broadcasts = %d, want 1 on create", rec.count())
	}
}

func TestCreateSessionUnknownMap(t *testing.T) {
	svc, _, _ := newEnv(t)
	_, err := svc.CreateSession("nope", "")
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeNotFound {
		t.Fatalf("err = %v, want a NOT_FOUND service error", err)
	}
}

func TestCreateSessionRejectsLongName(t *testing.T) {
	svc, _, _ := newEnv(t)
	long := make([]byte, DefaultMaxDisplayName+1)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.CreateSession("tri", string(long))
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeInvalidArgument {
		t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
	}
}

func TestSessionSnapshotIsIsolatedFromTheMap(t *testing.T) {
	// Editing a map must never disturb a game already playing it.
	maps := newFakeMaps(triMap())
	svc := New(maps, newFakeSessions(), nil, Options{})
	s, err := svc.CreateSession("tri", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maps.Save(engine.MapConfig{
		ID: "tri", Name: "Changed",
		Edges: []engine.EdgeConfig{{PointA: 1, PointB: 2}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Map.Name != "Triangle" || len(got.Map.Edges) != 3 {
		t.Errorf("session map changed under it: name=%q edges=%d", got.Map.Name, len(got.Map.Edges))
	}
}

// ---- moving ---------------------------------------------------------------

func TestStartThenWin(t *testing.T) {
	svc, _, rec := newEnv(t)
	s, _ := svc.CreateSession("tri", "")

	s, err := svc.Start(s.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.State.Pos != 1 || s.State.Moves != 0 {
		t.Fatalf("after start: pos=%d moves=%d", s.State.Pos, s.State.Moves)
	}

	for _, node := range []int{2, 3, 1} {
		s, err = svc.Move(s.ID, node)
		if err != nil {
			t.Fatalf("move %d: %v", node, err)
		}
	}
	if s.State.Status != engine.StatusWon {
		t.Fatalf("status = %s, want WON", s.State.Status)
	}
	if s.State.Moves != 3 {
		t.Errorf("moves = %d, want 3", s.State.Moves)
	}
	if s.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 after a finished game", s.Attempts)
	}
	if s.BestMoves == nil || *s.BestMoves != 3 {
		t.Errorf("bestMoves = %v, want 3", s.BestMoves)
	}
	if got := rec.last().Seq; got != s.Seq {
		t.Errorf("last broadcast seq = %d, want %d", got, s.Seq)
	}
}

func TestMoveAfterWinIsRejected(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	svc.Start(s.ID, 1)
	for _, n := range []int{2, 3, 1} {
		if _, err := svc.Move(s.ID, n); err != nil {
			t.Fatal(err)
		}
	}
	_, err := svc.Move(s.ID, 2)
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeSessionTerminal {
		t.Fatalf("err = %v, want SESSION_TERMINAL", err)
	}
}

func TestBlockedMoveBumpsSeqButDoesNotPersist(t *testing.T) {
	svc, sessions, rec := newEnv(t)
	s, _ := svc.CreateSession("chain", "")
	svc.Start(s.ID, 2)
	before := rec.last().Seq
	persistsBefore := sessions.persists

	// From node 2, node 4 does not exist.
	got, err := svc.Move(s.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastEvent.Blocked {
		t.Fatal("expected the move to be blocked")
	}
	if got.LastEvent.Reason == "" {
		t.Error("a blocked move should carry a reason")
	}
	if got.Seq <= before {
		t.Errorf("seq = %d, want it above %d so watchers see the bump", got.Seq, before)
	}
	if got.State.Moves != 0 {
		t.Errorf("moves = %d, want 0 for a blocked attempt", got.State.Moves)
	}
	if sessions.persists != persistsBefore {
		t.Error("a blocked move was persisted; it changed nothing")
	}
}

func TestMoveBeforeStartIsInvalid(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	_, err := svc.Move(s.ID, 2)
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeInvalidArgument {
		t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
	}
}

func TestDeadEndIsRecordedAsAnAttempt(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("chain", "")
	svc.Start(s.ID, 2)
	s, err := svc.Move(s.ID, 1) // strands 2-3
	if err != nil {
		t.Fatal(err)
	}
	if s.State.Status != engine.StatusLost {
		t.Fatalf("status = %s, want LOST", s.State.Status)
	}
	if s.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 for a lost game", s.Attempts)
	}
	if s.BestMoves != nil {
		t.Errorf("bestMoves = %v, want nil: no game has been won", *s.BestMoves)
	}
}

// ---- bulk move ------------------------------------------------------------

func TestBulkMoveStopsOnBlocked(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	svc.Start(s.ID, 1)

	// 2 is joined, 9 is not: the third node must not be attempted.
	got, err := svc.BulkMove(s.ID, []int{2, 9, 3}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastEvent.Blocked {
		t.Fatal("expected the bulk move to stop at the blocked step")
	}
	if got.State.Moves != 1 {
		t.Errorf("moves = %d, want 1: only 1->2 was drawn", got.State.Moves)
	}
	if got.State.Pos != 2 {
		t.Errorf("pos = %d, want 2", got.State.Pos)
	}
}

func TestBulkMoveSkipsBlockedWhenAsked(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	svc.Start(s.ID, 1)

	got, err := svc.BulkMove(s.ID, []int{2, 9, 3, 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Status != engine.StatusWon {
		t.Fatalf("status = %s, want WON: the blocked 9 should have been skipped", got.State.Status)
	}
	if got.State.Moves != 3 {
		t.Errorf("moves = %d, want 3", got.State.Moves)
	}
}

func TestBulkMoveStopsWhenGameEnds(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	svc.Start(s.ID, 1)

	// The win happens on the third step; the trailing moves must be ignored.
	got, err := svc.BulkMove(s.ID, []int{2, 3, 1, 2, 3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Status != engine.StatusWon {
		t.Fatalf("status = %s, want WON", got.State.Status)
	}
	if got.State.Moves != 3 {
		t.Errorf("moves = %d, want 3: moves after the win must not run", got.State.Moves)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
}

func TestBulkMoveRejectsTooManySteps(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	nodes := make([]int, MaxSteps+1)
	_, err := svc.BulkMove(s.ID, nodes, true)
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeInvalidArgument {
		t.Fatalf("err = %v, want INVALID_ARGUMENT for an oversized bulk move", err)
	}
}

func TestBulkMoveEmptyIsANoOp(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	got, err := svc.BulkMove(s.ID, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Status != engine.StatusReady {
		t.Errorf("status = %s, want READY", got.State.Status)
	}
}

// ---- undo/reset -----------------------------------------------------------

func TestUndo(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	if _, err := svc.Undo(s.ID); err == nil {
		t.Fatal("undo on an unstarted trail should fail")
	}
	svc.Start(s.ID, 1)
	svc.Move(s.ID, 2)
	got, err := svc.Undo(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Moves != 0 || got.State.Pos != 1 {
		t.Errorf("after undo: moves=%d pos=%d, want 0/1", got.State.Moves, got.State.Pos)
	}
	if got.State.Remaining != 3 {
		t.Errorf("remaining = %d, want 3", got.State.Remaining)
	}
}

func TestResetKeepsHistory(t *testing.T) {
	svc, _, _ := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	svc.Start(s.ID, 1)
	for _, n := range []int{2, 3, 1} {
		s, _ = svc.Move(s.ID, n)
	}
	if s.Attempts != 1 || s.BestMoves == nil {
		t.Fatalf("expected one win recorded, got attempts=%d best=%v", s.Attempts, s.BestMoves)
	}

	got, err := svc.Reset(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Status != engine.StatusReady || got.State.Pos != -1 {
		t.Errorf("after reset: status=%s pos=%d, want READY/-1", got.State.Status, got.State.Pos)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 kept", got.Attempts)
	}
	if got.BestMoves == nil || *got.BestMoves != 3 {
		t.Errorf("bestMoves = %v, want 3 kept", got.BestMoves)
	}
	if got.State.Remaining != 3 {
		t.Errorf("remaining = %d, want the full 3 again", got.State.Remaining)
	}
}

// ---- solver ---------------------------------------------------------------

func TestSolveAttachesAPlan(t *testing.T) {
	svc, _, rec := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	before := rec.count()

	got, err := svc.Solve(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastPlan == nil || !got.LastPlan.Found {
		t.Fatalf("lastPlan = %+v, want a found plan", got.LastPlan)
	}
	if len(got.LastPlan.Steps) != 3 {
		t.Errorf("plan steps = %d, want 3", len(got.LastPlan.Steps))
	}
	if rec.count() != before+1 {
		t.Error("attaching a plan should broadcast once")
	}
	// The plan must actually win when played.
	svc.Start(s.ID, got.LastPlan.Start)
	done, err := svc.BulkMove(s.ID, got.LastPlan.Steps, true)
	if err != nil {
		t.Fatal(err)
	}
	if done.State.Status != engine.StatusWon {
		t.Fatalf("playing the plan ended in %s, want WON", done.State.Status)
	}
}

func TestSolveOnAnImpossibleMapFails(t *testing.T) {
	svc := New(newFakeMaps(engine.MapConfig{
		ID: "star", Name: "Star",
		Edges: []engine.EdgeConfig{
			{PointA: 1, PointB: 2}, {PointA: 1, PointB: 3}, {PointA: 1, PointB: 4}, {PointA: 1, PointB: 5},
		},
	}), newFakeSessions(), nil, Options{})
	s, err := svc.CreateSession("star", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Solve(s.ID)
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeInvalidArgument {
		t.Fatalf("err = %v, want INVALID_ARGUMENT when no plan exists", err)
	}
}

func TestHintDoesNotChangeTheGame(t *testing.T) {
	svc, _, rec := newEnv(t)
	s, _ := svc.CreateSession("tri", "")
	before := rec.count()

	plan, err := svc.Hint(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Found {
		t.Fatal("hint found nothing on a solvable map")
	}
	if rec.count() != before {
		t.Error("hint broadcast a change; it must be read-only")
	}
	after, err := svc.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Seq != s.Seq {
		t.Error("hint changed the session")
	}
}

// ---- listing --------------------------------------------------------------

func TestListSessionsSortAndLimit(t *testing.T) {
	svc, sessions, _ := newEnv(t)
	a, _ := svc.CreateSession("tri", "A")
	b, _ := svc.CreateSession("tri", "B")
	// Make B clearly the most recent.
	sessions.sessions[a.ID].LastActionAt = a.CreatedAt.Add(-time.Hour)

	recent, err := svc.ListSessions(SessionFilter{Sort: "recent", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].ID != b.ID {
		t.Fatalf("recent list = %+v, want just B", recent)
	}

	all, err := svc.ListSessions(SessionFilter{Sort: "created"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("created list has %d sessions, want 2", len(all))
	}

	if _, err := svc.ListSessions(SessionFilter{Sort: "bogus"}); err == nil {
		t.Error("an unknown sort should be rejected")
	}
	if _, err := svc.ListSessions(SessionFilter{Limit: -1}); err == nil {
		t.Error("a negative limit should be rejected")
	}
}

func TestListSessionsFiltersByMap(t *testing.T) {
	svc, _, _ := newEnv(t)
	if _, err := svc.CreateSession("tri", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateSession("chain", ""); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListSessions(SessionFilter{MapID: "chain"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MapID != "chain" {
		t.Fatalf("filtered list = %+v, want one chain session", got)
	}
}

func TestSessionLimit(t *testing.T) {
	sessions := newFakeSessions()
	sessions.max = 1
	svc := New(newFakeMaps(triMap()), sessions, nil, Options{})
	if _, err := svc.CreateSession("tri", ""); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateSession("tri", "")
	var se *Error
	if !errors.As(err, &se) || se.Code != CodeLimit {
		t.Fatalf("err = %v, want LIMIT_REACHED", err)
	}
}

// ---- map admin ------------------------------------------------------------

func TestValidateMap(t *testing.T) {
	svc, _, _ := newEnv(t)

	ve, m := svc.ValidateMap(engine.MapConfig{ID: "bad id", Name: "X"})
	if ve == nil {
		t.Fatal("expected validation issues")
	}
	if m != nil {
		t.Error("a map that failed to parse was still returned")
	}

	ve, m = svc.ValidateMap(triMap())
	if ve != nil {
		t.Fatalf("valid map reported issues: %v", ve)
	}
	if m == nil || m.Par != 3 {
		t.Fatalf("valid map not returned correctly: %+v", m)
	}
}

func TestMapCRUD(t *testing.T) {
	svc, _, _ := newEnv(t)
	if _, err := svc.CreateMap(triMap()); err == nil {
		t.Error("creating a duplicate map should fail")
	}
	cfg := engine.MapConfig{ID: "fresh", Name: "Fresh", Edges: []engine.EdgeConfig{{PointA: 1, PointB: 2}}}
	if _, err := svc.CreateMap(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMap("fresh"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteMap("fresh"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetMap("fresh"); err == nil {
		t.Error("deleted map is still readable")
	}
}

// ---- concurrency ----------------------------------------------------------

func TestConcurrentMovesKeepStateConsistent(t *testing.T) {
	// Twenty goroutines race the same session. Whatever the interleaving, the
	// per-edge usage must never go negative and the counts must stay coherent.
	svc := New(newFakeMaps(engine.MapConfig{
		ID: "many", Name: "Many",
		Edges: []engine.EdgeConfig{
			{PointA: 1, PointB: 2, Count: 50}, {PointA: 2, PointB: 3, Count: 50},
		},
	}), newFakeSessions(), nil, Options{})
	s, err := svc.CreateSession("many", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(s.ID, 1); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				// Alternate directions so many attempts are legal.
				node := 2
				if (i+j)%2 == 0 {
					node = 3
				}
				if _, err := svc.Move(s.ID, node); err != nil {
					var se *Error
					if errors.As(err, &se) && se.Code == CodeSessionTerminal {
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()

	got, err := svc.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, u := range got.State.Usage() {
		if u < 0 {
			t.Errorf("edge %d usage went negative: %d", i, u)
		}
	}
	if got.State.Moves+got.State.Remaining != got.Map.StepCount {
		t.Errorf("moves(%d) + remaining(%d) != step count(%d)",
			got.State.Moves, got.State.Remaining, got.Map.StepCount)
	}
}

func TestShutdownPersistsEverySession(t *testing.T) {
	svc, sessions, _ := newEnv(t)
	if _, err := svc.CreateSession("tri", ""); err != nil {
		t.Fatal(err)
	}
	before := sessions.persists
	svc.Shutdown()
	if sessions.persists <= before {
		t.Error("Shutdown did not persist the live sessions")
	}
}
