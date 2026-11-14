<script lang="ts">
	import {
		boardGeometry,
		clampToLayout,
		edgeArrow,
		edgeLabelPointFor,
		edgeStates,
		legalTargets,
		nodeLabel,
		type Positions,
		type DrawnEdge
	} from '$lib/board';
	import { beginDrag, isClick, screenToUserScale, updateDrag, withPosition, type DragState } from '$lib/drag';
	import type { GameMap, GameState, StepEvent } from '$lib/types';

	let {
		map,
		// Named `game`, not `state`: a prop called `state` makes every `$state(...)`
		// rune in this file read as a store subscription on it.
		game,
		lastEvent = null,
		bumpKey = 0,
		onpick,
		showSolution = null,
		interactive = true,
		draggable = false
	}: {
		map: GameMap;
		game: GameState;
		lastEvent?: StepEvent | null;
		/** Changes on every server update so a repeated blocked move re-animates. */
		bumpKey?: number;
		/** Called with a node id when the player picks one. */
		onpick?: (node: number) => void;
		/** A full solution trail to overlay, when the solver has produced one. */
		showSolution?: number[] | null;
		interactive?: boolean;
		/**
		 * Allow nodes to be dragged into a tidier arrangement. This changes only
		 * what is drawn: positions never leave the browser and no game rule
		 * depends on them.
		 */
		draggable?: boolean;
	} = $props();

	/** Layout overrides from dragging, keyed by node id. Resets with the map. */
	let dragged = $state<Positions>(new Map());
	let svgEl = $state<SVGSVGElement | null>(null);
	// Named `dragState`, not `drag`: a bare name near the `state` prop risks
	// reading like a rune or a store subscription.
	let dragState = $state<DragState | null>(null);
	/** The node currently being dragged, for styling while it moves. */
	let draggingId = $state<number | null>(null);

	// A different map means the overrides belong to a board that is gone.
	let lastMapId = '';
	$effect(() => {
		const id = map.id;
		if (id !== lastMapId) {
			lastMapId = id;
			dragged = new Map();
			draggingId = null;
			dragState = null;
		}
	});

	const geometry = $derived(boardGeometry(map, dragged));
	const usage = $derived(edgeStates(map, game));
	const legal = $derived(legalTargets(map, game));
	const here = $derived(game.pos);
	const startNode = $derived(game.path.length > 0 ? game.path[0] : null);

	/** The trail segments drawn so far, as node-id pairs. */
	const drawnSegments = $derived.by(() => {
		const out: { a: number; b: number }[] = [];
		for (let i = 0; i + 1 < game.path.length; i++) out.push({ a: game.path[i], b: game.path[i + 1] });
		return out;
	});

	/** The solution overlay segments, when a plan is shown and not yet drawn. */
	const solutionSegments = $derived.by(() => {
		if (!showSolution || showSolution.length < 2) return [];
		const out: { a: number; b: number }[] = [];
		for (let i = 0; i + 1 < showSolution.length; i++) out.push({ a: showSolution[i], b: showSolution[i + 1] });
		return out;
	});

	/**
	 * Badge positions for the solution overlay, one per step.
	 *
	 * The midpoint is the natural spot, but two segments often share one — the
	 * diagonals of a square cross exactly at the centre — so their badges would
	 * stack and a number would vanish. Each badge is therefore placed at a
	 * fraction along its segment, and any badge still within a badge's diameter
	 * of an earlier one is nudged to the next fraction until it is clear.
	 */
	const solutionBadges = $derived.by(() => {
		const BADGE = 66; // 2 * r, in layout units
		const fractions = [0.5, 0.32, 0.68, 0.22, 0.78];
		const placed: { x: number; y: number }[] = [];
		const out: { x: number; y: number }[] = [];
		for (const seg of solutionSegments) {
			const a = pointFor(seg.a);
			const b = pointFor(seg.b);
			let spot = { x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 };
			for (const t of fractions) {
				const candidate = { x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t };
				const collides = placed.some(
					(p) => Math.hypot(p.x - candidate.x, p.y - candidate.y) < BADGE
				);
				spot = candidate;
				if (!collides) break;
			}
			placed.push(spot);
			out.push(spot);
		}
		return out;
	});

	// Edges already drawn and edges still owed decide the stroke style. A drawn
	// count of the whole edge turns it into a completed (dimmed) stroke; a
	// partially drawn one shows a half-state.
	function edgeClass(e: DrawnEdge): string {
		const u = usage.get(e.index);
		if (!u) return 'edge edge-idle';
		if (u.remaining <= 0) return 'edge edge-done';
		if (u.drawn > 0) return 'edge edge-partial';
		return 'edge edge-idle';
	}

	function pointFor(id: number): { x: number; y: number } {
		return geometry.points.get(id) ?? { x: 0, y: 0 };
	}

	/** Pointer position in client coordinates, from a mouse or touch event. */
	function clientOf(ev: PointerEvent): { x: number; y: number } {
		return { x: ev.clientX, y: ev.clientY };
	}

	function onNodePointerDown(ev: PointerEvent, id: number) {
		if (!draggable) return;
		// Left button / touch / pen only. A right-click must not start a drag.
		if (ev.button !== 0 && ev.pointerType === 'mouse') return;
		dragState = beginDrag(id, pointFor(id), clientOf(ev));
		draggingId = id;
		// Capture so the drag keeps tracking even when the pointer leaves the node.
		(ev.currentTarget as Element).setPointerCapture?.(ev.pointerId);
	}

	function onPointerMove(ev: PointerEvent) {
		if (!dragState) return;
		const next = updateDrag(dragState, clientOf(ev), screenToUserScale(svgEl));
		dragState = next.drag;
		if (next.position) {
			dragged = withPosition(dragged, dragState.id, next.position);
		}
	}

	function onPointerUp(ev: PointerEvent) {
		const finished = dragState;
		dragState = null;
		draggingId = null;
		(ev.currentTarget as Element).releasePointerCapture?.(ev.pointerId);
		// A press that never became a drag is a click: play it as a pick.
		if (finished && isClick(finished)) pick(finished.id);
	}

	/** Double-clicking a node puts it back where the map lays it out. */
	function onNodeDoubleClick(ev: MouseEvent, id: number) {
		if (!draggable) return;
		ev.preventDefault();
		const next = new Map(dragged);
		next.delete(id);
		dragged = next;
	}

	function segmentPath(a: number, b: number): string {
		const p = pointFor(a);
		const q = pointFor(b);
		return `M ${p.x.toFixed(1)} ${p.y.toFixed(1)} L ${q.x.toFixed(1)} ${q.y.toFixed(1)}`;
	}

	function blockedHere(): boolean {
		return !!lastEvent && lastEvent.blocked && lastEvent.move === game.moves;
	}

	function canPick(id: number): boolean {
		if (!interactive) return false;
		if (game.status === 'WON' || game.status === 'LOST') return false;
		return legal.has(id);
	}

	function pick(id: number) {
		if (canPick(id) && onpick) onpick(id);
	}
</script>

<div class="board" data-testid="board" data-bump={bumpKey}>
	<svg
		bind:this={svgEl}
		viewBox={geometry.viewBox}
		role="img"
		aria-label={`Map ${map.name}: ${map.nodeCount} nodes, ${map.edgeCount} edges, ${game.moves} of ${game.moves + game.remaining} traversals drawn`}
		class="board-svg"
		class:dragging={draggingId !== null}
		class:draggable
		onpointermove={onPointerMove}
		onpointerup={onPointerUp}
		onpointercancel={onPointerUp}
	>
		<!-- Edges first, so nodes paint over their endpoints. -->
		<g class="edges">
			{#each geometry.edges as e (e.index)}
				{@const label = edgeLabelPointFor(e)}
				{@const u = usage.get(e.index)}
				<g class={edgeClass(e)} data-edge={e.index}>
					<path d={e.path} class="edge-line" />
					{#if e.oneWay}
						{@const arrow = edgeArrow(e)}
						<!-- Node radius is 26 and the head is 20 long; the default back-off
						     of 62 leaves the tip ~16 units clear of the disc. -->
						<!-- The arrowhead is the only visible cue that an edge can be drawn
						     one way only, so it must be on the board, not just in a tooltip. -->
						<path
							class="edge-arrow"
							d="M -26 -17 L 20 0 L -26 17 Z"
							transform="translate({arrow.x.toFixed(1)} {arrow.y.toFixed(1)}) rotate({arrow.angle.toFixed(1)})"
						/>
					{/if}
					{#if e.count > 1}
						<text x={label.x} y={label.y} class="edge-count" text-anchor="middle" dominant-baseline="central"
							>{u ? u.remaining : e.count}</text
						>
					{/if}
					{#if e.oneWay}
						<title>
							one-way edge: draw {e.from} → {e.to} ({e.count} {e.count === 1 ? 'time' : 'times'})
						</title>
					{/if}
				</g>
			{/each}
		</g>

		<!-- The trail actually drawn, thicker and on top of the idle edges. -->
		<g class="trail">
			{#each drawnSegments as seg, i (i)}
				<path d={segmentPath(seg.a, seg.b)} class="trail-line" />
			{/each}
		</g>

		<!-- The solver's proposal, dashed and numbered, so it reads as an ordered
		     plan rather than just "these edges are left". -->
		{#if solutionSegments.length > 0}
			<g class="solution">
				{#each solutionSegments as seg, i (i)}
					{@const mid = solutionBadges[i] ?? pointFor(seg.a)}
					<path d={segmentPath(seg.a, seg.b)} class="solution-line" />
					<circle cx={mid.x} cy={mid.y} r="30" class="solution-step" />
					<text x={mid.x} y={mid.y} class="solution-num" text-anchor="middle" dominant-baseline="central"
						>{i + 1}</text
					>
				{/each}
			</g>
		{/if}

		<g class="nodes">
			{#each geometry.nodes as n (n.id)}
				{@const p = pointFor(n.id)}
				{@const isHere = here === n.id}
				{@const pickable = canPick(n.id)}
				{#if interactive}
					<g
						class="node {isHere ? 'node-here' : ''} {pickable ? 'node-legal' : ''} {startNode === n.id ? 'node-start' : ''} {draggingId === n.id ? 'node-dragging' : ''} {blockedHere() && isHere ? 'node-bump' : ''}"
						transform="translate({p.x} {p.y})"
						role="button"
						tabindex={pickable ? 0 : -1}
						aria-label={nodeLabel(n)}
						aria-disabled={!pickable}
						data-node={n.id}
						onpointerdown={(ev) => onNodePointerDown(ev, n.id)}
						ondblclick={(ev) => onNodeDoubleClick(ev, n.id)}
						onkeydown={(ev) => {
							if (ev.key === 'Enter' || ev.key === ' ') {
								ev.preventDefault();
								pick(n.id);
							}
						}}
					>
						<circle r="26" class="node-disc" />
						{#if startNode === n.id}<circle r="40" class="start-ring" aria-hidden="true" />{/if}
						<text class="node-id" text-anchor="middle" dominant-baseline="central">{n.id}</text>
					</g>
				{:else}
					<g
						class="node {isHere ? 'node-here' : ''} {startNode === n.id ? 'node-start' : ''} {draggingId === n.id ? 'node-dragging' : ''}"
						transform="translate({p.x} {p.y})"
						aria-hidden="true"
						data-node={n.id}
						onpointerdown={(ev) => onNodePointerDown(ev, n.id)}
						ondblclick={(ev) => onNodeDoubleClick(ev, n.id)}
					>
						<circle r="26" class="node-disc" />
						{#if startNode === n.id}<circle r="40" class="start-ring" aria-hidden="true" />{/if}
						<text class="node-id" text-anchor="middle" dominant-baseline="central">{n.id}</text>
					</g>
				{/if}
			{/each}
		</g>
	</svg>
</div>

<style>
	.board {
		width: 100%;
		background: radial-gradient(circle at 30% 20%, #ffffff, #eef0ff 70%);
		border: 1px solid #c7cdf5;
		border-radius: 0.75rem;
		overflow: hidden;
	}
	.board-svg {
		display: block;
		width: 100%;
		height: auto;
		max-height: 70vh;
		/* Nodes are draggable, so the browser must not claim the gesture for
		   panning. Pointer events on the nodes stop propagation where needed. */
		touch-action: none;
	}
	/* While a node is under the pointer, no text should be selected by accident. */
	.board-svg.dragging {
		user-select: none;
		cursor: grabbing;
	}

	.edge-line {
		fill: none;
		stroke: #8a93c9;
		stroke-width: 7;
		stroke-linecap: round;
	}
	.edge-partial .edge-line {
		stroke: #6d5ce0;
	}
	.edge-done .edge-line {
		stroke: #c3c8e8;
		stroke-dasharray: 2 10;
	}
	.edge-count {
		font-size: 34px;
		font-weight: 700;
		fill: #4b3fb0;
		paint-order: stroke;
		stroke: #ffffff;
		stroke-width: 8;
		pointer-events: none;
	}

	/* One-way edges: a solid arrowhead in the direction travel is allowed. It
	   follows the edge's own colour so a spent edge dims with its line. */
	.edge-arrow {
		fill: #4b3fb0;
		stroke: #ffffff;
		stroke-width: 4;
		stroke-linejoin: round;
		pointer-events: none;
	}
	.edge-partial .edge-arrow {
		fill: #6d5ce0;
	}
	.edge-done .edge-arrow {
		fill: #c3c8e8;
	}

	.trail-line {
		fill: none;
		stroke: #16a34a;
		stroke-width: 11;
		stroke-linecap: round;
	}

	.solution-line {
		fill: none;
		stroke: #f59e0b;
		stroke-width: 9;
		stroke-dasharray: 22 16;
		stroke-linecap: round;
		opacity: 0.9;
	}
	.solution-step {
		fill: #b45309;
		stroke: #ffffff;
		stroke-width: 4;
	}
	.solution-num {
		font-size: 30px;
		font-weight: 700;
		fill: #ffffff;
		pointer-events: none;
	}

	.node {
		cursor: default;
	}
	/* A draggable node advertises itself with a grab cursor, and switches to
	   grabbing while it moves. Legal nodes keep the pointer cursor, which is the
	   stronger signal: clicking them plays the game. */
	.board-svg.draggable .node {
		cursor: grab;
	}
	.node-legal {
		cursor: pointer;
	}
	.node-dragging {
		cursor: grabbing;
	}
	/* Lift the dragged node above its neighbours so it is never hidden. */
	.node-dragging .node-disc {
		stroke-width: 9;
		filter: drop-shadow(0 4px 8px rgba(43, 31, 122, 0.35));
	}
	.node-disc {
		fill: #ffffff;
		stroke: #4b3fb0;
		stroke-width: 6;
	}
	.node-legal .node-disc {
		fill: #f3f0ff;
		stroke: #6d5ce0;
		stroke-width: 8;
	}
	.node-here .node-disc {
		fill: #16a34a;
		stroke: #065f2e;
	}
	.node-id {
		font-size: 30px;
		font-weight: 700;
		fill: #2b1f7a;
		pointer-events: none;
	}
	.node-here .node-id {
		fill: #ffffff;
	}
	/* A dashed ring marks where the trail began, so the start is not lost once
	   the pen has moved on. */
	.start-ring {
		fill: none;
		stroke: #6d5ce0;
		stroke-width: 4;
		stroke-dasharray: 8 7;
	}
	/* Keyboard focus shows as a dashed ring on the disc. The default outline is
	   suppressed because SVG groups draw a square box around the node, which
	   reads as a selection artefact rather than a focus cue. */
	.node:focus {
		outline: none;
	}
	.node:focus-visible .node-disc {
		outline: none;
		stroke-dasharray: 6 4;
	}

	.node-bump {
		animation: bump 220ms ease-out;
	}
	@keyframes bump {
		0% {
			transform: translate(var(--x, 0), var(--y, 0));
		}
		50% {
			opacity: 0.45;
		}
		100% {
			opacity: 1;
		}
	}
</style>
