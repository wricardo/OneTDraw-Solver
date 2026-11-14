// Package websocket fans session snapshots out to subscribers. It knows nothing
// about the websocket protocol itself; the GraphQL transport layer owns that.
package websocket

import (
	"context"
	"sync"

	"github.com/wricardo/OneTDraw-Solver/game/service"
)

// subscriberBuffer is the per-subscriber channel capacity.
const subscriberBuffer = 8

// Hub implements service.Broadcaster. The zero value is not usable; call
// NewHub.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan *service.Session]struct{} // session id -> subscriber channels
	last map[string]uint64                             // session id -> highest Seq published
}

var _ service.Broadcaster = (*Hub)(nil)

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		subs: make(map[string]map[chan *service.Session]struct{}),
		last: make(map[string]uint64),
	}
}

// Broadcast publishes snapshot to every subscriber of snapshot.ID. The snapshot
// is shared by all subscribers and MUST be treated as read-only (the service
// only hands out clones). Broadcast never blocks: when a subscriber's buffer is
// full its oldest queued snapshot is dropped, so the newest update (e.g. the
// terminal WON/LOST one) is never lost. Snapshots whose Seq is not newer than
// the highest one already published for the session are discarded, so a late
// stale broadcast cannot evict a newer one.
func (h *Hub) Broadcast(snapshot *service.Session) {
	if snapshot == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.subs[snapshot.ID]
	if len(set) == 0 {
		return
	}
	if last, ok := h.last[snapshot.ID]; ok && snapshot.Seq <= last {
		return
	}
	h.last[snapshot.ID] = snapshot.Seq
	for ch := range set {
		publishLatest(ch, snapshot)
	}
}

// publishLatest enqueues v on ch, evicting the oldest queued value while the
// buffer is full. The caller must be the only sender on ch.
func publishLatest(ch chan *service.Session, v *service.Session) {
	for {
		select {
		case ch <- v:
			return
		default:
		}
		select {
		case <-ch: // drop oldest
		default:
		}
	}
}

// SubscribeSession returns a channel that receives a snapshot on every
// Broadcast for sessionID. The channel has a buffer of 8 with latest-wins
// delivery. It is closed once ctx is done.
func (h *Hub) SubscribeSession(ctx context.Context, sessionID string) <-chan *service.Session {
	ch := make(chan *service.Session, subscriberBuffer)
	h.mu.Lock()
	set := h.subs[sessionID]
	if set == nil {
		set = make(map[chan *service.Session]struct{})
		h.subs[sessionID] = set
	}
	set[ch] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		if s := h.subs[sessionID]; s != nil {
			delete(s, ch)
			if len(s) == 0 {
				delete(h.subs, sessionID)
				// Keep h.last until the session is gone from memory; a
				// resubscribe then correctly ignores stale snapshots.
			}
		}
		h.mu.Unlock()
		close(ch)
	}()
	return ch
}

// Forget drops the last-published sequence for sessionID. Call it when a
// session is deleted, so the id can be reused without stale filtering.
func (h *Hub) Forget(sessionID string) {
	h.mu.Lock()
	delete(h.last, sessionID)
	h.mu.Unlock()
}

// SubscriberCount reports how many channels are subscribed to a session.
func (h *Hub) SubscriberCount(sessionID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[sessionID])
}
