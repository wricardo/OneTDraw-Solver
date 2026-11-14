// Human-facing text for game concepts: what a step did, what a verdict means,
// how a node is referred to. Kept apart from the components so the wording can
// be tested directly.

import type { Analysis, GameMap, GameState, Session, StepEvent } from './types';

/** Sentence describing one attempted step. */
export function describeEvent(ev: StepEvent | null): string {
	if (!ev) return 'No move yet. Pick a node to start the trail.';
	if (ev.blocked) {
		const reason = ev.reason ?? 'that step is not allowed';
		return `Blocked: ${reason}. Nothing was drawn.`;
	}
	switch (ev.kind) {
		case 'START':
			return `Trail started at node ${ev.node}.`;
		case 'DRAW':
			return `Drew node ${ev.from} to node ${ev.to}${ev.edge !== null ? ` (edge ${ev.edge})` : ''}.`;
		default:
			return '';
	}
}

/** Short label for the current status. */
export function statusLabel(state: GameState): string {
	switch (state.status) {
		case 'READY':
			return 'Ready to start';
		case 'PLAYING':
			return 'Drawing';
		case 'WON':
			return 'Every edge drawn — solved';
		case 'LOST':
			return 'Stuck: edges remain';
	}
}

/** Explains a verdict in one sentence a player can act on. */
export function verdictText(a: Analysis): string {
	switch (a.verdict) {
		case 'CIRCUIT':
			return 'A closed tour exists: any node may start.';
		case 'TRAIL':
			return `An open trail exists: it must start at ${a.oddNodes.join(' or ')}.`;
		case 'DIRECTED':
			return 'One-way edges decide the start: any node with an edge may work.';
		case 'IMPOSSIBLE':
			return a.message || 'No single trail can cover every edge.';
	}
}

/** How much of the map is still to draw, as a percentage already drawn. */
export function progressPercent(state: GameState): number {
	const total = state.moves + state.remaining;
	if (total <= 0) return 0;
	return Math.round((state.moves / total) * 100);
}

/** A one-line summary of where the game stands, for a live region. */
export function sessionSummary(s: Session): string {
	const st = s.state;
	if (st.status === 'READY') return `${s.map.name}: ready to start.`;
	if (st.status === 'WON') return `${s.map.name}: solved in ${st.moves} moves.`;
	if (st.status === 'LOST') return `${s.map.name}: stuck with ${st.remaining} traversals left.`;
	const pct = progressPercent(st);
	return `${s.map.name}: ${st.moves} of ${st.moves + st.remaining} drawn (${pct}%), at node ${st.pos}.`;
}
