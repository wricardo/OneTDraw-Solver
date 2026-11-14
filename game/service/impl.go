package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

const (
	// DefaultMaxDisplayName is the display-name limit in characters.
	DefaultMaxDisplayName = 40
	// DefaultListLimit and MaxListLimit bound ListSessions.
	DefaultListLimit = 50
	MaxListLimit     = 500
	// MaxSteps bounds the nodes accepted by one BulkMove call. Blocked attempts
	// are free, so without a cap a request could ask for unbounded work.
	MaxSteps = 20000
)

// Options tunes New. The zero value selects the defaults.
type Options struct {
	// SessionTTL: on New, sessions whose LastActionAt is older than this are
	// deleted. 0 (default) disables sweeping.
	SessionTTL time.Duration
	// MaxDisplayName is the maximum display-name length in characters.
	MaxDisplayName int
}

type gameService struct {
	maps     MapStore
	sessions SessionStore
	bc       Broadcaster // may be nil
	opts     Options
}

// New creates the game service. bc may be nil (no subscribers).
func New(maps MapStore, sessions SessionStore, bc Broadcaster, opts Options) GameService {
	if opts.MaxDisplayName <= 0 {
		opts.MaxDisplayName = DefaultMaxDisplayName
	}
	svc := &gameService{maps: maps, sessions: sessions, bc: bc, opts: opts}
	if opts.SessionTTL > 0 {
		svc.sweepExpired(time.Now().Add(-opts.SessionTTL))
	}
	return svc
}

func (svc *gameService) sweepExpired(cutoff time.Time) {
	for _, s := range svc.sessions.List() {
		s.Lock()
		expired := s.LastActionAt.Before(cutoff)
		id := s.ID
		s.Unlock()
		if !expired {
			continue
		}
		if err := svc.DeleteSession(id); err != nil {
			log.Printf("service: sweep expired session %s: %v", id, err)
		}
	}
}

// ---- helpers ---------------------------------------------------------------

func notFound(id string) *Error { return Errorf(CodeNotFound, "session %q not found", id) }

func (svc *gameService) get(id string) (*Session, error) {
	s, ok := svc.sessions.Get(id)
	if !ok {
		return nil, notFound(id)
	}
	return s, nil
}

func (svc *gameService) persist(s *Session) {
	if err := svc.sessions.Persist(s); err != nil {
		log.Printf("service: persist session %s: %v", s.ID, err)
	}
}

func (svc *gameService) broadcast(snap *Session) {
	if svc.bc != nil {
		svc.bc.Broadcast(snap)
	}
}

// touch records a state change: Seq++ and LastActionAt. Lock must be held.
func touch(s *Session) {
	s.Seq++
	s.LastActionAt = time.Now()
}

// recordOutcome counts a finished game. Lock must be held, and the state must
// be terminal.
func recordOutcome(s *Session) {
	s.Attempts++
	if s.State.Status == engine.StatusWon && (s.BestMoves == nil || s.State.Moves < *s.BestMoves) {
		b := s.State.Moves
		s.BestMoves = &b
	}
}

// applyStep applies one engine step on s, records it and counts an outcome if
// the game just ended. Lock must be held.
func applyStep(s *Session, fn func(*engine.State, *engine.Map) (engine.StepEvent, error)) (engine.StepEvent, bool, error) {
	ev, err := fn(&s.State, s.Map)
	if err != nil {
		return ev, false, err
	}
	s.LastEvent = &ev
	if s.State.Status.Terminal() {
		recordOutcome(s)
		return ev, true, nil
	}
	return ev, false, nil
}

func (svc *gameService) cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n > svc.opts.MaxDisplayName {
		return "", Errorf(CodeInvalidArgument, "display name too long (%d characters, max %d)", n, svc.opts.MaxDisplayName)
	}
	return name, nil
}

func newSessionID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// requirePlaying returns SESSION_TERMINAL if s has ended. Lock must be held.
func requirePlaying(s *Session) error {
	if s.State.Status.Terminal() {
		return Errorf(CodeSessionTerminal, "trail is already %s; reset the session first", s.State.Status)
	}
	return nil
}

// ---- sessions --------------------------------------------------------------

// CreateSession starts a session on a private snapshot of map mapID.
func (svc *gameService) CreateSession(mapID, displayName string) (*Session, error) {
	name, err := svc.cleanName(displayName)
	if err != nil {
		return nil, err
	}
	m, err := svc.maps.Get(mapID)
	if err != nil {
		return nil, err
	}
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	snapMap := m.Clone()
	now := time.Now()
	s := &Session{
		ID: id, DisplayName: name, MapID: snapMap.ID, Map: snapMap,
		State: engine.NewState(snapMap),
		Seq:   1, CreatedAt: now, LastActionAt: now,
	}
	if err := svc.sessions.Put(s); err != nil {
		return nil, err
	}
	svc.persist(s)
	s.Lock()
	snap := s.Clone()
	s.Unlock()
	svc.broadcast(snap)
	return snap, nil
}

// GetSession returns a snapshot.
func (svc *gameService) GetSession(id string) (*Session, error) {
	s, err := svc.get(id)
	if err != nil {
		return nil, err
	}
	s.Lock()
	defer s.Unlock()
	return s.Clone(), nil
}

// ListSessions returns snapshots matching f.
func (svc *gameService) ListSessions(f SessionFilter) ([]*Session, error) {
	limit := f.Limit
	switch {
	case limit < 0:
		return nil, Errorf(CodeInvalidArgument, "limit must be >= 0")
	case limit == 0:
		limit = DefaultListLimit
	case limit > MaxListLimit:
		limit = MaxListLimit
	}
	byCreated := false
	switch strings.ToLower(f.Sort) {
	case "", "recent":
	case "created":
		byCreated = true
	default:
		return nil, Errorf(CodeInvalidArgument, "unknown sort %q (want recent or created)", f.Sort)
	}

	type entry struct {
		s   *Session
		id  string
		key time.Time
	}
	var entries []entry
	for _, s := range svc.sessions.List() {
		s.Lock()
		if f.MapID == "" || s.MapID == f.MapID {
			key := s.LastActionAt
			if byCreated {
				key = s.CreatedAt
			}
			entries = append(entries, entry{s: s, id: s.ID, key: key})
		}
		s.Unlock()
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].key.Equal(entries[j].key) {
			return entries[i].key.After(entries[j].key)
		}
		return entries[i].id < entries[j].id
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]*Session, len(entries))
	for i, e := range entries {
		e.s.Lock()
		out[i] = e.s.Clone()
		e.s.Unlock()
	}
	return out, nil
}

// DeleteSession removes the session from memory and storage.
func (svc *gameService) DeleteSession(id string) error {
	return svc.sessions.Delete(id)
}

// RenameSession sets the display name.
func (svc *gameService) RenameSession(id, name string) (*Session, error) {
	name, err := svc.cleanName(name)
	if err != nil {
		return nil, err
	}
	s, err := svc.get(id)
	if err != nil {
		return nil, err
	}
	s.Lock()
	s.DisplayName = name
	touch(s)
	snap := s.Clone()
	s.Unlock()
	svc.persist(s)
	svc.broadcast(snap)
	return snap, nil
}

// Start places the first node of the trail. It is not a counted move, so a
// blocked choice bumps Seq and is broadcast but does not count.
func (svc *gameService) Start(sessionID string, node int) (*Session, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	err = requirePlaying(s)
	if err != nil {
		s.Unlock()
		return nil, err
	}
	ev, _, err := applyStep(s, func(st *engine.State, m *engine.Map) (engine.StepEvent, error) {
		return st.Start(m, node)
	})
	if err != nil {
		s.Unlock()
		return nil, err
	}
	touch(s)
	snap := s.Clone()
	s.Unlock()
	if !ev.Blocked {
		svc.persist(s)
	}
	svc.broadcast(snap)
	return snap, nil
}

// Move draws one edge to node. A blocked attempt is not an error and is not
// counted, but it still bumps Seq and is broadcast, so subscribers can show the
// rejection.
func (svc *gameService) Move(sessionID string, node int) (*Session, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	return svc.step(s, func(st *engine.State, m *engine.Map) (engine.StepEvent, error) {
		return st.Travel(m, node)
	})
}

// step runs one engine step against a live session, translating the engine's
// sentinel errors into coded service errors so every transport gets the same
// codes.
func (svc *gameService) step(s *Session, fn func(*engine.State, *engine.Map) (engine.StepEvent, error)) (*Session, error) {
	s.Lock()
	if err := requirePlaying(s); err != nil {
		s.Unlock()
		return nil, err
	}
	ev, _, err := applyStep(s, fn)
	if err != nil {
		s.Unlock()
		return nil, codedStepError(err)
	}
	touch(s)
	snap := s.Clone()
	s.Unlock()
	if !ev.Blocked {
		svc.persist(s)
	}
	svc.broadcast(snap)
	return snap, nil
}

// codedStepError maps an engine step error to a service error.
func codedStepError(err error) error {
	switch {
	case errors.Is(err, engine.ErrNotStarted):
		return Errorf(CodeInvalidArgument, "place the first node with start() before moving")
	case errors.Is(err, engine.ErrTerminal):
		return Errorf(CodeSessionTerminal, "the trail is already finished; reset the session first")
	default:
		return err
	}
}

// BulkMove draws a whole node sequence under one lock and one broadcast. It
// stops as soon as the game ends and, when stopOnBlocked is set, at the first
// blocked step, so a wrong plan cannot run on from the wrong node. Otherwise
// blocked steps are processed and ignored.
func (svc *gameService) BulkMove(sessionID string, nodes []int, stopOnBlocked bool) (*Session, error) {
	if len(nodes) > MaxSteps {
		return nil, Errorf(CodeInvalidArgument, "too many steps (%d, max %d)", len(nodes), MaxSteps)
	}
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	if err := requirePlaying(s); err != nil {
		s.Unlock()
		return nil, err
	}
	if len(nodes) == 0 {
		snap := s.Clone()
		s.Unlock()
		return snap, nil
	}
	moved := false
	for _, n := range nodes {
		ev, ended, err := applyStep(s, func(st *engine.State, m *engine.Map) (engine.StepEvent, error) {
			return st.Travel(m, n)
		})
		if err != nil {
			s.Unlock()
			return nil, err
		}
		moved = moved || !ev.Blocked
		if ended || (ev.Blocked && stopOnBlocked) {
			break
		}
	}
	touch(s)
	snap := s.Clone()
	s.Unlock()
	if moved {
		svc.persist(s)
	}
	svc.broadcast(snap)
	return snap, nil
}

// Undo takes back the last drawn edge.
func (svc *gameService) Undo(sessionID string) (*Session, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	if len(s.State.Path) < 2 {
		s.Unlock()
		return nil, Errorf(CodeInvalidArgument, "nothing to undo: the trail has no drawn edge yet")
	}
	ev, ok := s.State.Undo(s.Map)
	if ok {
		s.LastEvent = &ev
	}
	touch(s)
	snap := s.Clone()
	s.Unlock()
	svc.persist(s)
	svc.broadcast(snap)
	return snap, nil
}

// Reset puts the session back at the start, keeping Attempts and BestMoves.
func (svc *gameService) Reset(sessionID string) (*Session, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	s.State = engine.NewState(s.Map)
	s.LastEvent = nil
	s.LastPlan = nil
	touch(s)
	snap := s.Clone()
	s.Unlock()
	svc.persist(s)
	svc.broadcast(snap)
	return snap, nil
}

// Hint computes the next node on some completion, without modifying the game.
func (svc *gameService) Hint(sessionID string) (*engine.PlanResult, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	st := s.State.Clone()
	m := s.Map
	s.Unlock()
	plan, err := engine.Plan(m, st)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// Solve attaches a full completion plan to the session and broadcasts it. It
// does not play the plan; a client may replay it with bulkMove.
func (svc *gameService) Solve(sessionID string) (*Session, error) {
	s, err := svc.get(sessionID)
	if err != nil {
		return nil, err
	}
	s.Lock()
	st := s.State.Clone()
	m := s.Map
	s.Unlock()

	plan, err := engine.Plan(m, st)
	if err != nil {
		return nil, err
	}
	if !plan.Found {
		return nil, Errorf(CodeInvalidArgument, "no solution could be found for this trail state")
	}

	s.Lock()
	s.LastPlan = &Plan{Start: plan.Start, Steps: append([]int(nil), plan.Steps...), Found: true, At: time.Now()}
	touch(s)
	snap := s.Clone()
	s.Unlock()
	svc.persist(s)
	svc.broadcast(snap)
	return snap, nil
}

// ---- maps ------------------------------------------------------------------

// ListMaps lists all maps.
func (svc *gameService) ListMaps() []*engine.Map { return svc.maps.List() }

// GetMap returns one map.
func (svc *gameService) GetMap(id string) (*engine.Map, error) { return svc.maps.Get(id) }

// SaveMap validates and stores a map; existing sessions keep their snapshot.
func (svc *gameService) SaveMap(cfg engine.MapConfig) (*engine.Map, error) {
	return svc.maps.Save(cfg)
}

// CreateMap validates and stores a new map; it fails if the id already exists.
func (svc *gameService) CreateMap(cfg engine.MapConfig) (*engine.Map, error) {
	return svc.maps.Create(cfg)
}

// DeleteMap removes a map. Existing sessions keep their snapshot.
func (svc *gameService) DeleteMap(id string) error { return svc.maps.Delete(id) }

// ValidateMap checks a map without storing it. It returns the structural error
// (nil when the map parses) and the parsed map (nil when it does not).
func (svc *gameService) ValidateMap(cfg engine.MapConfig) (*engine.ValidationError, *engine.Map) {
	m, err := engine.NewMap(cfg)
	if err != nil {
		var ve *engine.ValidationError
		if errors.As(err, &ve) {
			return ve, nil
		}
		return &engine.ValidationError{Issues: []string{err.Error()}}, nil
	}
	return nil, m
}

// ---- shutdown --------------------------------------------------------------

// Shutdown persists every session. It is idempotent.
func (svc *gameService) Shutdown() {
	for _, s := range svc.sessions.List() {
		svc.persist(s)
	}
}
