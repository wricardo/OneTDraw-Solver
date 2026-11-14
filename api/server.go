// Package api assembles the HTTP surface: GraphQL (POST plus websocket
// subscriptions), the playground, /llms.txt, the legacy OneTDraw REST routes
// and the SPA frontend.
package api

import (
	"bytes"
	"context"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	gorillaws "github.com/gorilla/websocket"
	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/graph"
	"github.com/wricardo/OneTDraw-Solver/graph/generated"
	"github.com/wricardo/OneTDraw-Solver/graph/model"
	"github.com/wricardo/OneTDraw-Solver/transport/websocket"
)

const (
	// MaxRequestBodyBytes caps GraphQL POST bodies.
	MaxRequestBodyBytes = 1 << 20
	// ComplexityLimit bounds the cost of one GraphQL operation.
	ComplexityLimit = 2000
	// wsPingInterval is the graphql-transport-ws keepalive ping period.
	wsPingInterval = 10 * time.Second
	// wsInitTimeout bounds the wait for connection_init after an upgrade.
	wsInitTimeout = 10 * time.Second
	// sessionsPerCostUnit: a sessions list of N rows costs ceil(N/10) times its
	// per-row selection cost.
	sessionsPerCostUnit = 10
	// mapsPerCostUnit: the maps query returns every map with its whole graph,
	// so it is priced per map.
	mapsPerCostUnit = 1
)

// Options configures the server.
type Options struct {
	// Introspection enables GraphQL introspection (GRAPHQL_INTROSPECTION).
	Introspection bool
	// Playground serves /playground (GRAPHQL_PLAYGROUND).
	Playground bool
	// AllowedOrigins is the comma-separated ALLOWED_ORIGINS list for websocket
	// upgrades; empty or "*" allows all.
	AllowedOrigins string
	// PublicURL overrides the base URL rendered into /llms.txt; empty derives it
	// from the request.
	PublicURL string
	// LLMSTemplate is the text/template source for /llms.txt.
	LLMSTemplate string
	// WSInitTimeout is how long a websocket may stay open without sending
	// connection_init; zero means the default.
	WSInitTimeout time.Duration
	// UIDirs are candidate frontend directories, first existing wins. Defaults
	// to ./frontend/build then ./static.
	UIDirs []string
	// Legacy is the handler serving the original OneTDraw solver REST routes
	// (/puzzles, /puzzle/...). nil disables them.
	Legacy http.Handler
	// LegacyPrefixes are the top-level path prefixes routed to Legacy.
	LegacyPrefixes []string
}

// Server is the HTTP handler for the whole application.
type Server struct {
	router *mux.Router
	llms   *template.Template
	opts   Options
}

// NewServer builds the router for svc/hub.
func NewServer(svc service.GameService, hub *websocket.Hub, opts Options) (*Server, error) {
	llms, err := template.New("llms").Parse(opts.LLMSTemplate)
	if err != nil {
		return nil, err
	}
	s := &Server{router: mux.NewRouter(), llms: llms, opts: opts}

	s.router.Handle("/graphql", withCORS(opts.AllowedOrigins, withHTTPRequest(limitBody(newGraphQLServer(svc, hub, opts)))))
	if opts.Playground {
		s.router.Handle("/playground", playground.Handler("One T Draw GraphQL playground", "/graphql"))
	}
	s.router.HandleFunc("/llms.txt", s.handleLLMS)
	s.router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	if opts.Legacy != nil {
		for _, prefix := range opts.LegacyPrefixes {
			s.router.PathPrefix(prefix).Handler(opts.Legacy)
		}
	}
	s.router.PathPrefix("/").Handler(spaHandler(opts.UIDirs))
	return s, nil
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// newGraphQLServer builds the gqlgen handler with transports and guards.
func newGraphQLServer(svc service.GameService, hub *websocket.Hub, opts Options) *handler.Server {
	initTimeout := opts.WSInitTimeout
	if initTimeout == 0 {
		initTimeout = wsInitTimeout
	}
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers:  graph.NewResolver(svc, hub),
		Complexity: complexityRoot(),
	}))
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(&transport.Websocket{
		KeepAlivePingInterval: wsPingInterval,
		PingPongInterval:      wsPingInterval,
		InitTimeout:           initTimeout,
		Upgrader:              newUpgrader(opts.AllowedOrigins),
	})
	srv.Use(extension.FixedComplexityLimit(ComplexityLimit))
	if opts.Introspection {
		srv.Use(extension.Introspection{})
	}
	srv.AroundOperations(logOperation)
	return srv
}

// complexityRoot weights the queries that fan out, so aliased abuse exceeds
// ComplexityLimit while the frontend's own queries stay under it.
func complexityRoot() generated.ComplexityRoot {
	var c generated.ComplexityRoot
	// sessions: every sessionsPerCostUnit rows add one more copy of the
	// per-row cost; limit is clamped like the service does.
	c.Query.Sessions = func(child int, _ *model.SessionSort, limit *int, _ *string) int {
		n := service.DefaultListLimit
		if limit != nil && *limit > 0 {
			n = min(*limit, service.MaxListLimit)
		}
		units := (n + sessionsPerCostUnit - 1) / sessionsPerCostUnit
		return safeMul(child, units) + 1
	}
	// maps: the catalog carries every map's whole graph, which is the single
	// most expensive field in the schema.
	c.Query.Maps = func(child int) int {
		return safeMul(child, mapsPerCostUnit) + 1
	}
	return c
}

// safeMul multiplies non-negative a by b without overflowing.
func safeMul(a, b int) int {
	if a != 0 && b > math.MaxInt/a {
		return math.MaxInt
	}
	return a * b
}

// logOperation logs every GraphQL operation on completion.
func logOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	opCtx := graphql.GetOperationContext(ctx)
	name := opCtx.OperationName
	if name == "" {
		name = "<anonymous>"
	}
	start := time.Now()
	rh := next(ctx)
	return func(ctx context.Context) *graphql.Response {
		resp := rh(ctx)
		if resp != nil {
			log.Printf("[graphql] op=%q duration=%s errors=%d", name, time.Since(start), len(resp.Errors))
		}
		return resp
	}
}

// withCORS answers CORS for browsers calling /graphql cross-origin. Origins
// follow the same allowlist as the websocket check (empty or "*" = all).
func withCORS(allowedOrigins string, next http.Handler) http.Handler {
	allowed := parseAllowedOrigins(allowedOrigins)
	check := makeCheckOrigin(allowed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || !check(r) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "POST")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Key")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withHTTPRequest stores the *http.Request in the context so resolvers can read
// headers (X-Admin-Key).
func withHTTPRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), graph.HTTPRequestKey{}, r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// limitBody caps request bodies at MaxRequestBodyBytes.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// newUpgrader builds the websocket upgrader whose origin check honours the
// comma-separated allowed list ("" or "*" allows everything).
func newUpgrader(allowedOrigins string) gorillaws.Upgrader {
	return gorillaws.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     makeCheckOrigin(parseAllowedOrigins(allowedOrigins)),
	}
}

// parseAllowedOrigins splits the comma-separated ALLOWED_ORIGINS value. Empty
// or "*" yields nil, meaning every origin is allowed.
func parseAllowedOrigins(allowedOrigins string) []string {
	var allowed []string
	if allowedOrigins != "" && allowedOrigins != "*" {
		for _, o := range strings.Split(allowedOrigins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowed = append(allowed, o)
			}
		}
	}
	return allowed
}

// makeCheckOrigin allows every origin when allowed is empty, otherwise an exact
// match. Requests without an Origin header (non-browser clients) are allowed.
func makeCheckOrigin(allowed []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		if len(allowed) == 0 {
			return true
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		log.Printf("websocket: rejected origin %q (not in ALLOWED_ORIGINS)", origin)
		return false
	}
}

// handleLLMS renders /llms.txt. Base URL: PublicURL if set, otherwise the host
// the client used, so docs never point at a different server.
func (s *Server) handleLLMS(w http.ResponseWriter, r *http.Request) {
	baseURL := strings.TrimRight(s.opts.PublicURL, "/")
	if baseURL == "" {
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		baseURL = scheme + "://" + r.Host
	}
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http")
	var buf bytes.Buffer
	data := struct{ BaseURL, WSURL string }{BaseURL: baseURL, WSURL: wsURL}
	if err := s.llms.Execute(&buf, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

// spaRoutes mirrors the top-level directories in frontend/src/routes.
var spaRoutes = map[string]bool{
	"": true, "play": true, "watch": true, "maps": true, "editor": true,
	"learn": true, "sessions": true, "solver": true,
}

// isSPARoute reports whether the first path segment is a client-side route.
func isSPARoute(urlPath string) bool {
	first, _, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	return spaRoutes[first]
}

// spaHandler serves the first existing directory of dirs (default
// ./frontend/build then ./static) with SPA fallback to index.html. Unknown
// routes still report 404 so crawlers and clients see real status codes.
func spaHandler(dirs []string) http.Handler {
	if len(dirs) == 0 {
		dirs = []string{"./frontend/build", "./static"}
	}
	uiDir := dirs[len(dirs)-1]
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			uiDir = d
			break
		}
	}
	fs := http.FileServer(http.Dir(uiDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(uiDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			setCacheHeaders(w, r.URL.Path)
			fs.ServeHTTP(w, r)
			return
		}
		index, err := os.ReadFile(filepath.Join(uiDir, "index.html"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if !isSPARoute(r.URL.Path) {
			w.WriteHeader(http.StatusNotFound)
		}
		_, _ = w.Write(index)
	})
}

// setCacheHeaders marks hashed SvelteKit assets immutable and everything else
// revalidatable.
func setCacheHeaders(w http.ResponseWriter, urlPath string) {
	if strings.HasPrefix(urlPath, "/_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
}
