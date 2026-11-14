// Package graph implements the GraphQL API: gqlgen resolvers, subscription
// forwarding, and conversion from engine/service types to schema models.
package graph

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/transport/websocket"
)

// CodeInternal is the extensions.code for unexpected server failures.
const CodeInternal = "INTERNAL"

// subscriptionBuffer is the output channel size of the subscription forwarder.
const subscriptionBuffer = 8

// Resolver is the gqlgen dependency injection root.
type Resolver struct {
	Service service.GameService
	Hub     *websocket.Hub
}

// NewResolver builds the resolver root from the game service and hub.
func NewResolver(svc service.GameService, hub *websocket.Hub) *Resolver {
	return &Resolver{Service: svc, Hub: hub}
}

// HTTPRequestKey is the context key under which the *http.Request is stored by
// the HTTP middleware, so resolvers can read headers (X-Admin-Key).
type HTTPRequestKey struct{}

// checkAdminKey gates admin operations. ADMIN_API_KEY must be configured and
// sent as X-Admin-Key; local development can opt out explicitly with
// ALLOW_UNAUTHENTICATED_ADMIN=true (only honoured while no key is configured).
func checkAdminKey(ctx context.Context) error {
	required := os.Getenv("ADMIN_API_KEY")
	if required == "" {
		if os.Getenv("ALLOW_UNAUTHENTICATED_ADMIN") == "true" {
			return nil
		}
		return codedError(service.CodeForbidden, "admin operation disabled: set ADMIN_API_KEY and send X-Admin-Key")
	}
	r, _ := ctx.Value(HTTPRequestKey{}).(*http.Request)
	if r == nil {
		return codedError(service.CodeForbidden, "admin operation requires X-Admin-Key header")
	}
	// Compare fixed-length digests so the key length is not leaked.
	got := sha256.Sum256([]byte(r.Header.Get("X-Admin-Key")))
	want := sha256.Sum256([]byte(required))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return codedError(service.CodeForbidden, "forbidden: invalid or missing X-Admin-Key")
	}
	return nil
}

// codedError builds a gqlerror carrying extensions.code.
func codedError(code, msg string) *gqlerror.Error {
	return &gqlerror.Error{Message: msg, Extensions: map[string]any{"code": code}}
}

// toGQLError maps service/engine errors to gqlerrors with extensions.code.
// Coded service errors keep their code; map validation failures become
// INVALID_ARGUMENT; anything else is logged and reported as INTERNAL.
func toGQLError(err error) error {
	if err == nil {
		return nil
	}
	var ge *gqlerror.Error
	if errors.As(err, &ge) {
		return ge
	}
	var se *service.Error
	if errors.As(err, &se) {
		return codedError(se.Code, se.Msg)
	}
	var ve *engine.ValidationError
	if errors.As(err, &ve) {
		return codedError(service.CodeInvalidArgument, ve.Error())
	}
	if errors.Is(err, engine.ErrNotStarted) {
		return codedError(service.CodeInvalidArgument, "place the first node with start() before moving")
	}
	if errors.Is(err, engine.ErrTerminal) {
		return codedError(service.CodeSessionTerminal, err.Error())
	}
	log.Printf("graph: internal error: %v", err)
	return codedError(CodeInternal, "internal error")
}

// isNotFound reports whether err is a coded NOT_FOUND service error.
func isNotFound(err error) bool {
	var se *service.Error
	return errors.As(err, &se) && se.Code == service.CodeNotFound
}
