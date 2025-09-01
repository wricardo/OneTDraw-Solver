# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OneTDraw-Solver is a Go-based algorithm implementation that solves OneTDraw puzzles (inspired by the iPhone game "One T Draw"). The solver uses concurrent goroutines to find all possible solutions for graph traversal puzzles where edges must be visited exactly once.

## Architecture

### Core Components

1. **Solver Package** (`solver/`)
   - `solver.go`: Main solving algorithm using recursive backtracking with concurrent starting point exploration
   - `solution_handler.go`: Handles solution collection and counting
   - `printer.go`: Formats solutions for output (JSON or clean text)

2. **Web Interface**
   - `webserver.go`: HTTP server using goweb framework
   - `routes.go`: API endpoints for puzzle listing, solving, and visualization
   - `static/ui2.html`: Canvas-based UI for puzzle visualization

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

# JSON output format
go run main.go -solve puzzles/house.json -output json

# Run without using all CPU cores
go run main.go -maxprocs=false
```

### Testing
```bash
# Run tests (uses launchpad.net/gocheck framework)
go test ./solver

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

The solver uses parallel goroutines to explore all possible starting points simultaneously. Each goroutine performs depth-first search with backtracking to find valid paths that visit all edges exactly as specified by their count. Solutions are collected thread-safely and deduplicated before returning.