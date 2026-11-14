// Regression cover for the one-way presentation: arrows must be visible on the
// board, point the legal way, clear the node disc, and not sit under the count
// label.
//
// It runs against the real server layout (src/test/fixtures/map-layouts.json,
// dumped from game/engine so the positions are exactly what the client
// receives). A synthetic layout would not catch the real crowding: the shipped
// maps have one-way edges as short as ~176 units, where a midpoint label and an
// arrowhead near the destination land almost on top of each other.
import { describe, expect, it } from 'vitest';
import {
	ARROW_HALF_LEN,
	arrowTip,
	boardGeometry,
	edgeArrow,
	edgeLabelPointFor,
	type DrawnEdge
} from './board';
import layouts from '../test/fixtures/map-layouts.json';
import type { GameMap } from './types';

const NODE_RADIUS = 26;

interface RawNode {
	id: number;
	level: number;
	x: number;
	y: number;
}
interface RawEdge {
	index: number;
	a: number;
	b: number;
	count: number;
	oneWay: boolean;
	from?: number;
	to?: number;
}
interface RawMap {
	id: string;
	name: string;
	nodes: RawNode[];
	edges: RawEdge[];
}

/** Shapes a dumped map like the API's GameMap, which is what the board consumes. */
function asGameMap(raw: RawMap): GameMap {
	return {
		id: raw.id,
		name: raw.name,
		description: '',
		difficulty: 'easy',
		nodeCount: raw.nodes.length,
		edgeCount: raw.edges.length,
		stepCount: raw.edges.reduce((n, e) => n + e.count, 0),
		levelCount: 1,
		maxMoves: 1,
		parMoves: null,
		analysis: {
			nodeCount: raw.nodes.length,
			edgeCount: raw.edges.length,
			stepCount: 0,
			connected: true,
			directional: true,
			oddNodes: [],
			startNodes: [],
			verdict: 'DIRECTED',
			message: ''
		},
		nodes: raw.nodes,
		edges: raw.edges.map((e) => ({
			index: e.index,
			a: e.a,
			b: e.b,
			count: e.count,
			oneWay: e.oneWay,
			from: e.from ?? null,
			to: e.to ?? null
		}))
	};
}

const maps = Object.values(layouts as Record<string, RawMap>).map(asGameMap);

const oneWayEdges = maps.flatMap((m) =>
	boardGeometry(m)
		.edges.filter((e) => e.oneWay)
		.map((e) => ({ map: m.id, edge: e }))
);

describe('one-way edges across the shipped maps', () => {
	it('finds the one-way edges it is meant to check', () => {
		expect(maps.length).toBeGreaterThan(0);
		expect(oneWayEdges.length).toBeGreaterThan(0);
	});

	it('has direction data on every one-way edge', () => {
		for (const { map, edge } of oneWayEdges) {
			expect(edge.from, `${map} edge ${edge.index} has no from`).not.toBeNull();
			expect(edge.to, `${map} edge ${edge.index} has no to`).not.toBeNull();
			// The recorded direction is one of the edge's own endpoints, either way round.
			const ends = [edge.a, edge.b].sort((x, y) => x - y);
			expect([edge.from!, edge.to!].sort((x, y) => x! - y!)).toEqual(ends);
		}
	});

	it('points every arrow at its destination, clear of the node', () => {
		for (const { map, edge } of oneWayEdges) {
			const arrow = edgeArrow(edge);
			const tip = arrowTip(arrow);
			const target = edge.from === edge.b ? edge.from_ : edge.to_;
			const dist = Math.hypot(tip.x - target.x, tip.y - target.y);
			expect(dist, `${map} edge ${edge.index} tip too close to node`).toBeGreaterThan(
				NODE_RADIUS + 8
			);
			// And it must be nearer the destination than the source.
			const source = edge.from === edge.b ? edge.to_ : edge.from_;
			expect(dist).toBeLessThan(Math.hypot(tip.x - source.x, tip.y - source.y));
		}
	});

	it('keeps the count label clear of the arrowhead', () => {
		const LABEL_HALF = 17;
		for (const { map, edge } of oneWayEdges) {
			const label = edgeLabelPointFor(edge);
			const arrow = edgeArrow(edge);
			const gap =
				Math.hypot(label.x - arrow.x, label.y - arrow.y) - LABEL_HALF - ARROW_HALF_LEN;
			expect(gap, `${map} edge ${edge.index} label/arrow gap`).toBeGreaterThan(4);
		}
	});

	it('lifts one-way labels off the edge line so they never sit on its ink', () => {
		// The label and the line are the same purple, so a label on the line reads
		// as part of it however much whitespace there is along the edge. Measure
		// the perpendicular distance from the label to the straight source-target
		// line; it must clear the stroke (7 units) comfortably.
		for (const { map, edge } of oneWayEdges) {
			if (edge.count <= 1) continue;
			const label = edgeLabelPointFor(edge);
			const a = edge.from_,
				b = edge.to_;
			const dx = b.x - a.x,
				dy = b.y - a.y;
			const len = Math.hypot(dx, dy) || 1;
			// Distance from a point to the infinite line through a and b.
			const dist = Math.abs(dy * label.x - dx * label.y + b.x * a.y - b.y * a.x) / len;
			expect(dist, `${map} edge ${edge.index} label offset from line`).toBeGreaterThan(20);
		}
	});

	it('never places a label on a node disc', () => {
		for (const { map, edge } of oneWayEdges) {
			if (edge.count <= 1) continue; // no label drawn
			const label = edgeLabelPointFor(edge);
			for (const node of [edge.from_, edge.to_]) {
				expect(
					Math.hypot(label.x - node.x, label.y - node.y),
					`${map} edge ${edge.index} label sits on a node`
				).toBeGreaterThan(NODE_RADIUS);
			}
		}
	});

	it('gives the arrow a finite, non-degenerate rotation', () => {
		for (const { map, edge } of oneWayEdges) {
			const arrow = edgeArrow(edge);
			expect(Number.isFinite(arrow.angle), `${map} edge ${edge.index}`).toBe(true);
			expect(Number.isFinite(arrow.x)).toBe(true);
			expect(Number.isFinite(arrow.y)).toBe(true);
		}
	});

	it('exposes the arrow half-length so callers can reason about clearance', () => {
		expect(ARROW_HALF_LEN).toBeGreaterThan(0);
	});
});

describe('undirected edges get no arrow', () => {
	it('marks no undirected edge as one-way', () => {
		for (const m of maps) {
			for (const e of boardGeometry(m).edges) {
				if (e.oneWay) continue;
				expect(e.from).toBeNull();
				expect(e.to).toBeNull();
			}
		}
	});
});

// Keep the DrawnEdge type referenced so a refactor of the geometry shape fails
// here rather than silently changing what the components receive.
const _typecheck: DrawnEdge | null = null;
void _typecheck;
