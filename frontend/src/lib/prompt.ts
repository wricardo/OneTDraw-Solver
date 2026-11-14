// A ready-made prompt to hand the game to an AI assistant.
//
// The point is that a person can copy one block and paste it into any agent with
// web access; the agent then knows the endpoint, the session id and the calls it
// needs. Everything here is derived from the session, so the prompt can never
// drift from the schema.

import type { Session } from './types';

/** Builds the prompt for one session. */
export function buildSessionPrompt(origin: string, session: Session): string {
	const base = origin.replace(/\/$/, '');
	const api = `${base}/graphql`;
	const map = session.map;
	const startHint =
		map.analysis.verdict === 'TRAIL'
			? `This map's parity requires starting at node ${map.analysis.oddNodes.join(' or ')}.`
			: map.analysis.verdict === 'CIRCUIT'
				? 'Any node may start (closed tour).'
				: map.analysis.verdict === 'IMPOSSIBLE'
					? 'This map cannot be solved in one trail; say so rather than guessing.'
					: 'One-way edges decide the start; any node with an edge may work.';

	return `You are playing One T Draw: draw ONE continuous trail that covers every edge of the graph
exactly as many times as its count says. You may revisit nodes. Drawing an exhausted edge is a
blocked attempt (free, not counted). If you get stuck with edges still owed, you lose.

GraphQL endpoint: ${api}
A human is watching this session live: ${base}/watch/${session.id}

Session: ${session.id}
Map: ${map.name} (${map.id}) — ${map.edgeCount} edges, ${map.stepCount} traversals required.
A complete trail is exactly ${map.parMoves ?? '?'} moves long.
${startHint}

Rules of the API:
- Nodes are integers. Start with start(sessionID, node), which is not a counted move.
- Then move(sessionID, to) to draw each edge. Prefer bulkMove(sessionID, nodes: [...]) for a plan;
  it stops at the first blocked step by default, so check lastEvent.blocked afterwards.
- state.usage[i] is how many times edge i may still be drawn. state.pos is where you are.
- status WON means you covered every edge. status LOST means you are stuck; undo or reset.
- Read the map's edges first: query { map(id: "${map.id}") { nodes { id } edges { index a b count oneWay from to } } }

Steps:
1. Read the map edges.
2. Work out a route that draws every edge the right number of times.
3. mutation { start(sessionID: "${session.id}", node: <first node>) { state { pos status } } }
4. mutation { bulkMove(sessionID: "${session.id}", nodes: [<rest of route>]) {
     state { pos status moves remaining } lastEvent { blocked reason } } }
5. If blocked, read lastEvent.reason and state.pos, correct the route, and continue with bulkMove.
6. Stop when state.status is WON.

If you want a hint without changing the game:
query { hint(sessionID: "${session.id}") { start steps found } }
`;
}

/** Builds a shorter prompt for an agent that will pick its own map. */
export function buildOpenPrompt(origin: string): string {
	const base = origin.replace(/\/$/, '');
	return `Play One T Draw, a single-stroke graph puzzle, through its GraphQL API.

Endpoint: ${base}/graphql
Rules and every call: ${base}/llms.txt

1. Query { maps { id name difficulty stepCount parMoves analysis { verdict oddNodes startNodes } } }
   and pick a map (start with an easy one).
2. mutation { createSession(mapID: "<id>") { id } }
3. Follow ${base}/llms.txt: start a node, then draw every edge exactly count times.
4. Reply with the session id so a human can watch at ${base}/watch/<id>.
`;
}
