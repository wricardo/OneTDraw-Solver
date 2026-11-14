# Repository Guidelines

## Project Overview
One T Draw (product name: **One T Draw**; part of the **Dr Brain** collection) is a single-stroke
graph puzzle server (Go 1.25) plus a SvelteKit (Svelte 5) UI. A player draws one continuous trail
covering every edge of a graph exactly as many times as its `count` says. It exposes GraphQL
(queries, mutations, subscriptions over graphql-ws), `/llms.txt` (agent-facing rules, rendered from
`llms.txt.tmpl`), the original solver REST routes, and serves the built SPA.

The repository began as a standalone puzzle **solver**; that code lives on untouched in `solver/`
behind `-solve` and `routes.go`, and must keep working.

## Architecture & Data Flow
```
HTTP /graphql (api/server.go) → gqlgen resolver (graph/) → service.GameService (game/service)
   → engine (pure rules) ; step → Seq++ → Clone → persist → Broadcast
   → websocket.Hub (8-slot per-subscriber queue, drop-oldest) → graph/subscription_forward.go
   → graphql-ws → frontend store
```
- **Composition root**: `main.go:run` wires `config.Manager` (MapStore) +
  `session.Manager`/`FilePersistence` (SessionStore) + `websocket.Hub` (Broadcaster) into
  `service.New(...)`, then `api.NewServer`. Shutdown: stop HTTP, then persist sessions.
- **CLI mode**: `main.go:solveFile` keeps the original `-solve`/`-count_only`/`-output`/`-budget`
  behaviour, including the analysis line on stderr.
- **DI via interfaces** defined in `game/service` (`MapStore`, `SessionStore`, `Broadcaster`).
  `service` never imports `session`/`websocket`.
- **Engine** (`game/engine`): no IO. `NewMap`, `NewState`, `State.Start/Travel/Undo`,
  `Map.Analysis`, `Plan`, `Hint`, `ValidSolution`. `State` mutators use pointer receivers.
- **Locking**: a live `*service.Session` is guarded by `Lock/Unlock`. Never broadcast, do I/O, or
  `Persist` while holding the session lock. Hub/resolvers only see clones.
- **Delivery**: every broadcast = full snapshot + monotonic `Seq`. Subscription resolver subscribes
  first, then emits the current snapshot, then only increasing `Seq`. The frontend store merges by
  `seq` too.
- **Map snapshot**: sessions embed their own map copy, so editing/deleting a map never disturbs a
  game in progress.
- **Errors**: `*service.Error{Code,Msg}` (`NOT_FOUND`, `INVALID_ARGUMENT`, `SESSION_TERMINAL`,
  `FORBIDDEN`, `LIMIT_REACHED`), surfaced as `extensions.code`. Engine sentinels (`ErrNotStarted`,
  `ErrTerminal`) are translated in `game/service` so every transport gets the same codes. Unknown
  errors log and return `INTERNAL`.
- **Blocked moves**: not an error. They bump `Seq`, set `lastEvent.blocked`, and are broadcast, but
  are not counted and are not persisted.

## Key Directories
| Path | Purpose |
|---|---|
| `game/engine` | rules, analysis, trail state, planner (`types.go`, `state.go`) |
| `game/service` | `Session`, `GameService` (`impl.go`), errors/types |
| `game/config` | map store: `maps/*.json`, RWMutex, atomic temp+rename writes |
| `game/session` | in-memory registry + `sessions/<id>.json` persistence |
| `transport/websocket` | `Hub` (Broadcaster impl) |
| `graph/` | `schema.graphqls`, resolvers, `convert.go`, admin gate; `generated/`, `model/` are gqlgen output (do not hand-edit) |
| `api/` | mux, body cap (1 MiB), complexity cap, WS origin check, SPA fallback, legacy routes |
| `solver/`, `routes.go`, `webserver.go` | the original solver, untouched in behaviour |
| `validate/`, `cmd/validate` | map + reference-solution checks |
| `maps/`, `solutions/`, `puzzles/` | data; `sessions/` is runtime, gitignored |
| `frontend/` | SvelteKit + Tailwind v4; builds into `static/` (committed) |

## Development Commands
```bash
make build | run PORT=8090 | dev          # dev = go run . -port 8090 -debug
make test                                  # go test -race ./...
make validate                              # all maps + reference solutions
make generate                              # gqlgen, after editing graph/schema.graphqls
make check-frontend                        # svelte-check + vitest
make verify                                # fmt check, vet, test, frontend check
go test ./game/engine -run TestPlanSolvesEveryBundledMap   # single test
cd frontend && npm run build:static         # → static/
```

**gqlgen toolchain gotcha.** `gqlgen generate` fails under Go 1.26+ with
`package "github.com/vektah/gqlparser/v2/ast" without types was imported`. The `make generate`
target therefore runs it under the pinned Go 1.24 toolchain from the module cache
(`GOTOOLCHAIN=local`). Without that toolchain the target warns and tries anyway, which will fail.
Do not "fix" this by bumping gqlgen blindly: it changes `graph/generated` wholesale.

## Code Conventions & Common Patterns
- Standard `gofmt`; domain-noun types (`GameService`, `MapStore`), concrete impls often `Manager`.
- GraphQL camelCase; enums/codes `UPPER_SNAKE`.
- Mutating service pattern: lock → mutate → `touch` (Seq++, LastActionAt) → `Clone` → unlock →
  persist → broadcast.
- `Map.Clone` must never re-run the solver: it is called per move.
- `Plan` must stay allocation-light: it mutates one state and undoes each move (`applyMove`/
  `undoMove`), rather than cloning a state per branch.
- Layout is deterministic: no randomness, fixed iteration count, in `computeLayout`.
- Schema change flow: edit `graph/schema.graphqls` → `make generate` → update `graph/convert.go` +
  resolvers → mirror by hand in `frontend/src/lib/{queries,types}.ts` and `llms.txt.tmpl`.
- Frontend: Svelte 5 runes only (`$state/$derived/$effect/$props`), `onclick` attributes, keyed
  `{#each}`; urql (network-only, POST) + lazy shared `graphql-ws`; `unwrap()` takes an operation and
  awaits `toPromise()`.
- New top-level route → also add it to `spaRoutes` in `api/server.go`.

## Adding a map
Create `maps/<id>.json` with `id` (must match the file name), `name`, `description`, `difficulty`,
`Points` (with layout `Level`, plus `Hide` for spacers with no edges) and `Edges`
(`PointA`/`PointB`, optional `Count`, optional `Direction`). Then:
```bash
go run ./cmd/validate -write   # writes solutions/<id>.json for any map that has none
make validate
```
A map that is legitimately unsolvable (four odd nodes) is allowed but must have no reference
solution, and the game shows it as `IMPOSSIBLE`.

## Testing & QA
- Go: stdlib `testing`, `t.Helper`/`TempDir`/`Cleanup`, table + `t.Run`. Always run with `-race`.
- Golden invariant: `TestPlanSolvesEveryBundledMap` proves every shipped map's plan is a real walk.
- Service tests use hand-written fakes (`fakeMaps`, `fakeSessions`, `recorder`) in `impl_test.go`.
- API tests use `httptest.Server` and the real embedded `llms.txt.tmpl`.
- Frontend: Vitest + jsdom (`cd frontend && npx vitest run`), single file
  `npx vitest run src/lib/board.test.ts`.
- UI changes: load `/play/<id>` and `/solver` in a browser.

## Known Issues
- None tracked. Add items here only when verified.
