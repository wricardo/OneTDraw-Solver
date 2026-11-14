<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getContextClient } from '@urql/svelte';
	import { unwrap } from '$lib/errors';
	import AiPromptCard from '$lib/components/AiPromptCard.svelte';
	import MiniMap from '$lib/components/MiniMap.svelte';
	import { MAPS_QUERY } from '$lib/queries';
	import { difficultyLabel, groupByDifficulty } from '$lib/maps';
	import { startSession } from '$lib/start';
	import { buildOpenPrompt } from '$lib/prompt';
	import type { GameMap } from '$lib/types';

	const client = getContextClient();
	const origin = typeof window !== 'undefined' ? window.location.origin : '';

	let maps = $state<GameMap[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let starting = $state<string | null>(null);
	let name = $state('');

	onMount(async () => {
		try {
			const data = await unwrap<{ maps: GameMap[] }>(client.query(MAPS_QUERY, {}));
			maps = data.maps;
		} catch (e) {
			error = (e as { message?: string })?.message ?? 'Could not load the maps.';
		} finally {
			loading = false;
		}
	});

	const groups = $derived(groupByDifficulty(maps));

	async function play(mapID: string) {
		starting = mapID;
		try {
			const id = await startSession(client, mapID, name);
			await goto(`/play/${id}`);
		} catch (e) {
			error = (e as { message?: string })?.message ?? 'Could not start the game.';
			starting = null;
		}
	}
</script>

<svelte:head><title>{pageTitle()}</title></svelte:head>

<div class="mx-auto max-w-6xl px-4 sm:px-6 py-8">
	<section class="hero">
		<h1>One T Draw</h1>
		<p class="lede">
			A figure made of lines and dots. Draw <strong>every line exactly once</strong>, without lifting
			your pen and without retracing a spent line. Some lines are numbered: those you must draw that
			many times. One continuous stroke, a few seconds' thought, and the figure is done — or you get
			stuck and start again.
		</p>
		<div class="name-row">
			<label for="name">Name for the scoreboard</label>
			<input id="name" bind:value={name} placeholder="Anonymous" maxlength="40" />
		</div>
	</section>

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	{#if loading}
		<p class="muted" role="status">Loading maps…</p>
	{:else}
		{#each groups as group (group.difficulty)}
			<section class="group" aria-label={difficultyLabel(group.difficulty)}>
				<h2>{difficultyLabel(group.difficulty)}</h2>
				<div class="grid">
					{#each group.maps as map (map.id)}
						<article class="card" data-testid="map-card">
							<MiniMap {map} maxWidth={220} title={map.name} />
							<h3>{map.name}</h3>
							<p class="desc">{map.description}</p>
							<p class="meta">
								{map.nodeCount} nodes · {map.edgeCount} edges · {map.stepCount} traversals
							</p>
							<p class="par {map.parMoves === null ? 'none' : ''}">
								{#if map.parMoves === null}
									No single trail exists
								{:else}
									Complete trail: {map.parMoves} moves
								{/if}
							</p>
							<button
								type="button"
								class="play"
								onclick={() => play(map.id)}
								disabled={starting !== null}
								data-testid="play-{map.id}"
							>
								{starting === map.id ? 'Starting…' : 'Play'}
							</button>
						</article>
					{/each}
				</div>
			</section>
		{/each}

		<section class="ai">
			<AiPromptCard
				prompt={buildOpenPrompt(origin)}
				blurb="Copy this into an AI assistant that can make web requests, and it will play on its own."
			/>
		</section>
	{/if}
</div>

<style>
	.hero {
		margin-bottom: 2rem;
	}
	h1 {
		margin: 0 0 0.5rem;
		font-size: 2.25rem;
		font-weight: 700;
		color: #2b1f7a;
	}
	.lede {
		max-width: 46rem;
		font-size: 1rem;
		line-height: 1.6;
		color: #374151;
	}
	.name-row {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin-top: 1rem;
		font-size: 0.85rem;
		color: #4b5563;
	}
	input {
		border: 1px solid #a9b1e8;
		border-radius: 0.5rem;
		padding: 0.4rem 0.6rem;
		font-size: 0.9rem;
		color: #1f2937;
		background: #ffffff;
	}
	.group {
		margin-bottom: 2rem;
	}
	h2 {
		margin: 0 0 0.75rem;
		font-size: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: #4b3fb0;
	}
	.grid {
		display: grid;
		gap: 1rem;
		grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
	}
	.card {
		border: 1px solid #d3d8f0;
		border-radius: 0.9rem;
		background: #ffffff;
		padding: 0.75rem;
		display: flex;
		flex-direction: column;
	}
	.card h3 {
		margin: 0.6rem 0 0.3rem;
		font-size: 1rem;
		color: #2b1f7a;
	}
	.desc {
		margin: 0 0 0.5rem;
		font-size: 0.78rem;
		line-height: 1.45;
		color: #4b5563;
		flex: 1;
	}
	.meta {
		margin: 0 0 0.25rem;
		font-size: 0.7rem;
		color: #6b7280;
	}
	.par {
		margin: 0 0 0.6rem;
		font-size: 0.72rem;
		font-weight: 600;
		color: #15803d;
	}
	.par.none {
		color: #b45309;
	}
	.play {
		border: 1px solid #3a3090;
		background: #4b3fb0;
		color: #ffffff;
		border-radius: 0.55rem;
		padding: 0.5rem 0.9rem;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
	}
	.play:hover:not(:disabled) {
		background: #3a3090;
	}
	.play:disabled {
		opacity: 0.6;
		cursor: progress;
	}
	.ai {
		max-width: 34rem;
		margin-top: 1rem;
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
