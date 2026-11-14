<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onMount } from 'svelte';
	import { getContextClient } from '@urql/svelte';
	import { unwrap } from '$lib/errors';
	import MiniMap from '$lib/components/MiniMap.svelte';
	import { MAPS_QUERY } from '$lib/queries';
	import { difficultyLabel, sortMaps } from '$lib/maps';
	import type { GameMap } from '$lib/types';

	const client = getContextClient();
	let maps = $state<GameMap[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(async () => {
		try {
			const data = await unwrap<{ maps: GameMap[] }>(client.query(MAPS_QUERY, {}));
			maps = sortMaps(data.maps);
		} catch (e) {
			error = (e as { message?: string })?.message ?? 'Could not load the maps.';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head><title>{pageTitle('Maps')}</title></svelte:head>

<div class="mx-auto max-w-6xl px-4 sm:px-6 py-8">
	<h1>Maps</h1>
	<p class="lede">Every figure, with its size and the parity verdict. “Complete trail” is the number of edges a solution must draw.</p>

	{#if error}<p class="error" role="alert">{error}</p>{/if}
	{#if loading}
		<p class="muted" role="status">Loading…</p>
	{:else}
		<div class="grid">
			{#each maps as map (map.id)}
				<article class="card" data-testid="map-row">
					<MiniMap {map} maxWidth={144} title={map.name} />
					<div class="body">
						<h2><a href="/play/new?map={map.id}">▶ {map.name}</a></h2>
						<p class="desc">{map.description}</p>
						<table>
							<tbody>
								<tr><th>Difficulty</th><td>{difficultyLabel(map.difficulty)}</td></tr>
								<tr><th>Nodes</th><td>{map.nodeCount}</td></tr>
								<tr><th>Edges</th><td>{map.edgeCount}</td></tr>
								<tr><th>Traversals</th><td>{map.stepCount}</td></tr>
								<tr><th>Trail length</th><td data-testid="par">{map.parMoves ?? 'none'}</td></tr>
								<tr><th>Verdict</th><td class="verdict {map.analysis.verdict.toLowerCase()}">{map.analysis.verdict}</td></tr>
							</tbody>
						</table>
						<p class="msg">{map.analysis.message}</p>
					</div>
				</article>
			{/each}
		</div>
	{/if}
</div>

<style>
	h1 {
		margin: 0 0 0.4rem;
		font-size: 1.75rem;
		color: #2b1f7a;
	}
	.lede {
		margin: 0 0 1.5rem;
		max-width: 44rem;
		font-size: 0.9rem;
		color: #374151;
	}
	.grid {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
	}
	.card {
		display: flex;
		gap: 0.85rem;
		border: 1px solid #d3d8f0;
		border-radius: 0.9rem;
		background: #ffffff;
		padding: 0.75rem;
	}
	/* A fixed preview column keeps every card's text aligned and stops the
	   square SVG from squeezing the text into a sliver. */
	.card > :global(.mini) {
		width: 9rem;
		flex: 0 0 9rem;
	}
	.body {
		min-width: 0;
		flex: 1 1 auto;
	}
	h2 {
		margin: 0 0 0.25rem;
		font-size: 1rem;
	}
	h2 a {
		color: #2b1f7a;
		text-decoration: none;
	}
	h2 a:hover {
		text-decoration: underline;
	}
	.desc {
		margin: 0 0 0.5rem;
		font-size: 0.75rem;
		line-height: 1.45;
		color: #4b5563;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.72rem;
	}
	th {
		text-align: left;
		font-weight: 500;
		color: #6b7280;
		padding: 0.1rem 0;
		width: 45%;
		white-space: nowrap;
	}
	td {
		color: #1f2937;
		padding: 0.1rem 0;
	}
	.verdict {
		font-weight: 600;
	}
	.verdict.impossible {
		color: #b45309;
	}
	.verdict.circuit,
	.verdict.trail {
		color: #15803d;
	}
	.verdict.directed {
		color: #4b3fb0;
	}
	.msg {
		margin: 0.45rem 0 0;
		font-size: 0.7rem;
		color: #6b7280;
	}
	.muted {
		color: #6b7280;
		font-size: 0.9rem;
	}
	.error {
		border: 1px solid #fca5a5;
		background: #fef2f2;
		color: #991b1b;
		border-radius: 0.6rem;
		padding: 0.6rem 0.9rem;
		font-size: 0.85rem;
	}
</style>
