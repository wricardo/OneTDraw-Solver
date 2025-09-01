# OneTDraw-Solver

A high-performance Go-based algorithm implementation that solves OneTDraw puzzles - graph traversal puzzles where edges must be visited exactly once. This project was created as a collaborative challenge between [@adarqui](https://github.com/adarqui) and [@wricardo](https://github.com/wricardo), inspired by the iPhone game "One T Draw".

<img width="409" height="512" alt="Screenshot 2025-09-01 at 6 39 17 AM" src="https://github.com/user-attachments/assets/5cf503cd-e447-4de4-8b5e-910d8b586837" />

## Features

- **High-Performance Solver**: Uses concurrent goroutines with recursive backtracking to explore all possible solutions
- **Web Interface**: Interactive canvas-based puzzle visualization and solving
- **Multiple Output Formats**: Clean text or JSON output
- **Flexible Puzzle Format**: JSON-based puzzle definition with support for directional constraints
- **Comprehensive Test Suite**: Built with benchmarking capabilities

## How It Works

The solver uses parallel goroutines to simultaneously explore all possible starting points in a graph. Each goroutine performs depth-first search with backtracking to find valid paths that traverse all edges exactly as specified. Solutions are collected thread-safely and deduplicated before output.

## Installation

This is a GOPATH-style Go project. Clone and run:

```bash
git clone https://github.com/wricardo/OneTDraw-Solver.git
cd OneTDraw-Solver
go get -d ./...
```

## Usage

### Web Interface (Default)

Start the web server to visualize and solve puzzles interactively:

```bash
go run main.go
# Server starts on http://localhost:8090
```

### Command Line Solving

Solve a specific puzzle file:

```bash
# Solve and show all solutions
go run main.go -solve puzzles/house.json

# Count solutions only
go run main.go -solve puzzles/house.json -count_only

# JSON output format
go run main.go -solve puzzles/house.json -output json

# Limit CPU usage (disable multi-core processing)
go run main.go -maxprocs=false
```

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

- **`solver/`**: Core solving algorithm and solution handling
- **`webserver.go`**: HTTP server with puzzle API
- **`routes.go`**: Web API endpoints
- **`static/ui2.html`**: Canvas-based puzzle visualization
- **`puzzles/`**: Example puzzle definitions

## Contributing

This project uses GOPATH-style Go modules. Contributions welcome - feel free to add new puzzles, improve the algorithm, or enhance the web interface.