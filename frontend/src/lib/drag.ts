// Dragging nodes around the board.
//
// Node positions are presentation only: the server stores a layout hint and no
// game rule depends on where a node is drawn. So dragging is a purely local
// rearrangement — it never touches game state, never calls the API, and cannot
// make an unsolvable position solvable.
//
// The hard part is not the movement, it is telling a drag from a click: clicking
// a node is how you play the game, so a drag must never be mistaken for a pick,
// and a pick must never be swallowed by a tiny hand tremor. Both are decided
// here, in one place, so the board component stays about drawing.

import { clampToLayout, type Point, type Positions } from './board';

/** How far the pointer must travel before a press becomes a drag, in px. */
export const DRAG_THRESHOLD_PX = 4;

/**
 * Where a node sits after a pointer moves `delta` (in SVG user units) from the
 * position it was picked up at. Clamped so it cannot be dragged off the canvas.
 */
export function movedPosition(origin: Point, delta: Point): Point {
	return clampToLayout({ x: origin.x + delta.x, y: origin.y + delta.y });
}

/** Returns a copy of positions with id moved to p. */
export function withPosition(positions: Positions, id: number, p: Point): Map<number, Point> {
	const next = new Map(positions);
	next.set(id, p);
	return next;
}

/** Converts a screen-space delta into the SVG's user-space delta. */
export function screenToUserScale(svg: SVGSVGElement | null): number {
	if (!svg) return 1;
	const ctm = svg.getScreenCTM();
	if (!ctm) return 1;
	// The mean of the x and y scale factors: the board keeps a square aspect, so
	// they are equal, and averaging avoids a divide-by-zero on a degenerate CTM.
	const sx = Math.hypot(ctm.a, ctm.b);
	const sy = Math.hypot(ctm.c, ctm.d);
	const s = (sx + sy) / 2;
	return s > 0 ? 1 / s : 1;
}

export interface DragState {
	/** The node being dragged. */
	id: number;
	/** Pointer position when the press started, in client coordinates. */
	startClient: Point;
	/** The node's position when the press started, in layout space. */
	startLayout: Point;
	/** True once the pointer has travelled past the threshold. */
	active: boolean;
}

/**
 * Starts a drag. It is not yet `active`: only movement past
 * DRAG_THRESHOLD_PX turns a press into a real drag, so a click that wobbles by a
 * pixel still picks the node instead of nudging it.
 */
export function beginDrag(id: number, startLayout: Point, client: Point): DragState {
	return { id, startClient: { ...client }, startLayout: { ...startLayout }, active: false };
}

/**
 * Advances a drag to the current pointer position, returning the new state and
 * the node's new layout position.
 *
 * `scale` converts client pixels to layout units (see screenToUserScale). Once
 * the drag is active it stays active even if the pointer returns near the start,
 * so a drag can never turn back into a click halfway through.
 */
export function updateDrag(
	drag: DragState,
	client: Point,
	scale: number
): { drag: DragState; position: Point | null } {
	const dx = client.x - drag.startClient.x;
	const dy = client.y - drag.startClient.y;
	const active = drag.active || Math.hypot(dx, dy) >= DRAG_THRESHOLD_PX;
	if (!active) return { drag: { ...drag, active: false }, position: null };

	const position = movedPosition(drag.startLayout, { x: dx * scale, y: dy * scale });
	return { drag: { ...drag, active: true }, position };
}

/** True when a finished press should count as a click on the node, not a drag. */
export function isClick(drag: DragState | null): boolean {
	return !!drag && !drag.active;
}
