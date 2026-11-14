# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OneTDraw-Solver is a Go-based algorithm implementation that solves OneTDraw puzzles (inspired by the iPhone game "One T Draw"). The solver uses concurrent goroutines to find all possible solutions for graph traversal puzzles where edges must be visited exactly once.

## Architecture

### Core Components

1. **Solver Package** (`solver/`)
   - `solver.go`: Main solving algorithm — recursive backtracking with concurrent starting point exploration, dead-branch pruning, and time/solution/cancel bounds
   - `analyze.go`: Pre-flight Euler analysis (parity, connectivity) and solution deduplication
   - `solution_handler.go`: Handles solution collection and counting
   - `printer.go`: Formats solutions for output (JSON or clean text)

2. **Web Interface**
   - `webserver.go`: HTTP server using goweb framework
   - `routes.go`: API endpoints for puzzle listing, analysis, streaming solve/count, and stopping a search
   - `static/index.html`: Self-contained UI (no external CDNs) with live progress, route playback, and a paged solution gallery
   - `static/ui2.html`: The original canvas UI, kept for reference but no longer served by `/`

3. **Puzzle Format**
   - JSON structure with `Edges` array containing:
     - `PointA`, `PointB`: Edge endpoints
     - `Count`: Number of times edge must be traversed
     - `Direction`: Optional unidirectional constraint

## Commands

### Build and Run
```bash
# Run with web server (default on port 8090)
go run main.go

# Solve a specific puzzle file
go run main.go -solve puzzles/house.json

# Count solutions only
go run main.go -solve puzzles/house.json -count_only

# Stop after a time budget (the biggest levels cannot be enumerated)
go run main.go -solve puzzles/level54.json -budget 10s

# JSON output format
go run main.go -solve puzzles/house.json -output json

# Run without using all CPU cores
go run main.go -maxprocs=false
```

### Testing
```bash
# Run all tests
go test ./...

# Run benchmarks
go test -bench=. ./solver
```

### Code Formatting
```bash
# Format and fix imports before building
goimports -w .
```

## Dependencies

This is a GOPATH-style project (no go.mod) with dependencies:
- `github.com/stretchr/goweb` - Web framework
- `launchpad.net/gocheck` - Testing framework

## Algorithm Details

The solver uses parallel goroutines to explore all possible starting points simultaneously. Each goroutine performs depth-first search with backtracking to find valid paths that visit all edges exactly as specified by their count.

Two properties make it fast enough for the larger levels:

- **Dead-branch pruning.** After each step, if the edges still owed have split into groups that can no longer reach each other, the subtree is abandoned. This only cuts branches with zero completions, so the solution set is unchanged.
- **Bounded search.** Every search accepts a deadline, a solution cap, and a cancel channel. The largest bundled puzzle (`level54`) has a walk space far too large to enumerate, so the UI streams progress and lets the user stop; results found so far are kept.