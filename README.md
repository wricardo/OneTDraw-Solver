# One T Draw

A single-stroke graph puzzle, playable by **humans and AI alike** — and solvable by brute force.

A map is a graph of **nodes** joined by **edges**. Your job is to draw **one continuous trail** that
covers **every edge exactly as many times as its count says**. You may revisit nodes freely; you may
not redraw a spent edge; get stuck with edges still owed and the trail is dead.

Humans play in the browser by clicking nodes. AI agents play the same game through a public
**GraphQL API**: they read the graph, draw edges, and check the result. A human and an AI can share a
session, and anyone can watch it live. The solver is part of the game: press **S** and the same
engine that judges your run finds a completion.

`./one-t-draw -solve puzzles/house.json` still does exactly what this repository always did: it
brute-forces a puzzle file and prints every walk.

## How it works

| Action | What happens |
|---|---|
| Click a node (unstarted) | Places the first node of the trail. Not a counted move. |
| Click a neighbouring node | Draws that edge if it has traversals left. One move. |
| Click a non-neighbour | Blocked: nothing changes, nothing is counted. |
| Draw the last owed edge | You win. |
| Get stuck with edges left | You lose (`DEAD_END`). Undo or reset. |

- An edge labelled **2** must be drawn twice. Spent edges turn grey.
- A **one-way** edge can only be drawn in its arrow's direction.
- Every complete trail has the **same length**, because every solution draws exactly the same edges
  the same number of times. That number is the map's `parMoves`.

## The parity analysis

Before you draw, the map is classified:

| Verdict | Meaning |
|---|---|
| `CIRCUIT` | A closed tour exists, so **any** node may start. |
| `TRAIL` | An open trail exists; it **must** start at one of the odd-degree nodes. |
| `DIRECTED` | The map has one-way edges, so parity does not decide; search does. |
| `IMPOSSIBLE` | No single trail can cover every edge. One map, `jose`, is like this on purpose. |

## Maps

Eleven maps ship with the game, ordered easy to hard: **Regular Triangle**, **Directional
Triangle**, **Little T**, **Jamaican Flag**, **Jose**, **House**, **Level 53–57**. Each map file is a
graph with layout hints; `make validate` checks every map and its reference solution.

## Playing with AI

**1. Copy the prompt from the game.** Start a game in the browser and click **Play with an AI**. It
copies a ready-made prompt with the session and the calls the agent needs. Paste it into an agent
with web access, then watch the session live.

**2. Point an agent at the API.** The game publishes a guide for AI agents at `/llms.txt`, with the
rules, endpoints and examples for every call.

The API is plain GraphQL at `/graphql`. A game takes three calls:

```graphql
query    { maps { id name difficulty stepCount parMoves analysis { verdict oddNodes startNodes } } }  # 1. pick a map
mutation { createSession(mapID: "house") { id state { status } } }                                    # 2. start a game
mutation { start(sessionID: "…", node: 4) { state { pos status } } }                                  # 3. place the pen
mutation { bulkMove(sessionID: "…", nodes: [5,2,1,3,2,4,3,5]) { state { status moves remaining } } }   # 4. draw
```

The solver is on the same API: `solve(sessionID)` attaches a completion plan and `hint(sessionID)`
returns the next node without changing the game.

## Running it

```sh
make run                 # builds and serves http://localhost:8090
make dev                 # run from source with debug logging
make build-frontend      # rebuild the SvelteKit UI into static/
make validate            # check every map and reference solution
make test                # go test -race ./...
make verify              # gofmt check, vet, test, frontend check
```

CLI mode is unchanged:

```sh
./one-t-draw -solve puzzles/house.json                    # every walk
./one-t-draw -solve puzzles/house.json -count_only        # just the count
./one-t-draw -solve puzzles/level54.json -budget 10s      # stop early
./one-t-draw -solve puzzles/house.json -output json       # JSON
```

Environment: `.env.example` documents `ADMIN_API_KEY`, `ALLOWED_ORIGINS`, `MAX_SESSIONS`,
`SESSION_TTL`, and the GraphQL flags.

## Architecture

```
HTTP /graphql (api/server.go) → gqlgen resolver (graph/) → service.GameService (game/service)
   → engine (pure rules) ; step → touch(Seq++) → Clone → persist → broadcast
   → websocket.Hub → graph/subscription_forward.go → graphql-ws → frontend store
```

| Path | Purpose |
|---|---|
| `game/engine` | Rules: map parsing, analysis, the trail state, the planner |
| `game/service` | Sessions and maps; the only layer that mutates a live game |
| `game/config` | File-backed map store (`maps/*.json`) |
| `game/session` | In-memory registry + `sessions/<id>.json` persistence |
| `transport/websocket` | Snapshot fan-out hub |
| `graph/` | `schema.graphqls`, resolvers, conversion; `generated/`, `model/` are gqlgen output |
| `api/` | mux, body/complexity caps, `/llms.txt`, SPA fallback, legacy solver routes |
| `frontend/` | SvelteKit 5 + Tailwind v4; builds into `static/` (committed) |
| `solver/` | The original walk enumerator behind `-solve` and `/puzzle/*` |
| `validate/`, `cmd/validate` | Map + reference-solution checks |
| `maps/`, `solutions/`, `puzzles/` | Data |

### The planner

`engine.Plan` searches for a completion from whatever trail state it is given:

- **Dead-branch pruning.** If the edges still owed can no longer all be reached from the pen, the
  branch is abandoned.
- **Parity pruning.** With only undirected edges left, a completion leaves zero or two odd-degree
  nodes, and if two the pen is on one of them. With only one-way edges left it uses the directed
  balance test. With both, no sound static test exists, so nothing is pruned.
- **Memoisation.** A state is (position, remaining traversals per edge); a subtree proven dead is
  never explored twice.
- **Warnsdorff ordering.** Forced corridors are tried before open rooms.

Neither pruning rule can discard a real solution, so `Plan` returning `found: false` means there is
none.

### Layout

Map files carry a `Level` per point, which is a hint rather than a geometry: several nodes often
share a level, and one map has no levels at all. Placing each level on a row drew those nodes in a
straight line and collapsed a triangle into one segment. `computeLayout` therefore seeds the rows and
then runs a fixed number of deterministic force-relaxation iterations (springs along edges,
repulsion between all pairs, a weak pull to the seed row). No randomness, so a map looks the same on
every load and in every client.

## Deployment

Production runs at **https://onetdraw.wricardo.net** on the shared ARM64 Ubuntu EC2 host
(`184.73.131.85`) alongside the other Dr Brain games, behind nginx with a Let's Encrypt
certificate.

**Continuous deployment.** Every push to `master` runs `.github/workflows/deploy.yml`: vet, test,
`cmd/validate`, build a static `linux/arm64` binary, upload it with `maps/`, `static/`, `puzzles/`,
`legacy-static/`, `solutions/` and `puzzles.json`, unpack into `/opt/onetdraw` (keeping `sessions/`
and `.env`), restart the service and smoke-test GraphQL. The archive is unpacked to a staging
directory and verified before the running release is replaced, so a truncated upload cannot leave
the site with no maps and no UI.

Repository secrets: `EC2_HOST`, `EC2_SSH_KEY` (a deploy-only key), `EC2_KNOWN_HOSTS`.

**One-time host setup** (done; repeat only on a new host):

1. `/opt/onetdraw/` owned by `ubuntu`, with `sessions/` and a `.env` (playground off, a random
   `ADMIN_API_KEY`, `ALLOWED_ORIGINS` set to the site's http and https origins).
2. systemd unit `onetdraw.service`: `WorkingDirectory=/opt/onetdraw`,
   `ExecStart=/opt/onetdraw/onetdraw -host 127.0.0.1 -port 8088 -maps-dir maps -sessions-dir sessions -public-url https://onetdraw.wricardo.net`,
   `Restart=always`.
3. nginx `server` block for `onetdraw.wricardo.net` proxying to `127.0.0.1:8088` with WebSocket
   upgrade headers and a long `proxy_read_timeout`.
4. DNS at the host, then `sudo certbot --nginx -d onetdraw.wricardo.net --redirect`
   (`certbot.timer` renews).
5. The deploy key's public half in `~/.ssh/authorized_keys`, and the three secrets above.

**Ports on this host.** `8080` tesla, `8081`, `8082` impostor, `8083` ai-http-bin, `8084`
colorsort, `8085` quickgql, `8086` motor-programming, `8087` mazes, **`8088` one-t-draw**. Pick a
free one before adding a service.

**Shipping UI changes.** CI builds the frontend (`npm ci`, `svelte-check`, `npm test`,
`build:static`) before packaging, so a UI change is deployed as soon as it is pushed — there is no
"remember to rebuild `static/`" step. `static/` stays committed so the server can be run locally
without Node; run `make build-frontend` if you want that copy to match. Note that Vite derives chunk
filenames from the build, so a local rebuild rewrites ~17 files even when the output is
byte-identical — CI's artifact is the authoritative one.

## About

One T Draw is part of **Dr Brain**, a collection of browser brain games. The original solver was a
collaborative challenge between [@adarqui](https://github.com/adarqui) and
[@wricardo](https://github.com/wricardo), inspired by the iPhone game "One T Draw".
