package graph

import (
	"errors"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
)

// codeOf extracts extensions.code from a mapped error.
func codeOf(t *testing.T, err error) string {
	t.Helper()
	mapped := toGQLError(err)
	if mapped == nil {
		return ""
	}
	var ge *gqlerror.Error
	if !errors.As(mapped, &ge) {
		t.Fatalf("mapped error is %T, want *gqlerror.Error", mapped)
	}
	code, _ := ge.Extensions["code"].(string)
	return code
}

func TestToGQLErrorMapsCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"not started", engine.ErrNotStarted, service.CodeInvalidArgument},
		{"terminal", engine.ErrTerminal, service.CodeSessionTerminal},
		{"coded not found", service.Errorf(service.CodeNotFound, "x"), service.CodeNotFound},
		{"coded limit", service.Errorf(service.CodeLimit, "x"), service.CodeLimit},
		{"validation", &engine.ValidationError{Issues: []string{"bad"}}, service.CodeInvalidArgument},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeOf(t, tc.err); got != tc.want {
				t.Errorf("code = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToGQLErrorHidesInternals(t *testing.T) {
	// An unexpected error must not leak its message to the client, but it must
	// still be reported with a code so a client can branch on it.
	err := errors.New("database on fire")
	mapped := toGQLError(err)
	ge, ok := mapped.(*gqlerror.Error)
	if !ok {
		t.Fatalf("mapped type = %T", mapped)
	}
	if ge.Message == "database on fire" {
		t.Error("the internal error message leaked to the client")
	}
	if code, _ := ge.Extensions["code"].(string); code != CodeInternal {
		t.Errorf("code = %q, want %q", code, CodeInternal)
	}
}

func TestToGQLErrorNil(t *testing.T) {
	if toGQLError(nil) != nil {
		t.Error("a nil error should map to nil")
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(service.Errorf(service.CodeNotFound, "x")) {
		t.Error("a NOT_FOUND error was not recognised")
	}
	if isNotFound(service.Errorf(service.CodeLimit, "x")) {
		t.Error("a LIMIT_REACHED error was treated as NOT_FOUND")
	}
	if isNotFound(nil) {
		t.Error("nil was treated as NOT_FOUND")
	}
}
