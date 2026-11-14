// Board geometry: turning a map's nodes and edges into drawable shapes.
//
// The server sends node positions in a normalized 0..1000 layout box and edges
// as node-id pairs. Everything a renderer needs — where to stroke an edge, how
// far to bow a doubled edge so it does not hide its twin, which node a click
// landed on — is derived here, so the SVG component stays pure presentation.

import type { Edge, GameMap, GameState, Node } from './types';

/** The layout box the server uses for node coordinates. */
export const LAYOUT_SIZE = 1000;

/** Padding inside the viewBox, in layout units, so strokes are not clipped. */
const PADDING = 70;

export interface Point {
	x: number;
	y: number;
}

/** An edge ready to stroke, with its bow and its own id. */
export interface DrawnEdge {
	index: number;
	a: number;
	b: number;
	count: number;
	oneWay: boolean;
	from: number | null;
	to: number | null;
	/** The straight-line endpoints. */
	from_: Point;
	to_: Point;
	/** Quadratic control point; equals the midpoint when the edge is straight. */
	control: Point;
	/** SVG path data for the edge, including any bow. */
	path: string;
	/** True when the edge curves, so a direction arrow has somewhere to sit. */
	curved: boolean;
}

export interface BoardGeometry {
	width: number;
	height: number;
	/** Padding-adjusted viewBox. */
	viewBox: string;
	nodes: Node[];
	byId: Map<number, Node>;
	edges: DrawnEdge[];
	/** Node id -> indices of the edges touching it. */
	incident: Map<number, number[]>;
	/** Node id -> its final drawn position, after any dragged overrides. */
	points: Map<number, Point>;
}

/**
 * Node positions the player has dragged, in the board's own display space
 * (i.e. what `place` returns). Presentation only: the server never sees these,
 * and no game rule depends on them.
 */
export type Positions = ReadonlyMap<number, Point>;

/** Applies the layout padding, mapping the normalized box into the viewBox. */
export function place(n: Node): Point {
	const span = LAYOUT_SIZE - PADDING * 2;
	return {
		x: PADDING + (n.x / LAYOUT_SIZE) * span,
		y: PADDING + (n.y / LAYOUT_SIZE) * span
	};
}

/**
 * Keeps a point inside the drawable box.
 *
 * Dragging must not push a node off the canvas, where it could no longer be
 * grabbed again. The margin leaves room for the node disc plus its stroke.
 */
export function clampToLayout(p: Point, radius = 30): Point {
	const lo = PADDING - 20 + radius;
	const hi = LAYOUT_SIZE - PADDING + 20 - radius;
	return {
		x: Math.min(hi, Math.max(lo, p.x)),
		y: Math.min(hi, Math.max(lo, p.y))
	};
}

/** Builds the printable geometry for a map, honouring any dragged positions. */
export function boardGeometry(map: GameMap, positions?: Positions | null): BoardGeometry {
	const byId = new Map<number, Node>();
	for (const n of map.nodes) byId.set(n.id, n);

	// Bow every edge that shares endpoints with another edge (a doubled edge) so
	// both are visible, and give every edge a slight consistent curvature when it
	// is one of several between the same pair.
	const pairCount = new Map<string, number>();
	const pairSeen = new Map<string, number>();
	for (const e of map.edges) {
		const key = `${Math.min(e.a, e.b)}:${Math.max(e.a, e.b)}`;
		pairCount.set(key, (pairCount.get(key) ?? 0) + 1);
	}

	const edges: DrawnEdge[] = [];
	const incident = new Map<number, number[]>();
	const points = new Map<number, Point>();

	// Position lookup: a dragged override wins, otherwise the stored layout.
	const posOf = (n: Node): Point => positions?.get(n.id) ?? place(n);

	for (const e of map.edges) {
		const na = byId.get(e.a);
		const nb = byId.get(e.b);
		if (!na || !nb) continue; // defensive: a malformed map cannot crash the board
		const from_ = posOf(na);
		const to_ = posOf(nb);

		const key = `${Math.min(e.a, e.b)}:${Math.max(e.a, e.b)}`;
		const total = pairCount.get(key) ?? 1;
		const seen = pairSeen.get(key) ?? 0;
		pairSeen.set(key, seen + 1);

		let bow = 0;
		if (total > 1) {
			// Spread the group symmetrically around the straight line.
			const offset = (seen - (total - 1) / 2) * 2;
			bow = offset * 26;
			if (bow === 0) bow = 26; // a single straight twin would hide the pair
		}
		// One-way edges are drawn straight: their arrowhead sits just before the
		// destination, clear of the count label at the midpoint, so it needs no
		// bow to keep out of the way.

		const mid = { x: (from_.x + to_.x) / 2, y: (from_.y + to_.y) / 2 };
		const dx = to_.x - from_.x;
		const dy = to_.y - from_.y;
		const len = Math.hypot(dx, dy) || 1;
		// Perpendicular in screen space; bow is applied along it.
		const control = { x: mid.x + (-dy / len) * bow, y: mid.y + (dx / len) * bow };
		const path = bowToPath(from_, control, to_);

		edges.push({
			index: e.index,
			a: e.a,
			b: e.b,
			count: e.count,
			oneWay: e.oneWay,
			from: e.from,
			to: e.to,
			from_,
			to_,
			control,
			path,
			curved: bow !== 0
		});
		for (const id of [e.a, e.b]) {
			const list = incident.get(id);
			if (list) list.push(e.index);
			else incident.set(id, [e.index]);
		}
	}

	for (const n of map.nodes) points.set(n.id, posOf(n));

	return {
		width: LAYOUT_SIZE,
		height: LAYOUT_SIZE,
		// The viewBox is square and the layout already includes padding, so the
		// SVG just spans the whole normalized box.
		viewBox: `0 0 ${LAYOUT_SIZE} ${LAYOUT_SIZE}`,
		nodes: map.nodes,
		byId,
		edges,
		incident,
		points
	};
}

/** Path data for a straight or bowed edge. */
function bowToPath(from: Point, control: Point, to: Point): string {
	const straight =
		Math.abs(control.x - (from.x + to.x) / 2) < 0.01 && Math.abs(control.y - (from.y + to.y) / 2) < 0.01;
	if (straight) return `M ${round(from.x)} ${round(from.y)} L ${round(to.x)} ${round(to.y)}`;
	return `M ${round(from.x)} ${round(from.y)} Q ${round(control.x)} ${round(control.y)} ${round(to.x)} ${round(to.y)}`;
}

function round(v: number): number {
	return Math.round(v * 10) / 10;
}

/** A point on an edge's quadratic Bézier at parameter t in [0,1]. */
function curvePoint(e: DrawnEdge, t: number): Point {
	const u = 1 - t;
	return {
		x: u * u * e.from_.x + 2 * u * t * e.control.x + t * t * e.to_.x,
		y: u * u * e.from_.y + 2 * u * t * e.control.y + t * t * e.to_.y
	};
}

/** The point half-way along an edge's drawn path, where its count label sits. */
export function edgeLabelPoint(e: DrawnEdge): Point {
	return curvePoint(e, 0.5);
}

/**
 * Where to put an edge's count label, keeping clear of its arrowhead.
 *
 * On a plain edge the midpoint is the natural home. On a one-way edge that does
 * not work: the arrowhead claims the run up to the destination, and the edge
 * line itself is the same colour as the label, so even a few units of real
 * whitespace still reads as one continuous glyph (the shortest shipped one-way
 * edge, littlet 11-12, is only 176 units long). Moving the label along the line
 * therefore cannot win.
 *
 * Instead the label is lifted off the line, perpendicular to travel, beside a
 * point safely behind the arrowhead. Sideways separation is unambiguous at any
 * edge length, and it also keeps the label off the line's own ink.
 */
export function edgeLabelPointFor(e: DrawnEdge): Point {
	if (!e.oneWay) return curvePoint(e, 0.5);

	const reversed = e.from !== null && e.to !== null && e.from === e.b && e.to === e.a;
	const tOf = (s: number) => (reversed ? 1 - s : s);
	const arrow = edgeArrow(e);

	// A point a fixed distance behind the arrowhead, along travel.
	const anchor = (() => {
		for (const s of [0.5, 0.44, 0.38, 0.32, 0.26]) {
			const p = curvePoint(e, tOf(s));
			if (Math.hypot(p.x - arrow.x, p.y - arrow.y) >= LABEL_ARROW_GAP) return p;
		}
		return curvePoint(e, tOf(0.26));
	})();

	// Push it off the line. The side is chosen consistently so a map does not
	// flip labels between renders: perpendicular to the direction of travel.
	const source = reversed ? e.to_ : e.from_;
	const target = reversed ? e.from_ : e.to_;
	let dx = target.x - source.x;
	let dy = target.y - source.y;
	const len = Math.hypot(dx, dy) || 1;
	dx /= len;
	dy /= len;
	// Screen-space perpendicular. Keeping the y term positive on a horizontal
	// edge puts the label below the line, clear of a count label on a twin edge.
	let nx = -dy;
	let ny = dx;
	if (ny < 0 || (ny === 0 && nx < 0)) {
		nx = -nx;
		ny = -ny;
	}
	return { x: anchor.x + nx * LABEL_SIDE_OFFSET, y: anchor.y + ny * LABEL_SIDE_OFFSET };
}

/** Distance a one-way count label is lifted off its edge, in layout units. */
export const LABEL_SIDE_OFFSET = 34;

/** Clearance kept along the edge between a count label's anchor and the arrowhead. */
export const LABEL_ARROW_GAP = 46;

export interface EdgeArrow {
	/** Centre of the arrowhead. */
	x: number;
	y: number;
	/** Rotation in degrees, pointing along the legal direction of travel. */
	angle: number;
}

/** Half-length of the arrowhead along its own axis, in layout units. */
export const ARROW_HALF_LEN = 20;

/**
 * Where to draw a one-way edge's arrowhead, and which way it points.
 *
 * The head sits a short way before the destination node rather than at the
 * midpoint, for two reasons: the count label already owns the midpoint (most
 * one-way edges in the bundled maps are doubled, so an arrow placed there would
 * collide with the number), and an arrow close to the target node reads as "you
 * arrive here", which is what the constraint actually means.
 *
 * `backOff` is the distance from the destination node's CENTRE to the arrow's
 * TIP, so callers state the clearance they want directly. 38 puts the tip 12
 * units clear of a 26-unit node disc: enough whitespace to read as separate ink,
 * and tight enough that the arrow still claims the run up to the node. Being
 * generous here is not free — on a 176-unit edge (littlet 11-12) a larger
 * back-off pushes the head into the count label.
 */
export function edgeArrow(e: DrawnEdge, backOff = 38): EdgeArrow {
	// Travel runs from -> to, which may be either way round relative to the a->b
	// parameterisation of the curve.
	const reversed = e.from !== null && e.to !== null && e.from === e.b && e.to === e.a;
	const target = reversed ? e.from_ : e.to_;

	// Map a fraction of travel (0 = origin, 1 = target) onto the curve.
	const tOf = (s: number) => (reversed ? 1 - s : s);

	// Bisect for the travel fraction whose curve point is `backOff` from the
	// target. The distance decreases monotonically over this range, so plain
	// bisection converges.
	let lo = 0.2;
	let hi = 1;
	for (let i = 0; i < 24; i++) {
		const mid = (lo + hi) / 2;
		const p = curvePoint(e, tOf(mid));
		if (Math.hypot(p.x - target.x, p.y - target.y) > backOff) lo = mid;
		else hi = mid;
	}
	const t = tOf((lo + hi) / 2);
	const u = 1 - t;

	// Tangent of the curve there, oriented along travel.
	let dx = 2 * u * (e.control.x - e.from_.x) + 2 * t * (e.to_.x - e.control.x);
	let dy = 2 * u * (e.control.y - e.from_.y) + 2 * t * (e.to_.y - e.control.y);
	if (reversed) {
		dx = -dx;
		dy = -dy;
	}
	// A zero tangent cannot occur on a validated map (nodes are distinct), but
	// pointing right is a harmless fallback rather than a NaN rotation.
	const angle = dx === 0 && dy === 0 ? 0 : (Math.atan2(dy, dx) * 180) / Math.PI;

	// `t` is where the TIP goes; step back along the tangent to centre the head.
	const tip = curvePoint(e, t);
	const rad = (angle * Math.PI) / 180;
	return {
		x: tip.x - ARROW_HALF_LEN * Math.cos(rad),
		y: tip.y - ARROW_HALF_LEN * Math.sin(rad),
		angle
	};
}

/** The arrowhead's tip position, derived from its centre. */
export function arrowTip(a: EdgeArrow): Point {
	const rad = (a.angle * Math.PI) / 180;
	return { x: a.x + ARROW_HALF_LEN * Math.cos(rad), y: a.y + ARROW_HALF_LEN * Math.sin(rad) };
}

/** The drawn node positions, for quick hit testing. */
export function nodePoints(g: BoardGeometry): Map<number, Point> {
	const out = new Map<number, Point>();
	for (const n of g.nodes) out.set(n.id, place(n));
	return out;
}

export interface EdgeState {
	/** Traversals still owed. */
	remaining: number;
	/** Traversals already drawn. */
	drawn: number;
}

/** Per-edge usage for the current game state, keyed by edge index. */
export function edgeStates(map: GameMap, state: GameState): Map<number, EdgeState> {
	const out = new Map<number, EdgeState>();
	for (const e of map.edges) {
		const remaining = state.usage[e.index] ?? e.count;
		out.set(e.index, { remaining, drawn: e.count - remaining });
	}
	return out;
}

/** Nodes the player may legally step to right now. */
export function legalTargets(map: GameMap, state: GameState): Set<number> {
	const out = new Set<number>();
	if (state.status !== 'PLAYING' && state.status !== 'READY') return out;
	const here = state.pos;
	const nodesWithEdges = new Set<number>();
	for (const e of map.edges) {
		nodesWithEdges.add(e.a);
		nodesWithEdges.add(e.b);
	}
	if (here === null) {
		// READY: any node with an edge can be the first pick.
		return nodesWithEdges;
	}
	for (const e of map.edges) {
		if ((state.usage[e.index] ?? 0) <= 0) continue;
		if (e.oneWay && e.from !== here) continue;
		if (e.a === here) out.add(e.b);
		else if (e.b === here) out.add(e.a);
	}
	return out;
}

/** Short description of one node, for accessible names and tooltips. */
export function nodeLabel(n: Node): string {
	return `node ${n.id}`;
}
