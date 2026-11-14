import { describe, expect, it } from 'vitest';
import { createReplay } from './replay';
import type { Session } from './types';

function session(path: number[]): Session {
	return {
		id: 's',
		displayName: '',
		mapId: 'm',
		map: {
			id: 'm',
			name: 'M',
			description: '',
			difficulty: 'easy',
			nodeCount: 2,
			edgeCount: 1,
			stepCount: 1,
			levelCount: 1,
			maxMoves: 1,
			parMoves: 1,
			analysis: {
				nodeCount: 2,
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
		state: {
			pos: path[path.length - 1] ?? null,
			status: 'PLAYING',
			lossReason: null,
			moves: Math.max(0, path.length - 1),
			blocked: 0,
			remaining: 1,
			path,
			usage: [1]
		},
		seq: path.length,
		createdAt: '',
		lastActionAt: '',
		attempts: 0,
		bestMoves: null,
		lastEvent: null,
		lastPlan: null,
		autoPlay: false
	};
}

describe('replay', () => {
	it('shows a first snapshot in full rather than animating history', () => {
		const r = createReplay();
		r.sync(session([1, 2, 3, 4]));
		const frame = r.frame();
		expect(frame?.session.state.path).toEqual([1, 2, 3, 4]);
		expect(frame?.replaying).toBe(false);
	});

	it('jumps straight to the truth when the trail shrinks (undo or reset)', () => {
		const r = createReplay();
		r.sync(session([1, 2, 3, 4]));
		r.frame();
		r.sync(session([1, 2]));
		const frame = r.frame();
		expect(frame?.session.state.path).toEqual([1, 2]);
		expect(frame?.replaying).toBe(false);
	});

	it('animates a newly appended tail', () => {
		const r = createReplay();
		r.sync(session([1, 2]));
		r.frame();
		// A bulkMove appends three nodes at once.
		r.sync(session([1, 2, 3, 4, 5]));
		const first = r.frame();
		expect(first?.replaying).toBe(true);
		// Never shows more than the authoritative trail.
		expect(first!.session.state.path.length).toBeLessThanOrEqual(5);
	});

	it('reports nothing before the first sync', () => {
		expect(createReplay().frame()).toBeNull();
	});

	it('forgets its session on destroy', () => {
		const r = createReplay();
		r.sync(session([1, 2]));
		r.destroy();
		expect(r.frame()).toBeNull();
	});
});
