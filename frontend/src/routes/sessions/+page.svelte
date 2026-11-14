<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onMount } from 'svelte';
	import { getContextClient } from '@urql/svelte';
	import { unwrap } from '$lib/errors';
	import { SESSIONS_QUERY, DELETE_SESSION_MUTATION } from '$lib/queries';
	import { timeAgo } from '$lib/ui';
	import type { SessionSummary } from '$lib/types';

	const client = getContextClient();
	let sessions = $state<SessionSummary[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	async function load() {
		loading = true;
		try {
			const data = await unwrap<{ sessions: SessionSummary[] }>(
				client.query(SESSIONS_QUERY, { sort: 'RECENT', limit: 100 })
			);
			sessions = data.sessions;
		} catch (e) {
			error = (e as { message?: string })?.message ?? 'Could not load the sessions.';
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function remove(id: string) {
		try {
			await unwrap(client.mutation(DELETE_SESSION_MUTATION, { id }));
			sessions = sessions.filter((s) => s.id !== id);
		} catch (e) {
			error = (e as { message?: string })?.message ?? 'Could not delete the game.';
		}
	}

	function statusText(s: SessionSummary): string {
		if (s.state.status === 'WON') return `solved in ${s.state.moves}`;
		if (s.state.status === 'LOST') return 'stuck';
		if (s.state.status === 'READY') return 'not started';
		return `${s.state.moves} drawn`;
	}
</script>

<svelte:head><title>{pageTitle('Sessions')}</title></svelte:head>

<div class="mx-auto max-w-4xl px-4 sm:px-6 py-8">
	<div class="head">
		<h1>Live sessions</h1>
		<button type="button" class="btn" onclick={load} disabled={loading}>Refresh</button>
	</div>
	<p class="lede">Every game in progress or recently played. Open one to watch it, or play alongside.</p>

	{#if error}<p class="error" role="alert">{error}</p>{/if}
	{#if loading}
		<p class="muted" role="status">Loading…</p>
	{:else if sessions.length === 0}
		<p class="muted">No sessions yet. <a href="/" class="underline">Start one</a>.</p>
	{:else}
		<table class="sessions" data-testid="sessions-table">
			<thead>
				<tr><th>Map</th><th>Player</th><th>Status</th><th>Last move</th><th></th></tr>
			</thead>
			<tbody>
				{#each sessions as s (s.id)}
					<tr data-testid="session-row">
						<td><a href="/watch/{s.id}" class="underline">{s.map.name}</a></td>
						<td>{s.displayName || 'Anonymous'}</td>
						<td>
							<span class="status {s.state.status.toLowerCase()}">{statusText(s)}</span>
							{#if s.bestMoves !== null}<span class="best">best {s.bestMoves}</span>{/if}
						</td>
						<td class="when">{timeAgo(s.lastActionAt)}</td>
						<td class="actions">
							<a href="/play/{s.id}" class="btn small">Play</a>
							<button type="button" class="btn small danger" onclick={() => remove(s.id)} aria-label="Delete session">Delete</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<style>
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
	}
	h1 {
		margin: 0;
		font-size: 1.75rem;
		color: #2b1f7a;
	}
	.lede {
		margin: 0.4rem 0 1.5rem;
		font-size: 0.9rem;
		color: #374151;
	}
	.sessions {
		width: 100%;
		border-collapse: collapse;
		background: #ffffff;
		border: 1px solid #d3d8f0;
		border-radius: 0.75rem;
		overflow: hidden;
		font-size: 0.85rem;
	}
	th,
	td {
		text-align: left;
		padding: 0.55rem 0.75rem;
		border-bottom: 1px solid #eef0fa;
	}
	th {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: #6b7280;
		background: #f8f9ff;
	}
	.status {
		font-weight: 600;
		color: #4b5563;
	}
	.status.won {
		color: #15803d;
	}
	.status.lost {
		color: #b91c1c;
	}
	.best {
		margin-left: 0.4rem;
		font-size: 0.72rem;
		color: #6b7280;
	}
	.when {
		color: #6b7280;
		font-size: 0.78rem;
	}
	.actions {
		display: flex;
		gap: 0.4rem;
	}
	.btn {
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.5rem;
		padding: 0.35rem 0.7rem;
		font-size: 0.78rem;
		font-weight: 600;
		cursor: pointer;
		text-decoration: none;
	}
	.btn:hover {
		background: #f3f0ff;
	}
	.btn.small {
		padding: 0.25rem 0.55rem;
		font-size: 0.72rem;
	}
	.btn.danger {
		color: #b91c1c;
		border-color: #fca5a5;
	}
	.btn.danger:hover {
		background: #fef2f2;
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
