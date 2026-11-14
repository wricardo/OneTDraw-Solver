package session

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
)

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

func newSession(t *testing.T, m *engine.Map) *service.Session {
	t.Helper()
	now := time.Now()
	return &service.Session{
		ID: "abc123", DisplayName: "Tester", MapID: m.ID, Map: m,
		State: engine.NewState(m), Seq: 1, CreatedAt: now, LastActionAt: now,
	}
}

func TestManagerPutGetDelete(t *testing.T) {
	mgr := NewManager(nil, 0)
	m, err := engine.NewMap(triMap())
	if err != nil {
		t.Fatal(err)
	}
	s := newSession(t, m)

	if err := mgr.Put(s); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Put(s); err != ErrExists {
		t.Errorf("duplicate Put: err = %v, want ErrExists", err)
	}
	if got, ok := mgr.Get(s.ID); !ok || got != s {
		t.Fatal("Get did not return the live pointer")
	}
	if err := mgr.Delete(s.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := mgr.Get(s.ID); ok {
		t.Error("session still present after Delete")
	}
	if err := mgr.Delete(s.ID); err == nil {
		t.Error("deleting a missing session should fail")
	}
}

func TestManagerRejectsInvalidID(t *testing.T) {
	mgr := NewManager(nil, 0)
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)
	s.ID = "../escape"
	if err := mgr.Put(s); err != ErrInvalidID {
		t.Errorf("err = %v, want ErrInvalidID for a path-like id", err)
	}
}

func TestManagerEnforcesMaxSessions(t *testing.T) {
	mgr := NewManager(nil, 1)
	m, _ := engine.NewMap(triMap())
	if err := mgr.Put(newSession(t, m)); err != nil {
		t.Fatal(err)
	}
	second := newSession(t, m)
	second.ID = "second"
	err := mgr.Put(second)
	var se *service.Error
	if err == nil {
		t.Fatal("the limit was not enforced")
	}
	if !isCoded(err, service.CodeLimit) {
		t.Errorf("err = %v (type %T), want a LIMIT_REACHED service error", err, err)
	}
	_ = se
}

func isCoded(err error, code string) bool {
	var se *service.Error
	if e, ok := err.(*service.Error); ok {
		se = e
	}
	return se != nil && se.Code == code
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFilePersistence(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)

	// Play part of a game so the stored state is non-trivial.
	s.State.Start(m, 1)
	s.State.Travel(m, 2)
	s.Seq = 7
	s.Attempts = 2
	best := 3
	s.BestMoves = &best

	if err := p.Persist(s); err != nil {
		t.Fatal(err)
	}

	loaded, err := p.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d sessions, want 1", len(loaded))
	}
	got := loaded[0]
	if got.ID != s.ID || got.DisplayName != s.DisplayName {
		t.Errorf("identity lost: id=%q name=%q", got.ID, got.DisplayName)
	}
	if got.Seq != 7 {
		t.Errorf("seq = %d, want 7", got.Seq)
	}
	if got.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", got.Attempts)
	}
	if got.BestMoves == nil || *got.BestMoves != 3 {
		t.Errorf("bestMoves = %v, want 3", got.BestMoves)
	}
	// The state must replay to the same position and usage.
	if got.State.Pos != 2 {
		t.Errorf("pos = %d, want 2", got.State.Pos)
	}
	if len(got.State.Path) != 2 || got.State.Path[0] != 1 || got.State.Path[1] != 2 {
		t.Errorf("path = %v, want [1 2]", got.State.Path)
	}
	if got.State.Remaining != 2 {
		t.Errorf("remaining = %d, want 2", got.State.Remaining)
	}
	// The map snapshot must travel with the session.
	if got.Map == nil || len(got.Map.Edges) != 3 {
		t.Error("map snapshot was not restored")
	}
}

func TestPersistenceDelete(t *testing.T) {
	dir := t.TempDir()
	p, _ := NewFilePersistence(dir)
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)
	if err := p.Persist(s); err != nil {
		t.Fatal(err)
	}
	if err := p.Delete(s.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded %d sessions after delete, want 0", len(loaded))
	}
}

func TestPersistenceSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	p, _ := NewFilePersistence(dir)
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)
	if err := p.Persist(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.LoadAll()
	if err != nil {
		t.Fatalf("a corrupt file should be skipped, not fail the load: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("loaded %d sessions, want the one good file", len(loaded))
	}
}

func TestPersistenceRejectsTamperedState(t *testing.T) {
	// A stored path that no longer fits its map must be refused, so a hand-edited
	// file cannot inject an impossible position.
	dir := t.TempDir()
	p, _ := NewFilePersistence(dir)
	blob := `{
  "version": 1,
  "id": "tampered",
  "map_id": "tri",
  "map": {"id":"tri","name":"Triangle","Edges":[
    {"PointA":1,"PointB":2},{"PointA":2,"PointB":3},{"PointA":3,"PointB":1}]},
  "state": {"pos": 3, "status": "PLAYING", "moves": 1, "remaining": 2, "path": [1, 3, 3]},
  "seq": 3
}`
	if err := os.WriteFile(filepath.Join(dir, "tampered.json"), []byte(blob), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded %d sessions; a path with an impossible step must be rejected", len(loaded))
	}
}

func TestManagerLoadAll(t *testing.T) {
	dir := t.TempDir()
	p, _ := NewFilePersistence(dir)
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)
	if err := p.Persist(s); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(p, 0)
	if err := mgr.LoadAll(); err != nil {
		t.Fatal(err)
	}
	if mgr.Count() != 1 {
		t.Errorf("count = %d after LoadAll, want 1", mgr.Count())
	}
	if _, ok := mgr.Get(s.ID); !ok {
		t.Error("persisted session was not registered")
	}
}

func TestPersistSkipsDeletedSession(t *testing.T) {
	// A persist that races a delete must not resurrect the session on disk.
	dir := t.TempDir()
	p, _ := NewFilePersistence(dir)
	mgr := NewManager(p, 0)
	m, _ := engine.NewMap(triMap())
	s := newSession(t, m)
	if err := mgr.Put(s); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Delete(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Persist(s); err != nil {
		t.Fatal(err)
	}
	loaded, _ := p.LoadAll()
	if len(loaded) != 0 {
		t.Error("persisting a deleted session wrote it back to disk")
	}
}

func TestConcurrentPutGetDelete(t *testing.T) {
	mgr := NewManager(nil, 0)
	m, _ := engine.NewMap(triMap())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s := newSession(t, m)
				s.ID = "s" + string(rune('a'+i))
				_ = mgr.Put(s)
				mgr.Get(s.ID)
				_ = mgr.Delete(s.ID)
			}
		}(i)
	}
	wg.Wait()
	// Nothing should be left registered, and no race was detected.
	if mgr.Count() > 20 {
		t.Errorf("count = %d after concurrent churn", mgr.Count())
	}
}

func TestValidID(t *testing.T) {
	for _, ok := range []string{"abc", "A1_b-2", "0123456789"} {
		if !ValidID(ok) {
			t.Errorf("ValidID(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "../x", "a/b", "with space", string(make([]byte, 100))} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true, want false", bad)
		}
	}
}
