// Package session keeps live game sessions in memory and persists them to
// disk, so a restart does not lose a game in progress.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
)

// DefaultMaxSessions is used when NewManager is given maxSessions <= 0.
const DefaultMaxSessions = 1000

// Errors returned by the manager and its persistence layer.
var (
	ErrInvalidID = errors.New("invalid session id")
	ErrExists    = errors.New("session already exists")
)

var idRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// ValidID reports whether id is a safe session id (also a safe file name).
func ValidID(id string) bool { return idRe.MatchString(id) }

// Persistence loads and stores session files. A nil Persistence keeps sessions
// in memory only.
type Persistence interface {
	LoadAll() ([]*service.Session, error)
	Persist(s *service.Session) error
	Delete(id string) error
}

// Manager is the in-memory registry of live sessions with optional
// write-through persistence. It implements service.SessionStore.
//
// Lock order: ioMu -> Session lock. mu is only ever held briefly and never
// together with a Session lock or while doing file I/O.
type Manager struct {
	p           Persistence
	maxSessions int

	mu       sync.RWMutex
	sessions map[string]*service.Session

	// ioMu serialises persistence writes and deletes so file contents always
	// follow the order snapshots were taken in, and so a Delete can never be
	// overtaken by a late Persist of the same session.
	ioMu sync.Mutex
}

var _ service.SessionStore = (*Manager)(nil)

// NewManager creates a manager. p may be nil for a memory-only store.
func NewManager(p Persistence, maxSessions int) *Manager {
	if maxSessions <= 0 {
		maxSessions = DefaultMaxSessions
	}
	return &Manager{p: p, maxSessions: maxSessions, sessions: make(map[string]*service.Session)}
}

// LoadAll registers every session the persistence layer has on disk. Sessions
// whose stored map no longer parses are skipped with a logged warning.
func (m *Manager) LoadAll() error {
	if m.p == nil {
		return nil
	}
	sessions, err := m.p.LoadAll()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if err := m.Put(s); err != nil {
			log.Printf("session: skip persisted session %s: %v", s.ID, err)
		}
	}
	return nil
}

// Put registers a new live session. It fails with LIMIT_REACHED when the cap is
// reached, ErrInvalidID for a malformed id and ErrExists for a duplicate. It
// does not persist.
func (m *Manager) Put(s *service.Session) error {
	if s == nil || !ValidID(s.ID) {
		return ErrInvalidID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[s.ID]; ok {
		return ErrExists
	}
	if len(m.sessions) >= m.maxSessions {
		return service.Errorf(service.CodeLimit, "session limit reached (%d live sessions); delete one first", m.maxSessions)
	}
	m.sessions[s.ID] = s
	return nil
}

// Get returns the live session pointer.
func (m *Manager) Get(id string) (*service.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

// Delete removes the session from memory and storage.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	_, found := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()

	if m.p != nil && ValidID(id) {
		m.ioMu.Lock()
		err := m.p.Delete(id)
		m.ioMu.Unlock()
		switch {
		case err == nil:
			found = true
		case errors.Is(err, os.ErrNotExist):
		default:
			return err
		}
	}
	if !found {
		return service.Errorf(service.CodeNotFound, "session %q not found", id)
	}
	return nil
}

// List returns every live session (live pointers, unspecified order).
func (m *Manager) List() []*service.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*service.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

// Count returns the number of live sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Persist durably writes a snapshot of s. It takes the session lock itself, so
// callers MUST NOT hold it. Sessions no longer registered (deleted
// concurrently) are skipped, so a late persist cannot resurrect them.
func (m *Manager) Persist(s *service.Session) error {
	if m.p == nil || s == nil {
		return nil
	}
	if _, ok := m.Get(s.ID); !ok {
		return nil
	}
	s.Lock()
	snap := s.Clone()
	s.Unlock()

	m.ioMu.Lock()
	defer m.ioMu.Unlock()
	return m.p.Persist(snap)
}

// FilePersistence stores one JSON file per session in a directory.
type FilePersistence struct {
	dir string
}

// NewFilePersistence creates the directory if missing and returns a store.
func NewFilePersistence(dir string) (*FilePersistence, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	return &FilePersistence{dir: dir}, nil
}

// sessionDisk is the on-disk form of a session: the map snapshot travels with
// it, so a session still replays correctly after its map is edited or deleted.
type sessionDisk struct {
	Version      int               `json:"version"`
	ID           string            `json:"id"`
	DisplayName  string            `json:"display_name"`
	MapID        string            `json:"map_id"`
	Map          engine.MapConfig  `json:"map"`
	State        engine.State      `json:"state"`
	LastEvent    *engine.StepEvent `json:"last_event,omitempty"`
	LastPlan     *planDisk         `json:"last_plan,omitempty"`
	Seq          uint64            `json:"seq"`
	CreatedAt    string            `json:"created_at"`
	LastActionAt string            `json:"last_action_at"`
	Attempts     int               `json:"attempts"`
	BestMoves    *int              `json:"best_moves,omitempty"`
	AutoPlay     bool              `json:"auto_play,omitempty"`
}

type planDisk struct {
	Start int   `json:"start"`
	Steps []int `json:"steps"`
	Found bool  `json:"found"`
}

// Persist writes s to <dir>/<id>.json atomically.
func (p *FilePersistence) Persist(s *service.Session) error {
	return p.write(s)
}

// Delete removes <dir>/<id>.json.
func (p *FilePersistence) Delete(id string) error {
	if !ValidID(id) {
		return ErrInvalidID
	}
	err := os.Remove(p.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return os.ErrNotExist
	}
	return err
}

func (p *FilePersistence) path(id string) string { return filepath.Join(p.dir, id+".json") }

// LoadAll reads every session file, replaying each stored state against its map
// snapshot so a corrupted or stale file cannot inject an impossible position.
func (p *FilePersistence) LoadAll() ([]*service.Session, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}
	var out []*service.Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		s, err := p.read(filepath.Join(p.dir, e.Name()))
		if err != nil {
			log.Printf("session: skip %s: %v", e.Name(), err)
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (p *FilePersistence) read(path string) (*service.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d sessionDisk
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	if !ValidID(d.ID) {
		return nil, ErrInvalidID
	}
	mp, err := engine.NewMap(d.Map)
	if err != nil {
		return nil, fmt.Errorf("map snapshot: %w", err)
	}
	st, err := replayState(mp, d.State)
	if err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}
	created, err := parseTime(d.CreatedAt)
	if err != nil {
		return nil, err
	}
	lastAction, err := parseTime(d.LastActionAt)
	if err != nil {
		return nil, err
	}
	s := &service.Session{
		ID: d.ID, DisplayName: d.DisplayName, MapID: d.MapID, Map: mp,
		State: st, LastEvent: d.LastEvent, Seq: d.Seq,
		CreatedAt: created, LastActionAt: lastAction,
		Attempts: d.Attempts, BestMoves: d.BestMoves, AutoPlay: d.AutoPlay,
	}
	if d.LastPlan != nil {
		s.LastPlan = &service.Plan{Start: d.LastPlan.Start, Steps: d.LastPlan.Steps, Found: d.LastPlan.Found, At: lastAction}
	}
	return s, nil
}

func (p *FilePersistence) write(s *service.Session) error {
	d := sessionDisk{
		Version: 1, ID: s.ID, DisplayName: s.DisplayName, MapID: s.MapID,
		Map: s.Map.Config(), State: s.State, LastEvent: s.LastEvent,
		Seq: s.Seq, CreatedAt: formatTime(s.CreatedAt), LastActionAt: formatTime(s.LastActionAt),
		Attempts: s.Attempts, BestMoves: s.BestMoves, AutoPlay: s.AutoPlay,
	}
	if s.LastPlan != nil {
		d.LastPlan = &planDisk{Start: s.LastPlan.Start, Steps: s.LastPlan.Steps, Found: s.LastPlan.Found}
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(p.dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, p.path(s.ID))
}

// replayState rebuilds a state by replaying its recorded path against the map,
// which re-derives the per-edge usage counts. It rejects a path that no longer
// fits the map.
func replayState(m *engine.Map, stored engine.State) (engine.State, error) {
	st := engine.NewState(m)
	if len(stored.Path) == 0 {
		return st, nil
	}
	ev, err := st.Start(m, stored.Path[0])
	if err != nil {
		return engine.State{}, err
	}
	if ev.Blocked {
		return engine.State{}, fmt.Errorf("cannot start at node %d", stored.Path[0])
	}
	for i := 1; i < len(stored.Path); i++ {
		ev, err := st.Travel(m, stored.Path[i])
		if err != nil {
			return engine.State{}, err
		}
		if ev.Blocked {
			return engine.State{}, fmt.Errorf("step %d blocked: %s", i, ev.Reason)
		}
	}
	st.Blocked = stored.Blocked
	return st, nil
}
