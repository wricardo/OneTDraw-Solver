import { describe, expect, it } from 'vitest';
import { applyUpdate, mergeSession } from './session';
import type { Session, SessionUpdate } from '../types';

function session(seq: number, moves = 0): Session {
	return {
		id: 's',
		displayName: '',
		mapId: 'm',
		map: {
			id: 'm',
			name: 'M',
			description: '',
			difficulty: 'easy',
			nodeCount: 1,
			edgeCount: 1,
			stepCount: 1,
			levelCount: 1,
			maxMoves: 1,
			parMoves: 1,
			analysis: {
				nodeCount: 1,
				edgeCount: 1,
				stepCount: 1,
				connected: true,
				directional: false,
				oddNodes: [],
				startNodes: [],
				verdict: 'CIRCUIT',
				message: ''
			},
			nodes: [],
			edges: []
		},
		state: { pos: null, status: 'READY', lossReason: null, moves, blocked: 0, remaining: 1, path: [], usage: [1] },
		seq,
		createdAt: '',
		lastActionAt: '',
		attempts: 0,
		bestMoves: null,
		lastEvent: null,
		lastPlan: null,
		autoPlay: false
	};
}

describe('session merging', () => {
	it('accepts a newer snapshot', () => {
		const merged = mergeSession(session(1), session(2, 1));
		expect(merged.seq).toBe(2);
		expect(merged.state.moves).toBe(1);
	});

	it('ignores a stale snapshot so the board cannot roll back', () => {
		const merged = mergeSession(session(5, 3), session(4, 2));
		expect(merged.seq).toBe(5);
		expect(merged.state.moves).toBe(3);
	});

	it('takes the first snapshot when there is nothing to merge into', () => {
		expect(mergeSession(null, session(1)).seq).toBe(1);
	});

	it('applies a subscription update through the same rule', () => {
		const update: SessionUpdate = { seq: 2, session: session(2, 1), event: null };
		expect(applyUpdate(session(1), update).seq).toBe(2);
		// A late duplicate changes nothing.
		expect(applyUpdate(session(3, 2), update).seq).toBe(3);
	});
});
