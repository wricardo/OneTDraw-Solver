<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onMount } from 'svelte';
	import { getContextClient } from '@urql/svelte';
	import { describeError, unwrap } from '$lib/errors';
	import Board from '$lib/components/Board.svelte';
	import MiniMap from '$lib/components/MiniMap.svelte';
	import { MAPS_QUERY } from '$lib/queries';
	import { sortMaps } from '$lib/maps';
	import { startSession } from '$lib/start';
	import { goto } from '$app/navigation';
	import {
		BULK_MOVE_MUTATION,
		CREATE_SESSION_MUTATION,
		SOLVE_MUTATION,
		START_MUTATION
	} from '$lib/queries';
	import { emptyState } from '$lib/preview';
	import type { GameMap, Session } from '$lib/types';

	// The solver page is the "brute force" surface: pick a map, run the solver,
	// and watch it draw the whole figure. It drives the same session API a human
	// uses, so what it shows is a real game being played.

	const client = getContextClient();

	let maps = $state<GameMap[]>([]);
	let selected = $state<string>('');
	let session = $state<Session | null>(null);
	let error = $state<string | null>(null);
	let running = $state(false);
	let loadingMaps = $state(true);
	let elapsedMs = $state<number | null>(null);

	onMount(async () => {
		try {
			const data = await unwrap<{ maps: GameMap[] }>(client.query(MAPS_QUERY, {}));
			maps = sortMaps(data.maps);
			selected = maps.find((m) => m.parMoves !== null)?.id ?? maps[0]?.id ?? '';
		} catch (e) {
			error = describeError(e).message;
		} finally {
			loadingMaps = false;
		}
	});

	const chosen = $derived(maps.find((m) => m.id === selected) ?? null);
	const solvable = $derived(!!chosen && chosen.parMoves !== null);

	/** Creates a session, asks it to solve, then draws the plan. */
	async function run() {
		if (!chosen || !solvable) return;
		running = true;
		error = null;
		elapsedMs = null;
		session = null;
		const started = performance.now();
		try {
			const created = (
				await unwrap<{ createSession: Session }>(
					client.mutation(CREATE_SESSION_MUTATION, { mapID: chosen.id, displayName: 'solver' })
				)
			).createSession;
			session = created;

			const solved = (await unwrap<{ solve: Session }>(client.mutation(SOLVE_MUTATION, { sessionID: created.id })))
				.solve;
			const plan = solved.lastPlan;
			if (!plan?.found) {
				error = 'The solver could not find a completion for this map.';
				return;
			}

			// Show the plan before drawing it so the overlay is visible even if the
			// bulkMove finishes instantly.
			session = { ...solved };
			const afterStart = (
				await unwrap<{ start: Session }>(client.mutation(START_MUTATION, { sessionID: created.id, node: plan.start }))
			).start;
			session = afterStart;
			const done = (
				await unwrap<{ bulkMove: Session }>(
					client.mutation(BULK_MOVE_MUTATION, {
						sessionID: created.id,
						nodes: plan.steps,
						stopOnBlocked: true
					})
				)
			).bulkMove;
			session = done;
			elapsedMs = Math.round(performance.now() - started);
		} catch (e) {
			error = describeError(e).message;
		} finally {
			running = false;
		}
	}

	async function openAsGame() {
		if (!session) return;
		await goto(`/play/${session.id}`);
	}

	async function playMap() {
		if (!chosen) return;
		await goto(`/play/${await startSession(client, chosen.id, 'solver')}`);
	}
</script>

<svelte:head><title>{pageTitle('Solver')}</title></svelte:head>

<div class="mx-auto max-w-6xl px-4 sm:px-6 py-8">
	<h1>Solver</h1>
	<p class="lede">
		The same engine that judges a game can brute-force one. Pick a map, press solve, and watch it
		draw every edge. It plays through the ordinary API, so the result is a real, replayable session.
	</p>

	{#if error}<p class="error" role="alert">{error}</p>{/if}

	{#if loadingMaps}
		<p class="muted" role="status">Loading maps…</p>
	{:else}
		<div class="picker">
			<label for="map">Map</label>
			<select id="map" bind:value={selected} disabled={running}>
				{#each maps as m (m.id)}
					<option value={m.id}>{m.name}{m.parMoves === null ? ' (unsolvable)' : ` — ${m.stepCount} edges`}</option>
				{/each}
			</select>
			<button type="button" class="btn primary" onclick={run} disabled={running || !solvable} data-testid="run-solver">
				{running ? 'Solving…' : 'Solve'}
			</button>
			{#if !solvable && chosen}
				<span class="note">This map has no single-trail solution, so there is nothing to draw.</span>
			{/if}
		</div>

		{#if chosen}
			<div class="layout">
				<section aria-label="Board">
					{#if session}
						<Board
							map={session.map}
							game={session.state}
							lastEvent={session.lastEvent}
							bumpKey={session.seq}
							showSolution={session.lastPlan ? [session.lastPlan.start, ...session.lastPlan.steps] : null}
							interactive={false}
							draggable
						/>
					{:else}
						<Board map={chosen} game={emptyState(chosen)} interactive={false} draggable />
					{/if}
				</section>
				<section class="side" aria-label="Result">
					<div class="panel">
						<h2>{chosen.name}</h2>
						<MiniMap map={chosen} maxWidth={200} title={chosen.name} />
						<table>
							<tbody>
								<tr><th>Nodes</th><td>{chosen.nodeCount}</td></tr>
								<tr><th>Edges</th><td>{chosen.edgeCount}</td></tr>
								<tr><th>Traversals</th><td>{chosen.stepCount}</td></tr>
								<tr><th>Complete trail</th><td>{chosen.parMoves ?? 'none'}</td></tr>
								<tr><th>Verdict</th><td>{chosen.analysis.verdict}</td></tr>
							</tbody>
						</table>
						<p class="msg">{chosen.analysis.message}</p>
					</div>

					{#if session}
						<div class="panel result" data-testid="solve-result">
							<h2>Result</h2>
							<p class="big {session.state.status.toLowerCase()}">
								{session.state.status === 'WON' ? 'Solved' : session.state.status}
							</p>
							<p class="detail">
								{session.state.moves} edges drawn, {session.state.remaining} left
								{#if elapsedMs !== null}· {elapsedMs} ms end to end{/if}
							</p>
							<p class="trail">{session.state.path.join(' → ')}</p>
							<div class="actions">
								<button type="button" class="btn" onclick={openAsGame} data-testid="open-session">Open as a game</button>
								<button type="button" class="btn" onclick={playMap}>Play it yourself</button>
							</div>
						</div>
					{/if}
				</section>
			</div>
		{/if}
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
		max-width: 46rem;
		font-size: 0.9rem;
		line-height: 1.6;
		color: #374151;
	}
	.picker {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.6rem;
		margin-bottom: 1.25rem;
		font-size: 0.85rem;
		color: #4b5563;
	}
	select {
		border: 1px solid #a9b1e8;
		border-radius: 0.5rem;
		padding: 0.45rem 0.6rem;
		font-size: 0.85rem;
		background: #ffffff;
		color: #1f2937;
		min-width: 16rem;
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
		margin: 0 0 0.5rem;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: #4b3fb0;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.75rem;
		margin-top: 0.5rem;
	}
	th {
		text-align: left;
		font-weight: 500;
		color: #6b7280;
		padding: 0.1rem 0;
		width: 55%;
	}
	td {
		color: #1f2937;
		padding: 0.1rem 0;
	}
	.msg {
		margin: 0.5rem 0 0;
		font-size: 0.72rem;
		color: #6b7280;
	}
	.result {
		border-color: #a7f3c6;
		background: #f6fffa;
	}
	.big {
		margin: 0 0 0.25rem;
		font-size: 1.3rem;
		font-weight: 700;
		color: #15803d;
	}
	.big.lost {
		color: #b91c1c;
	}
	.detail {
		margin: 0;
		font-size: 0.78rem;
		color: #4b5563;
	}
	.trail {
		margin: 0.5rem 0 0.6rem;
		font-family: ui-monospace, monospace;
		font-size: 0.72rem;
		color: #374151;
		word-break: break-word;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
	}
	.btn {
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.55rem;
		padding: 0.45rem 0.85rem;
		font-size: 0.8rem;
		font-weight: 600;
		cursor: pointer;
	}
	.btn:hover:not(:disabled) {
		background: #f3f0ff;
	}
	.btn:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}
	.btn.primary {
		background: #4b3fb0;
		border-color: #3a3090;
		color: #ffffff;
	}
	.btn.primary:hover:not(:disabled) {
		background: #3a3090;
	}
	.note {
		font-size: 0.75rem;
		color: #b45309;
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
