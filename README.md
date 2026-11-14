# OneTDraw-Solver

A high-performance Go-based algorithm implementation that solves OneTDraw puzzles - graph traversal puzzles where edges must be visited exactly once. This project was created as a collaborative challenge between [@adarqui](https://github.com/adarqui) and [@wricardo](https://github.com/wricardo), inspired by the iPhone game "One T Draw".

<img width="409" height="512" alt="Screenshot 2025-09-01 at 6 39 17 AM" src="https://github.com/user-attachments/assets/5cf503cd-e447-4de4-8b5e-910d8b586837" />

## Features

- **High-Performance Solver**: concurrent goroutines with recursive backtracking and dead-branch pruning, so puzzles with 90k+ solutions finish in well under a second
- **Bounded and cancellable searches**: every search accepts a time budget, a solution cap, and a stop signal, and reports whether it can be trusted as complete
- **Web Interface**: interactive graph visualization with live progress while searching, a Stop button, route-by-route playback, and a paged gallery of solutions
- **Pre-flight analysis**: tells you before solving whether a single path can exist at all, and where it must start (Euler parity and connectivity)
- **Multiple Output Formats**: Clean text or JSON output
- **Flexible Puzzle Format**: JSON-based puzzle definition with support for directional constraints
- **Comprehensive Test Suite**: unit tests, route tests, and benchmarking

## How It Works

The solver runs one goroutine per starting point, each doing depth-first search with backtracking. Two things make it fast on large puzzles:

- **Dead-branch pruning**: after every step it checks whether the edges still owed can all still be reached. If the unused edges have split into two disconnected groups, no continuation can cover them all, so the whole subtree is abandoned. This only discards branches with provably zero completions, so the solutions found are unchanged.
- **Bounded search**: a search can be limited by time, by a solution cap, or cancelled outright. Because these puzzles produce astronomically many walks on the larger levels, the web UI streams progress while searching and lets you stop, instead of blocking on a request that may never finish.

Solutions are also deduplicated: two walks that draw the same lines (one being the other reversed, or — for closed tours — a rotation of it) count once.

## Usage

### Web Interface (Default)

Start the web server to visualize and solve puzzles interactively:

```bash
go run main.go
# Server starts on http://localhost:8090
```

The UI shows the graph with a pre-flight analysis, then streams the search with live progress. Pick a search budget from the toolbar, and press **Stop** at any time — the routes found so far are kept.

### Command Line Solving

```bash
# Solve and show all solutions
go run main.go -solve puzzles/house.json

# Count solutions only
go run main.go -solve puzzles/house.json -count_only

# Stop after 10s and report what was found (useful for the biggest levels)
go run main.go -solve puzzles/level54.json -count_only -budget 10s

# JSON output format
go run main.go -solve puzzles/house.json -output json
```

The CLI prints a one-line analysis to stderr and keeps stdout clean, so `-count_only` and `-output json` stay machine-readable. If a search does not finish within `-budget`, the count is reported as a lower bound.

### Puzzle Format

Puzzles are defined in JSON format with points and edges:

```json
{
  "Points": [
    {"Point": 1, "Level": 1},
    {"Point": 2, "Level": 2}
  ],
  "Edges": [
    {
      "PointA": 1,
      "PointB": 2,
      "Count": 1,
      "Direction": "optional_constraint"
    }
  ]
}
```

- `Points`: Graph vertices with optional level information for visualization
- `Edges`: Connections between points
  - `Count`: Number of times the edge must be traversed
  - `Direction`: Optional unidirectional constraint

## Example Puzzles

The repository includes various puzzle examples in the `puzzles/` directory:

- `house.json` - Classic house drawing puzzle
- `regular_triangle.json` - Simple triangle
- `jamaican_flag.json` - Flag pattern
- `level53.json` through `level57.json` - Game levels

## Development

### Testing

```bash
# Run tests
go test ./solver

# Run benchmarks
go test -bench=. ./solver
```

### Code Formatting

```bash
# Format and fix imports
goimports -w .
```

## Dependencies

- `github.com/stretchr/goweb` - Web framework
- `launchpad.net/gocheck` - Testing framework

## Architecture

- **`solver/`**: core algorithm. `solver.go` holds the search and its bounds, `analyze.go` holds the pre-flight analysis and solution deduplication
- **`webserver.go`**: HTTP server
- **`routes.go`**: the API, including the streaming solve/count endpoints
- **`static/index.html`**: the UI, a single self-contained page (no external CDNs)
- **`puzzles/`**: example puzzle definitions

### API

| Route | Purpose |
| --- | --- |
| `GET /puzzles` | Puzzle list, each with its size and solvability verdict |
| `GET /puzzle/data/{name}` | Geometry plus analysis, for drawing the board |
| `GET /puzzle/stream/{name}` | NDJSON stream: `progress`/`heartbeat` lines, then a `result` line |
| `GET /puzzle/solve/{name}` | One-shot JSON result |
| `GET /puzzle/count/{name}` | NDJSON stream of an exhaustive count |
| `POST /puzzle/stop/{session}` | Ask a running stream to finish early and still return its results |

Both streaming routes accept `?budgetMs=`, and `/puzzle/stream/` also accepts `?limit=`. A stream is cancelled automatically if the client disconnects, so an abandoned tab does not leave a search burning CPU.

## Contributing

This project uses GOPATH-style Go modules. Contributions welcome - feel free to add new puzzles, improve the algorithm, or enhance the web interface.