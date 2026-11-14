// Command one-t-draw serves One T Draw as a playable game for humans and AI
// agents: a GraphQL API with websocket subscriptions, a SvelteKit UI, the
// original solver REST routes, and a CLI mode that still solves a puzzle file.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/wricardo/OneTDraw-Solver/api"
	"github.com/wricardo/OneTDraw-Solver/game/config"
	"github.com/wricardo/OneTDraw-Solver/game/service"
	"github.com/wricardo/OneTDraw-Solver/game/session"
	"github.com/wricardo/OneTDraw-Solver/solver"
	"github.com/wricardo/OneTDraw-Solver/transport/websocket"
)

const (
	appName = "one-t-draw"

	defaultMaxSessions = 1000
	shutdownTimeout    = 10 * time.Second
)

//go:embed llms.txt.tmpl
var llmsTxtSource string

var (
	port        = flag.Int("port", 8090, "HTTP server port")
	host        = flag.String("host", "0.0.0.0", "HTTP server host")
	mapsDir     = flag.String("maps-dir", "maps", "Directory containing map JSON files")
	sessionsDir = flag.String("sessions-dir", "sessions", "Directory for persisted sessions")
	debug       = flag.Bool("debug", false, "Enable debug logging")
	publicURL   = flag.String("public-url", "", "Public base URL rendered into /llms.txt (defaults to the request host)")

	// CLI solving. These keep the original command-line behaviour.
	solvePath = flag.String("solve", "", "File path to a puzzle to solve, then exit (CLI mode)")
	countOnly = flag.Bool("count_only", false, "With -solve: print only the count of solutions")
	output    = flag.String("output", "clean", "With -solve: output format [clean,json]")
	budget    = flag.Duration("budget", 0, "With -solve: stop after this long (0 = no limit)")
	maxProcs  = flag.Bool("maxprocs", true, "Pass false to NOT use all CPU cores")
)

func main() {
	flag.Parse()
	if *maxProcs {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	if *solvePath != "" {
		solveFile(*solvePath)
		return
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// envBool reads a boolean env var; unset returns def. "true", "1" and "yes" are
// true, anything else is false.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch v {
	case "true", "1", "yes":
		return true
	}
	return false
}

// envInt reads an integer env var; unset or invalid returns def.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// run wires the composition root and serves until a signal arrives.
func run() error {
	_ = godotenv.Load()

	maps, err := config.NewManager(*mapsDir)
	if err != nil {
		return fmt.Errorf("load maps: %w", err)
	}
	persist, err := session.NewFilePersistence(*sessionsDir)
	if err != nil {
		return fmt.Errorf("session store: %w", err)
	}
	sessions := session.NewManager(persist, envInt("MAX_SESSIONS", defaultMaxSessions))
	if err := sessions.LoadAll(); err != nil {
		return fmt.Errorf("restore sessions: %w", err)
	}
	hub := websocket.NewHub()

	ttl := time.Duration(0)
	if raw := os.Getenv("SESSION_TTL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			ttl = d
		} else {
			log.Printf("ignoring invalid SESSION_TTL %q: %v", raw, err)
		}
	}
	svc := service.New(maps, sessions, hub, service.Options{SessionTTL: ttl})

	srv, err := api.NewServer(svc, hub, api.Options{
		Introspection:  envBool("GRAPHQL_INTROSPECTION", true),
		Playground:     envBool("GRAPHQL_PLAYGROUND", true),
		AllowedOrigins: os.Getenv("ALLOWED_ORIGINS"),
		PublicURL:      *publicURL,
		LLMSTemplate:   llmsTxtSource,
		Legacy:         legacyHandler(),
		LegacyPrefixes: []string{"/puzzles", "/puzzle/", "/legacy-static/"},
	})
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: streaming solve routes legitimately run longer than a
		// normal request, and a write deadline would truncate the response
		// mid-body instead of reporting the delay.
		MaxHeaderBytes: 1 << 20,
	}
	if *debug {
		log.Printf("%s starting in debug mode", appName)
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (maps=%s sessions=%s)", addr, *mapsDir, *sessionsDir)
		log.Printf("GraphQL: http://%s/graphql  llms.txt: http://%s/llms.txt", addr, addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stop:
		log.Printf("received %v, shutting down", sig)
	case err := <-serveErr:
		svc.Shutdown()
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	svc.Shutdown()
	log.Println("server stopped")
	return nil
}

// solveFile is the original CLI mode: solve one puzzle file with the legacy
// solver and print the result. It is untouched by the game.
func solveFile(path string) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error loading puzzle: %v", err)
	}
	puzzle, err := solver.NewPuzzleFromBytes(fileContent)
	if err != nil {
		log.Fatalf("Error loading puzzle: %v", err)
	}

	// The analysis goes to stderr so it never corrupts the machine-readable
	// stdout that -count_only and -output json produce.
	fmt.Fprintf(os.Stderr, "%s\n", solver.Analyze(puzzle))

	if *countOnly {
		count, stats := solver.CountWith(puzzle, *budget, nil)
		if stats.Exhaustive {
			fmt.Println(count)
			return
		}
		fmt.Fprintf(os.Stderr, "Count incomplete after %v: at least %d walks (%d branches)\n",
			stats.Elapsed.Round(time.Millisecond), count, stats.Visited)
		fmt.Println(count)
		return
	}

	solutions, stats := solver.SolveWith(puzzle, 0, *budget, nil, nil)
	if !stats.Exhaustive {
		fmt.Fprintf(os.Stderr, "Search incomplete after %v: %d walks found, %d branches explored\n",
			stats.Elapsed.Round(time.Millisecond), len(*solutions), stats.Visited)
	}
	if *output == "json" {
		solutions.Print(solver.JsonPrinter{})
		return
	}
	solutions.Print(solver.CleanPrinter{})
}
