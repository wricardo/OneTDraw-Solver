<script lang="ts">
	import type { GameMap, GameState } from '$lib/types';
	import { boardGeometry, edgeArrow, edgeStates, LAYOUT_SIZE } from '$lib/board';

	let {
		map,
		state = null,
		title = '',
		href = '',
		maxWidth = 260
	}: {
		map: Pick<GameMap, 'id' | 'name' | 'nodes' | 'edges'>;
		/** Optional state, used to grey out edges that are fully drawn. */
		state?: GameState | null;
		title?: string;
		href?: string;
		maxWidth?: number;
	} = $props();

	// Reuse the board's geometry so a preview bows doubled edges and points
	// one-way arrows the same way the full board does; only the labels differ.
	const geometry = $derived(boardGeometry(map as GameMap));

	// Fit the viewBox to the nodes actually drawn. A map whose graph sits in a
	// small part of the layout box (or on a circle, or in one row) would
	// otherwise render as a speck in an empty square.
	//
	// The padding must clear a node's whole footprint (radius plus stroke), not
	// just its centre, or the outermost nodes are clipped at the edge.
	const NODE_HALF = 34 + 14 / 2; // radius + half the stroke width used below
	// MIN_SIDE keeps every preview at a comparable zoom, so a three-node map does
	// not draw its nodes several times larger than a twelve-node one.
	const MIN_SIDE = 760;
	const viewBox = $derived.by(() => {
		let minX = Infinity;
		let minY = Infinity;
		let maxX = -Infinity;
		let maxY = -Infinity;
		for (const p of geometry.nodes) {
			minX = Math.min(minX, p.x);
			minY = Math.min(minY, p.y);
			maxX = Math.max(maxX, p.x);
			maxY = Math.max(maxY, p.y);
		}
		if (!Number.isFinite(minX)) return `0 0 ${LAYOUT_SIZE} ${LAYOUT_SIZE}`;
		const pad = NODE_HALF + 12;
		const w = maxX - minX + pad * 2;
		const h = maxY - minY + pad * 2;
		// Square it up and clamp: squaring keeps a straight row from collapsing
		// to a zero-height viewBox, and the lower clamp stops a tiny graph from
		// being blown up to giant nodes while its neighbour renders small.
		const side = Math.min(Math.max(Math.max(w, h), MIN_SIDE), LAYOUT_SIZE);
		const cx = (minX + maxX) / 2;
		const cy = (minY + maxY) / 2;
		return `${cx - side / 2} ${cy - side / 2} ${side} ${side}`;
	});

	const usage = $derived(state ? edgeStates(map as GameMap, state) : null);
	const label = $derived(title || map.name);
</script>

<svelte:element
	this={href ? 'a' : 'div'}
	{href}
	class="mini"
	aria-label={label}
	data-testid="mini-map"
	style="max-width: {maxWidth}px"
>
	<svg {viewBox} role="img" aria-hidden={href ? 'true' : undefined} preserveAspectRatio="xMidYMid meet">
		{#each geometry.edges as e (e.index)}
			{@const isDone = usage ? (usage.get(e.index)?.remaining ?? 0) <= 0 : false}
			<path
				d={e.path}
				fill="none"
				stroke={isDone ? '#c3c8e8' : '#8a93c9'}
				stroke-width={isDone ? 12 : 16}
				stroke-linecap="round"
			/>
			{#if e.oneWay}
				{@const a = edgeArrow(e, 58)}
				<!-- Node radius here is 34 and the head is 22 long; 58 keeps the tip
				     well clear of the disc at this larger stroke scale. -->
				<path
					d="M -22 -16 L 22 0 L -22 16 Z"
					fill={isDone ? '#c3c8e8' : '#5b4fc4'}
					transform="translate({a.x.toFixed(1)} {a.y.toFixed(1)}) rotate({a.angle.toFixed(1)})"
				/>
			{/if}
		{/each}
		{#each geometry.nodes as n (n.id)}
			<circle cx={n.x} cy={n.y} r="34" fill="#ffffff" stroke="#4b3fb0" stroke-width="14" />
		{/each}
	</svg>
</svelte:element>

<style>
	.mini {
		display: block;
		/* The SVG is square, so the box is too, and it never exceeds the caller's
		   max-width. shrink-0 keeps a flex parent from squeezing the drawing into
		   a sliver and starving the text beside it. */
		width: 100%;
		flex: 0 0 auto;
		aspect-ratio: 1;
		border: 1px solid #d3d8f0;
		border-radius: 0.6rem;
		background: radial-gradient(circle at 30% 20%, #ffffff, #eef0ff 70%);
		overflow: hidden;
	}
	.mini svg {
		display: block;
		width: 100%;
		height: 100%;
	}
</style>
