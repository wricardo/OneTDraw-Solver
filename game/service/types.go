// Package service owns the game sessions and maps. It is the only layer that
// mutates a live game: it locks a session, applies the change, bumps its
// sequence number, clones it, unlocks, persists, then broadcasts the clone.
package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

// Error is a coded service error. The GraphQL layer surfaces Code as
// extensions.code.
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// Error codes.
const (
	CodeNotFound        = "NOT_FOUND"
	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeSessionTerminal = "SESSION_TERMINAL"
	CodeForbidden       = "FORBIDDEN"
	CodeLimit           = "LIMIT_REACHED"
)

// Errorf builds a coded error.
func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Session is one game on one map. The map is a private snapshot taken when the
// session was created, so editing or deleting a map never touches a game in
// progress.
//
// A live *Session is guarded by its own Mutex. Never broadcast, persist or do
// I/O while holding it.
type Session struct {
	ID          string
	DisplayName string
	MapID       string
	Map         *engine.Map
	State       engine.State
	LastEvent   *engine.StepEvent

	// Seq increases on every change. Clients ignore updates with seq <= the
	// last one they saw.
	Seq          uint64
	CreatedAt    time.Time
	LastActionAt time.Time
	// Attempts counts finished games (won or lost) in this session.
	Attempts int
	// BestMoves is the fewest moves in a won game, nil before the first win.
	BestMoves *int
	// Solved is set once a solution has been asked for, so the client can show
	// the solver result without re-requesting it.
	LastPlan *Plan
	// AutoPlay marks a session driven by the built-in solver.
	AutoPlay bool

	mu sync.Mutex
}

// Plan is a solver result attached to a session.
type Plan struct {
	Start int
	Steps []int
	Found bool
	// At is when the plan was computed.
	At time.Time
}

// Lock acquires the session lock.
func (s *Session) Lock() { s.mu.Lock() }

// Unlock releases the session lock.
func (s *Session) Unlock() { s.mu.Unlock() }

// Clone returns a deep copy safe to hand to subscribers. The caller must hold
// the session lock.
func (s *Session) Clone() *Session {
	c := &Session{
		ID: s.ID, DisplayName: s.DisplayName, MapID: s.MapID,
		Map: s.Map, State: s.State.Clone(), Seq: s.Seq,
		CreatedAt: s.CreatedAt, LastActionAt: s.LastActionAt,
		Attempts: s.Attempts, AutoPlay: s.AutoPlay,
	}
	if s.BestMoves != nil {
		b := *s.BestMoves
		c.BestMoves = &b
	}
	if s.LastEvent != nil {
		ev := *s.LastEvent
		c.LastEvent = &ev
	}
	if s.LastPlan != nil {
		p := *s.LastPlan
		p.Steps = append([]int(nil), s.LastPlan.Steps...)
		c.LastPlan = &p
	}
	return c
}

// MapStore lists, reads and writes maps.
type MapStore interface {
	List() []*engine.Map
	Get(id string) (*engine.Map, error)
	Save(cfg engine.MapConfig) (*engine.Map, error)
	Create(cfg engine.MapConfig) (*engine.Map, error)
	Delete(id string) error
}

// SessionStore keeps live sessions and durably persists them.
type SessionStore interface {
	Put(s *Session) error
	Get(id string) (*Session, bool)
	List() []*Session
	Delete(id string) error
	// Persist writes a snapshot. The caller must not hold the session lock.
	Persist(s *Session) error
}

// Broadcaster fans session snapshots out to subscribers.
type Broadcaster interface {
	Broadcast(snapshot *Session)
}

// SessionFilter selects and orders ListSessions results.
type SessionFilter struct {
	Sort  string // "recent" (LastActionAt) or "created" (CreatedAt)
	Limit int
	MapID string
}

// GameService is the session and map API used by the GraphQL resolvers.
type GameService interface {
	// Sessions
	CreateSession(mapID, displayName string) (*Session, error)
	GetSession(id string) (*Session, error)
	ListSessions(f SessionFilter) ([]*Session, error)
	DeleteSession(id string) error
	RenameSession(id, displayName string) (*Session, error)
	Start(sessionID string, node int) (*Session, error)
	Move(sessionID string, node int) (*Session, error)
	BulkMove(sessionID string, nodes []int, stopOnBlocked bool) (*Session, error)
	Undo(sessionID string) (*Session, error)
	Reset(sessionID string) (*Session, error)
	// Hint reports the next node on a completion, without changing the game.
	Hint(sessionID string) (*engine.PlanResult, error)
	// Solve attaches a full completion plan to the session.
	Solve(sessionID string) (*Session, error)

	// Maps
	ListMaps() []*engine.Map
	GetMap(id string) (*engine.Map, error)
	SaveMap(cfg engine.MapConfig) (*engine.Map, error)
	CreateMap(cfg engine.MapConfig) (*engine.Map, error)
	DeleteMap(id string) error
	// ValidateMap checks a map without storing it.
	ValidateMap(cfg engine.MapConfig) (*engine.ValidationError, *engine.Map)

	Shutdown()
}

// IsNotFound reports whether err is a NOT_FOUND service error.
func IsNotFound(err error) bool {
	var se *Error
	return errors.As(err, &se) && se.Code == CodeNotFound
}
