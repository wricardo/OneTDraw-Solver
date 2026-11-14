import { describe, expect, it } from 'vitest';
import {
	beginDrag,
	DRAG_THRESHOLD_PX,
	isClick,
	movedPosition,
	screenToUserScale,
	updateDrag,
	withPosition
} from './drag';
import { boardGeometry, clampToLayout, LAYOUT_SIZE, place } from './board';
import type { GameMap } from './types';

function map(): GameMap {
	return {
		id: 't',
		name: 'T',
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
			oddNodes: [1, 2],
			startNodes: [1, 2],
			verdict: 'TRAIL',
			message: ''
		},
		nodes: [
			{ id: 1, level: 1, x: 100, y: 100 },
			{ id: 2, level: 1, x: 900, y: 900 }
		],
		edges: [{ index: 0, a: 1, b: 2, count: 1, oneWay: false, from: null, to: null }]
	};
}

describe('click versus drag', () => {
	const origin = { x: 100, y: 100 };
	const down = { x: 500, y: 400 };

	it('treats a press with no movement as a click', () => {
		const d = beginDrag(1, origin, down);
		expect(isClick(d)).toBe(true);
		const { drag, position } = updateDrag(d, down, 1);
		expect(position).toBeNull();
		expect(isClick(drag)).toBe(true);
	});

	it('tolerates a hand tremor below the threshold', () => {
		// A click that wobbles a pixel or two is still a click: otherwise picking
		// a node would be nearly impossible on a touch screen.
		const d = beginDrag(1, origin, down);
		const nudged = { x: down.x + DRAG_THRESHOLD_PX - 1, y: down.y };
		const { drag, position } = updateDrag(d, nudged, 1);
		expect(drag.active).toBe(false);
		expect(position).toBeNull();
		expect(isClick(drag)).toBe(true);
	});

	it('becomes a drag exactly at the threshold', () => {
		const d = beginDrag(1, origin, down);
		const moved = { x: down.x + DRAG_THRESHOLD_PX, y: down.y };
		const { drag, position } = updateDrag(d, moved, 1);
		expect(drag.active).toBe(true);
		expect(position).not.toBeNull();
		expect(isClick(drag)).toBe(false);
	});

	it('stays a drag even if the pointer returns to the start', () => {
		// Otherwise a drag that circles back would fire a pick on release, moving
		// the pen because the player tidied up the layout.
		const d = beginDrag(1, origin, down);
		const far = { x: down.x + 60, y: down.y + 60 };
		const started = updateDrag(d, far, 1).drag;
		expect(started.active).toBe(true);
		const back = updateDrag(started, down, 1);
		expect(back.drag.active).toBe(true);
		expect(isClick(back.drag)).toBe(false);
	});

	it('reports no click for a null drag', () => {
		expect(isClick(null)).toBe(false);
	});
});

describe('drag movement', () => {
	it('offsets the node by the pointer delta', () => {
		const p = movedPosition({ x: 300, y: 300 }, { x: 40, y: -25 });
		expect(p).toEqual({ x: 340, y: 275 });
	});

	it('clamps so a node cannot be lost off the canvas', () => {
		const far = movedPosition({ x: 500, y: 500 }, { x: 100000, y: 100000 });
		expect(far.x).toBeLessThan(LAYOUT_SIZE);
		expect(far.y).toBeLessThan(LAYOUT_SIZE);
		const negative = movedPosition({ x: 500, y: 500 }, { x: -100000, y: -100000 });
		expect(negative.x).toBeGreaterThan(0);
		expect(negative.y).toBeGreaterThan(0);
	});

	it('leaves a position already inside the box untouched', () => {
		expect(clampToLayout({ x: 400, y: 600 })).toEqual({ x: 400, y: 600 });
	});

	it('scales client pixels into layout units', () => {
		// A node 2x downscaled on screen must move twice as far in layout space to
		// keep up with the pointer.
		const d = beginDrag(1, { x: 300, y: 300 }, { x: 0, y: 0 });
		const { position } = updateDrag(d, { x: 40, y: 0 }, 2);
		expect(position).toEqual({ x: 380, y: 300 });
	});
});

describe('positions', () => {
	it('adds a moved node without touching the rest', () => {
		const original = new Map([[1, { x: 10, y: 10 }]]);
		const next = withPosition(original, 2, { x: 20, y: 20 });
		expect(next.get(1)).toEqual({ x: 10, y: 10 });
		expect(next.get(2)).toEqual({ x: 20, y: 20 });
		// The input map is not mutated, so Svelte sees a new reference.
		expect(original.has(2)).toBe(false);
		expect(next).not.toBe(original);
	});
});

describe('geometry honours dragged positions', () => {
	it('draws a node where it was dragged to', () => {
		const m = map();
		const moved = new Map([[1, { x: 400, y: 400 }]]);
		const g = boardGeometry(m, moved);
		expect(g.points.get(1)).toEqual({ x: 400, y: 400 });
		// Node 2 has no override, so it keeps its layout position.
		expect(g.points.get(2)).toEqual(place(m.nodes[1]));
	});

	it('moves the edges with their nodes', () => {
		const m = map();
		const base = boardGeometry(m);
		const moved = new Map([[1, { x: 800, y: 200 }]]);
		const after = boardGeometry(m, moved);
		expect(after.edges[0].from_).toEqual({ x: 800, y: 200 });
		expect(after.edges[0].from_).not.toEqual(base.edges[0].from_);
		// The other endpoint is unchanged.
		expect(after.edges[0].to_).toEqual(base.edges[0].to_);
	});

	it('recomputes the path, so edges stay attached', () => {
		const m = map();
		const after = boardGeometry(m, new Map([[1, { x: 500, y: 120 }]]));
		expect(after.edges[0].path).toContain('500');
	});

	it('is identical to the undragged layout when given no overrides', () => {
		const m = map();
		expect(JSON.stringify(boardGeometry(m, null))).toBe(JSON.stringify(boardGeometry(m)));
		expect(JSON.stringify(boardGeometry(m, new Map()))).toBe(JSON.stringify(boardGeometry(m)));
	});

	it('carries overrides for unknown node ids harmlessly', () => {
		const m = map();
		const g = boardGeometry(m, new Map([[999, { x: 1, y: 1 }]]));
		expect(g.points.size).toBe(2);
		expect(g.points.has(999)).toBe(false);
	});
});

describe('screenToUserScale', () => {
	it('falls back to 1 without an element or CTM', () => {
		expect(screenToUserScale(null)).toBe(1);
	});
});
