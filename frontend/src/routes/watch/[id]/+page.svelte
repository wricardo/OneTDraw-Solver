<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { getContextClient } from '@urql/svelte';
	import Board from '$lib/components/Board.svelte';
	import ConnectionBadge from '$lib/components/ConnectionBadge.svelte';
	import Stats from '$lib/components/Stats.svelte';
	import StatusLine from '$lib/components/StatusLine.svelte';
	import { describeEvent } from '$lib/hud';
	import { createSessionStore } from '$lib/stores/session';
	import { createSessionSource } from '$lib/stores/source';

	const sessionId = $page.params.id ?? '';
	const client = getContextClient();
	const store = createSessionStore(createSessionSource(client, sessionId), sessionId);
	onDestroy(() => store.destroy());

	const session = $derived($store.session);
	const connection = $derived($store.connection);
	const lastMove = $derived(describeEvent(session?.lastEvent ?? null));
</script>

<svelte:head><title>{pageTitle(session ? `Watching ${session.map.name}` : 'Watch')}</title></svelte:head>

<div class="mx-auto max-w-5xl px-4 sm:px-6 py-6">
	{#if $store.loading}
		<p role="status" class="muted">Loading game…</p>
	{:else if !session}
		<div role="alert" class="error">
			<strong>{$store.error?.title ?? 'Game unavailable'}.</strong>
			{$store.error?.message}
			<a href="/sessions" class="ml-2 underline">All sessions</a>
		</div>
	{:else}
		<div class="head">
			<div>
				<h1>Watching {session.map.name}</h1>
				<p class="sub">
					{session.displayName || 'Anonymous'} · <a class="underline" href="/play/{session.id}">play this game yourself</a>
				</p>
			</div>
			<ConnectionBadge state={connection} />
		</div>

		<div class="layout">
			<section aria-label="Board">
				<Board map={session.map} game={session.state} lastEvent={session.lastEvent} bumpKey={session.seq} interactive={false} draggable />
				<p class="last-move" data-testid="last-move">{lastMove}</p>
				<p class="sr-only" role="status" aria-live="polite">{lastMove}</p>
			</section>
			<section class="side" aria-label="Game">
				<StatusLine {session} />
				<Stats {session} />
				<div class="panel">
					<h2>Trail</h2>
					<p class="trail" data-testid="trail">{session.state.path.length ? session.state.path.join(' → ') : 'Not started yet.'}</p>
				</div>
			</section>
		</div>
	{/if}
</div>

<style>
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
	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		margin-bottom: 1rem;
	}
	h1 {
		margin: 0;
		font-size: 1.35rem;
		color: #2b1f7a;
	}
	.sub {
		margin: 0.15rem 0 0;
		font-size: 0.8rem;
		color: #4b5563;
	}
	.layout {
		display: grid;
		gap: 1.5rem;
		grid-template-columns: minmax(0, 1fr);
	}
	@media (min-width: 1024px) {
		.layout {
			grid-template-columns: minmax(0, 1fr) 320px;
		}
	}
	.side {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.panel {
		border: 1px solid #c7cdf5;
		border-radius: 0.75rem;
		background: #ffffff;
		padding: 0.75rem 1rem;
	}
	.panel h2 {
		margin: 0 0 0.4rem;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: #4b3fb0;
	}
	.trail {
		margin: 0;
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		line-height: 1.5;
		color: #374151;
		word-break: break-word;
	}
	.last-move {
		min-height: 1.25rem;
		margin: 0.5rem 0 0;
		font-size: 0.78rem;
		color: #4b5563;
	}
</style>
