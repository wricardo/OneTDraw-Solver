import { describe, expect, it } from 'vitest';
import { describeEvent, progressPercent, statusLabel, verdictText } from './hud';
import type { Analysis, GameState, StepEvent } from './types';

function state(overrides: Partial<GameState> = {}): GameState {
	return {
		pos: 1,
		status: 'PLAYING',
		lossReason: null,
		moves: 0,
		blocked: 0,
		remaining: 4,
		path: [1],
		usage: [1],
		...overrides
	};
}

function ev(overrides: Partial<StepEvent> = {}): StepEvent {
	return {
		move: 1,
		node: 2,
		from: 1,
		to: 2,
		kind: 'DRAW',
		edge: 0,
		blocked: false,
		reason: null,
		status: 'PLAYING',
		lossReason: null,
		...overrides
	};
}

function analysis(overrides: Partial<Analysis> = {}): Analysis {
	return {
		nodeCount: 3,
		edgeCount: 3,
		stepCount: 3,
		connected: true,
		directional: false,
		oddNodes: [],
		startNodes: [],
		verdict: 'CIRCUIT',
		message: '',
		...overrides
	};
}

describe('event description', () => {
	it('explains a fresh game', () => {
		expect(describeEvent(null)).toMatch(/Pick a node/);
	});

	it('names the node a trail started on', () => {
		expect(describeEvent(ev({ kind: 'START', from: null, to: 4, node: 4 }))).toMatch(/started at node 4/);
	});

	it('reports a drawn edge', () => {
		expect(describeEvent(ev())).toMatch(/node 1 to node 2/);
	});

	it('surfaces why a move was blocked', () => {
		const text = describeEvent(ev({ blocked: true, kind: 'NONE', reason: 'that edge is already fully drawn' }));
		expect(text).toMatch(/Blocked/);
		expect(text).toMatch(/already fully drawn/);
	});
});

describe('status and progress', () => {
	it('labels each status', () => {
		expect(statusLabel(state({ status: 'READY' }))).toMatch(/Ready/);
		expect(statusLabel(state({ status: 'WON' }))).toMatch(/solved/i);
		expect(statusLabel(state({ status: 'LOST' }))).toMatch(/Stuck/);
	});

	it('computes the drawn fraction', () => {
		expect(progressPercent(state({ moves: 1, remaining: 3 }))).toBe(25);
		expect(progressPercent(state({ moves: 0, remaining: 0 }))).toBe(0);
	});
});

describe('verdict text', () => {
	it('explains a closed tour', () => {
		expect(verdictText(analysis())).toMatch(/any node/);
	});

	it('names the required start nodes for an open trail', () => {
		const a = analysis({ verdict: 'TRAIL', oddNodes: [4, 5] });
		expect(verdictText(a)).toMatch(/4 or 5/);
	});

	it('says an impossible map cannot be drawn', () => {
		const a = analysis({ verdict: 'IMPOSSIBLE', message: 'impossible: 4 odd-degree nodes' });
		expect(verdictText(a)).toMatch(/4 odd-degree/);
	});
});
