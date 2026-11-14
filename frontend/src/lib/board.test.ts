import { describe, expect, it } from 'vitest';
import {
	arrowTip,
	boardGeometry,
	edgeArrow,
	edgeLabelPoint,
	edgeStates,
	legalTargets,
	nodePoints,
	place
} from './board';
import type { GameMap, GameState } from './types';

function map(overrides: Partial<GameMap> = {}): GameMap {
	return {
		id: 't',
		name: 'T',
		description: '',
		difficulty: 'easy',
		nodeCount: 3,
		edgeCount: 3,
		stepCount: 4,
		levelCount: 1,
		maxMoves: 4,
		parMoves: 4,
		analysis: {
			nodeCount: 3,
			edgeCount: 3,
			stepCount: 4,
			connected: true,
			directional: false,
			oddNodes: [1, 2],
			startNodes: [1, 2],
			verdict: 'TRAIL',
			message: ''
		},
		nodes: [
			{ id: 1, level: 1, x: 100, y: 100 },
			{ id: 2, level: 1, x: 900, y: 100 },
			{ id: 3, level: 1, x: 500, y: 900 }
		],
		edges: [
			{ index: 0, a: 1, b: 2, count: 2, oneWay: false, from: null, to: null },
			{ index: 1, a: 2, b: 3, count: 1, oneWay: false, from: null, to: null },
			{ index: 2, a: 3, b: 1, count: 1, oneWay: false, from: null, to: null }
		],
		...overrides
	};
}

function state(overrides: Partial<GameState> = {}): GameState {
	return {
		pos: null,
		status: 'READY',
		lossReason: null,
		moves: 0,
		blocked: 0,
		remaining: 4,
		path: [],
		usage: [2, 1, 1],
		...overrides
	};
}

describe('board geometry', () => {
	it('places nodes inside the padded layout box', () => {
		const g = boardGeometry(map());
		const pts = nodePoints(g);
		expect(pts.size).toBe(3);
		for (const p of pts.values()) {
			expect(p.x).toBeGreaterThan(0);
			expect(p.x).toBeLessThan(1000);
			expect(p.y).toBeGreaterThan(0);
			expect(p.y).toBeLessThan(1000);
		}
	});

	it('maps a corner node inward by the padding', () => {
		const p = place({ id: 1, level: 1, x: 0, y: 0 });
		expect(p.x).toBe(70);
		expect(p.y).toBe(70);
	});

	it('bows doubled edges so they do not overlap', () => {
		// Edge 0 is the only edge between 1 and 2, so a plain map draws it straight.
		const single = boardGeometry(map());
		expect(single.edges[0].curved).toBe(false);

		// Two edges between the same pair must both be visible.
		const doubled = boardGeometry(
			map({
				nodeCount: 2,
				edgeCount: 2,
				edges: [
					{ index: 0, a: 1, b: 2, count: 1, oneWay: false, from: null, to: null },
					{ index: 1, a: 1, b: 2, count: 1, oneWay: false, from: null, to: null }
				]
			})
		);
		expect(doubled.edges[0].curved).toBe(true);
		expect(doubled.edges[1].curved).toBe(true);
		// Opposite sides of the straight line.
		const midY = (doubled.edges[0].from_.y + doubled.edges[0].to_.y) / 2;
		expect(Math.sign(doubled.edges[0].control.y - midY)).not.toBe(
			Math.sign(doubled.edges[1].control.y - midY)
		);
	});

	it('is deterministic: the same map yields identical geometry', () => {
		expect(JSON.stringify(boardGeometry(map()))).toBe(JSON.stringify(boardGeometry(map())));
	});

	it('survives an edge pointing at a missing node', () => {
		const g = boardGeometry(
			map({ edges: [{ index: 0, a: 1, b: 99, count: 1, oneWay: false, from: null, to: null }] })
		);
		expect(g.edges).toHaveLength(0);
	});

	it('records which edges touch each node', () => {
		const g = boardGeometry(map());
		expect(g.incident.get(1)?.sort()).toEqual([0, 2]);
		expect(g.incident.get(2)?.sort()).toEqual([0, 1]);
	});

	it('gives the arrow point a position along the drawn path', () => {
		const g = boardGeometry(map());
		const p = edgeLabelPoint(g.edges[1]);
		expect(Number.isFinite(p.x)).toBe(true);
		expect(Number.isFinite(p.y)).toBe(true);
	});
});

describe('edge usage', () => {
	it('counts a doubled edge down without marking it spent on the first draw', () => {
		const m = map();
		const usage = edgeStates(m, state({ usage: [1, 1, 1], moves: 1 }));
		expect(usage.get(0)).toEqual({ remaining: 1, drawn: 1 });
		expect(usage.get(1)).toEqual({ remaining: 1, drawn: 0 });
	});

	it('marks a fully drawn edge as spent', () => {
		const usage = edgeStates(map(), state({ usage: [0, 1, 1] }));
		expect(usage.get(0)).toEqual({ remaining: 0, drawn: 2 });
	});
});

describe('legal targets', () => {
	it('offers every node with an edge before the trail starts', () => {
		expect([...legalTargets(map(), state())].sort()).toEqual([1, 2, 3]);
	});

	it('offers only joined nodes with traversals left once the trail is under way', () => {
		// Standing on 1: edges 1-2 (index 0) and 3-1 (index 2) both still usable.
		expect([...legalTargets(map(), state({ pos: 1, status: 'PLAYING' }))].sort()).toEqual([2, 3]);
	});

	it('drops a neighbour whose edge is used up', () => {
		// Edge 0 (1-2) is spent, so only 3 remains reachable from 1.
		expect([...legalTargets(map(), state({ pos: 1, status: 'PLAYING', usage: [0, 1, 1] }))]).toEqual([3]);
	});

	it('refuses a one-way edge walked backwards', () => {
		const m = map({
			edges: [{ index: 0, a: 1, b: 2, count: 1, oneWay: true, from: 1, to: 2 }]
		});
		expect([...legalTargets(m, state({ pos: 1, status: 'PLAYING', usage: [1] }))]).toEqual([2]);
		expect([...legalTargets(m, state({ pos: 2, status: 'PLAYING', usage: [1] }))]).toEqual([]);
	});

	it('offers nothing once the game is over', () => {
		expect(legalTargets(map(), state({ pos: 1, status: 'WON' })).size).toBe(0);
		expect(legalTargets(map(), state({ pos: 1, status: 'LOST' })).size).toBe(0);
	});
});

describe('one-way arrows', () => {
	// A one-way edge whose JSON endpoints run opposite to its legal direction, as
	// happens in the bundled maps (level53 has `4-5` meaning 5 → 4). The arrow
	// must follow Direction, not the endpoint order.
	function oneWayMap(from: number, to: number): GameMap {
		const a = Math.min(from, to);
		const b = Math.max(from, to);
		return map({
			nodeCount: 2,
			edgeCount: 1,
			edges: [{ index: 0, a, b, count: 1, oneWay: true, from, to }]
		});
	}

	it('sits near the destination, not at the midpoint', () => {
		const g = boardGeometry(oneWayMap(1, 2));
		const e = g.edges[0];
		const arrow = edgeArrow(e);
		const toMid = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y);

		// Much closer to the node it points at than to the midpoint (where the
		// count label lives).
		expect(toMid(arrow, e.to_)).toBeLessThan(toMid(arrow, edgeLabelPoint(e)));
	});

	it('points from source towards destination', () => {
		// Nodes 1 and 2 are laid out left to right, so travelling 1 → 2 points right.
		const g = boardGeometry(oneWayMap(1, 2));
		const arrow = edgeArrow(g.edges[0]);
		expect(Math.abs(arrow.angle)).toBeLessThan(45);
	});

	it('flips when the legal direction is reversed', () => {
		// The same two nodes, drawn 2 → 1, must point the other way.
		const g = boardGeometry(oneWayMap(2, 1));
		const arrow = edgeArrow(g.edges[0]);
		expect(Math.abs(arrow.angle)).toBeGreaterThan(135);
	});

	it('clears the node disc however short the edge is', () => {
		const NODE_RADIUS = 26;
		const g = boardGeometry(oneWayMap(1, 2));
		const e = g.edges[0];
		const arrow = edgeArrow(e);
		const tip = arrowTip(arrow);
		const target = e.from === e.b ? e.from_ : e.to_;
		const gap = Math.hypot(tip.x - target.x, tip.y - target.y) - NODE_RADIUS;
		// The tip must stand clear of the disc, not graze its outline: a grazing
		// arrow reads as touching the node and gets lost in its stroke.
		expect(gap).toBeGreaterThanOrEqual(10);
	});

	it('reports a tip ahead of its centre, along the direction of travel', () => {
		const g = boardGeometry(oneWayMap(1, 2));
		const arrow = edgeArrow(g.edges[0]);
		const tip = arrowTip(arrow);
		// Travelling 1 -> 2 (left to right), the tip is further right than the centre.
		expect(tip.x).toBeGreaterThan(arrow.x);
	});

	it('produces a finite rotation', () => {
		for (const [from, to] of [
			[1, 2],
			[2, 1],
			[2, 3],
			[3, 1]
		]) {
			const arrow = edgeArrow(boardGeometry(oneWayMap(from, to)).edges[0]);
			expect(Number.isFinite(arrow.x)).toBe(true);
			expect(Number.isFinite(arrow.y)).toBe(true);
			expect(Number.isFinite(arrow.angle)).toBe(true);
		}
	});

	it('bows a doubled one-way edge so the twin stays visible', () => {
		// level55 has doubled one-way edges; a shared straight line would hide one.
		const m = map({
			nodeCount: 2,
			edgeCount: 2,
			edges: [
				{ index: 0, a: 1, b: 2, count: 2, oneWay: true, from: 1, to: 2 },
				{ index: 1, a: 1, b: 2, count: 2, oneWay: true, from: 2, to: 1 }
			]
		});
		const g = boardGeometry(m);
		expect(g.edges[0].curved || g.edges[1].curved).toBe(true);
		// They must sit on opposite sides, or one arrow would hide the other.
		const midY = (g.edges[0].from_.y + g.edges[0].to_.y) / 2;
		expect(Math.sign(g.edges[0].control.y - midY)).not.toBe(Math.sign(g.edges[1].control.y - midY));
	});
});
