package graph

import (
	"context"

	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/graph/model"
	"github.com/wricardo/OneTDraw-Solver/transport/websocket"
)

// forwardSessionUpdates bridges the hub to a gqlgen subscription channel.
//
// It emits the current snapshot first (so a client that subscribes mid-game is
// immediately in sync), then every broadcast. Updates whose Seq is not newer
// than the last one emitted are dropped here as well as in the hub, because a
// transport can deliver out of order under load.
func forwardSessionUpdates(ctx context.Context, svc service.GameService, hub *websocket.Hub, sessionID string, out chan<- *model.SessionUpdate) {
	defer close(out)

	// Subscribe before reading the current snapshot: a broadcast that lands
	// between the two is then queued on the channel instead of being lost.
	ch := hub.SubscribeSession(ctx, sessionID)

	lastSeq := -1
	current, err := svc.GetSession(sessionID)
	if err == nil {
		lastSeq = int(current.Seq)
		if !emitUpdate(ctx, out, *toSessionUpdate(current)) {
			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case snap, ok := <-ch:
			if !ok {
				return
			}
			if snap == nil || int(snap.Seq) <= lastSeq {
				continue
			}
			lastSeq = int(snap.Seq)
			if !emitUpdate(ctx, out, *toSessionUpdate(snap)) {
				return
			}
		}
	}
}

// emitUpdate sends one update, reporting false when the client went away.
func emitUpdate(ctx context.Context, out chan<- *model.SessionUpdate, upd model.SessionUpdate) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- &upd:
		return true
	}
}
