<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContextClient } from '@urql/svelte';
	import { describeError, lossReasonText, unwrap, type DescribedError } from '$lib/errors';
	import AiPromptCard from '$lib/components/AiPromptCard.svelte';
	import Board from '$lib/components/Board.svelte';
	import ConnectionBadge from '$lib/components/ConnectionBadge.svelte';
	import Controls from '$lib/components/Controls.svelte';
	import ResultModal from '$lib/components/ResultModal.svelte';
	import ShortcutsDialog from '$lib/components/ShortcutsDialog.svelte';
	import Stats from '$lib/components/Stats.svelte';
	import StatusLine from '$lib/components/StatusLine.svelte';
	import { BULK_MOVE_MUTATION, MAPS_QUERY, MOVE_MUTATION, RESET_MUTATION, SOLVE_MUTATION, START_MUTATION, UNDO_MUTATION } from '$lib/queries';
	import { nextMap } from '$lib/maps';
	import { startSession } from '$lib/start';
	import { buildSessionPrompt } from '$lib/prompt';
	import { isTypingTarget, keyToAction } from '$lib/shortcuts';
	import { createSessionStore } from '$lib/stores/session';
	import { createSessionSource } from '$lib/stores/source';
	import { createReplay, prefersReducedMotion, type ReplayFrame } from '$lib/replay';	import { describeEvent } from '$lib/hud';
	import type { GameMap, Session } from '$lib/types';

	const sessionId = $page.params.id ?? '';
	const client = getContextClient();
	const store = createSessionStore(createSessionSource(client, sessionId), sessionId);
	onDestroy(() => store.destroy());

	const origin = typeof window !== 'undefined' ? window.location.origin : '';
	const live = $derived($store.session);
	const connection = $derived($store.connection);

	let actionError = $state<DescribedError | null>(null);
	let dismissedAttempts = $state<number | null>(null);
	let showHelp = $state(false);
	let busy = $state(false);

	// The solution overlay: node ids of a full completion, or null.
	let solution = $state<number[] | null>(null);
	let solutionStart = $state<number | null>(null);

	// Replay a bulkMove tile by tile so the pen visibly travels.
	const replay = createReplay(prefersReducedMotion() ? Date.now() : 0);
	onDestroy(() => replay.destroy());
	let replayFrameState = $state<ReplayFrame | null>(null);

	$effect(() => {
		const s = live;
		if (!s) return;
		replay.sync(s);
		// Ask for a frame now, then keep asking while the animation is running.
		const tick = () => {
			const f: ReplayFrame | null = replay.frame();
			replayFrameState = f;
			if (f?.replaying) requestAnimationFrame(tick);
		};
		tick();
	});
	const frame = $derived(replayFrameState);
	const session = $derived(frame?.session ?? null);

	let initialised = false;
	$effect(() => {
		const s = live;
		if (!s || initialised) return;
		initialised = true;
		if (s.state.status === 'WON' || s.state.status === 'LOST') dismissedAttempts = s.attempts;
	});

	const status = $derived(session?.state.status ?? 'READY');
	const terminal = $derived(status === 'WON' || status === 'LOST');
	const liveTerminal = $derived(live?.state.status === 'WON' || live?.state.status === 'LOST');
	const showModal = $derived(terminal && !!session && !frame?.replaying && dismissedAttempts !== session.attempts);
	const lastMove = $derived(describeEvent(session?.lastEvent ?? null));

	const resultText = $derived(
		!session || !terminal
			? ''
			: status === 'WON'
				? `Solved in ${session.state.moves} moves.`
				: `Stuck with ${session.state.remaining} traversals left. ${lossReasonText(session.state.lossReason)}`
	);

	let maps = $state<GameMap[] | null>(null);
	let nextBusy = $state(false);
	const next = $derived(maps && session ? nextMap(maps, session.mapId) : null);
	$effect(() => {
		if (!showModal || maps) return;
		maps = [];
		unwrap<{ maps: GameMap[] }>(client.query(MAPS_QUERY, {}))
			.then((d) => (maps = d.maps))
			.catch(() => undefined);
	});

	async function playNext() {
		if (!next || !session) return;
		nextBusy = true;
		try {
			await goto(`/play/${await startSession(client, next.id, session.displayName)}`);
		} catch (e) {
			actionError = describeError(e);
			dismissedAttempts = session.attempts;
		} finally {
			nextBusy = false;
		}
	}

	async function act(fn: () => Promise<Session | void>) {
		if (busy) return;
		busy = true;
		actionError = null;
		try {
			const s = await fn();
			if (s) store.apply(s);
		} catch (e) {
			actionError = describeError(e);
		} finally {
			busy = false;
		}
	}

	/** Picking a node starts the trail when it is unstarted, else draws the edge to it. */
	function pick(node: number) {
		if (!live || liveTerminal) return;
		if (live.state.pos === null) {
			return act(
				async () =>
					(await unwrap<{ start: Session }>(client.mutation(START_MUTATION, { sessionID: sessionId, node }))).start
			);
		}
		return act(
			async () => (await unwrap<{ move: Session }>(client.mutation(MOVE_MUTATION, { sessionID: sessionId, node }))).move
		);
	}

	function undo() {
		if (!live) return;
		return act(
			async () => (await unwrap<{ undo: Session }>(client.mutation(UNDO_MUTATION, { sessionID: sessionId }))).undo
		);
	}

	const reset = () =>
		act(async () => {
			const s = (await unwrap<{ reset: Session }>(client.mutation(RESET_MUTATION, { sessionID: sessionId }))).reset;
			dismissedAttempts = s.attempts;
			solution = null;
			solutionStart = null;
			return s;
		});

	/** Asks the solver for a completion and shows it as an overlay. */
	async function solve() {
		if (!live) return;
		busy = true;
		actionError = null;
		try {
			const s = (await unwrap<{ solve: Session }>(client.mutation(SOLVE_MUTATION, { sessionID: sessionId }))).solve;
			if (s.lastPlan?.found) {
				solutionStart = s.lastPlan.start;
				solution = [s.lastPlan.start, ...s.lastPlan.steps];
			}
			store.apply(s);
		} catch (e) {
			actionError = describeError(e);
		} finally {
			busy = false;
		}
	}

	/** Plays the shown solution from the current state through bulkMove. */
	async function playSolution() {
		if (!live || !solution) return;
		const plan = live.lastPlan;
		if (!plan?.found) return;
		const nodes = live.state.pos === null ? plan.steps : plan.steps;
		await act(async () => {
			if (live.state.pos === null) {
				store.apply(
					(await unwrap<{ start: Session }>(client.mutation(START_MUTATION, { sessionID: sessionId, node: plan.start })))
						.start
				);
			}
			return (await unwrap<{ bulkMove: Session }>(
				client.mutation(BULK_MOVE_MUTATION, { sessionID: sessionId, nodes, stopOnBlocked: true })
			)).bulkMove;
		});
		solution = null;
	}

	function onKeyDown(e: KeyboardEvent) {
		if (showModal || showHelp || e.defaultPrevented || isTypingTarget(e.target)) return;
		const action = keyToAction(e);
		if (!action) return;
		e.preventDefault();
		if (action.type === 'undo') undo();
		else if (action.type === 'reset') reset();
		else if (action.type === 'solve') solve();
		else if (action.type === 'help') showHelp = true;
		else if (action.type === 'dismiss') showHelp = false;
	}
</script>

<svelte:head><title>{pageTitle(session?.map.name)}</title></svelte:head>
<svelte:window onkeydown={onKeyDown} />

<div class="mx-auto max-w-7xl px-4 sm:px-6 py-6">
	{#if $store.loading}
		<p role="status" class="muted">Loading game…</p>
	{:else if !session}
		<div role="alert" class="error">
			<strong>{$store.error?.title ?? 'Game unavailable'}.</strong>
			{$store.error?.message}
			<a href="/" class="ml-2 underline">Back to the maps</a>
		</div>
	{:else}
		<div class="head">
			<div>
				<h1 title={session.map.description}>{session.map.name}</h1>
				<p class="sub">
					{session.displayName || 'Anonymous'} ·
					<a class="underline" href="/watch/{session.id}">spectator link</a>
				</p>
			</div>
			<ConnectionBadge state={connection} />
		</div>

		{#if actionError}
			<p role="alert" class="error" data-testid="action-error">
				<strong>{actionError.title}</strong>{#if actionError.code}<span class="code">[{actionError.code}]</span>{/if}
				{#if actionError.message}— {actionError.message}{/if}
			</p>
		{/if}

		<div class="layout">
			<section aria-label="Board">
				<Board
					map={session.map}
					game={session.state}
					lastEvent={session.lastEvent}
					bumpKey={session.seq}
					onpick={liveTerminal ? undefined : pick}
					showSolution={solution}
					draggable
				/>
				<p class="last-move" data-testid="last-move">{lastMove}</p>
				<p class="sr-only" role="status" aria-live="polite">{lastMove}</p>
			</section>

			<section class="side" aria-label="Game">
				<StatusLine {session} />
				<p class="sr-only" role="status" aria-live="polite" data-testid="result-live">{resultText}</p>
				<Stats {session} />
				<div class="panel">
					<h2>Controls</h2>
					<Controls
						onundo={undo}
						onreset={reset}
						onsolve={solve}
						disabled={busy}
						solveDisabled={busy || liveTerminal}
					/>
					<p class="hint">Click a node to place the pen, then click neighbours to draw edges.</p>
				</div>

				{#if solution}
					<div class="panel solution-panel" data-testid="solution-panel">
						<h2>Solver</h2>
						<p class="hint">
							{#if session.state.pos === null}
								Start at node <strong>{solutionStart}</strong>, then draw {solution.length - 1} edges. The dashed
								overlay is the whole trail.
							{:else}
								{solution.length - 1} edges remain on this completion. The dashed overlay is the rest of the trail.
							{/if}
						</p>
						<button type="button" class="btn" onclick={playSolution} disabled={busy} data-testid="play-solution">
							Draw it for me
						</button>
					</div>
				{/if}

				<div class="panel">
					<h2>Help</h2>
					<ul class="help">
						<li>Click a node to start; then click a neighbour to draw the edge between them.</li>
						<li>An edge may be drawn only as many times as it says. Spent edges are grey.</li>
						<li>A one-way edge can be drawn in one direction only.</li>
						<li>Get stuck with edges left and the game ends: undo, or reset.</li>
						<li><kbd>Z</kbd> undo · <kbd>R</kbd> reset · <kbd>S</kbd> solve · <kbd>?</kbd> shortcuts</li>
					</ul>
					<a href="/learn" class="more">All rules →</a>
				</div>

				<AiPromptCard
					prompt={buildSessionPrompt(origin, session)}
					blurb="Copy this prompt into an AI chat and it will try to draw the whole figure in this session."
				/>
			</section>
		</div>

		{#if showModal}
			<ResultModal
				{status}
				reason={session.state.lossReason}
				moves={session.state.moves}
				par={session.map.parMoves}
				best={session.bestMoves}
				nextName={next?.name ?? null}
				{nextBusy}
				onreset={reset}
				onnext={playNext}
				onclose={() => (dismissedAttempts = session.attempts)}
			/>
		{/if}
		{#if showHelp}
			<ShortcutsDialog open={showHelp} onclose={() => (showHelp = false)} />
		{/if}
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
		margin-top: 0.5rem;
	}
	.code {
		margin-left: 0.3rem;
		font-family: ui-monospace, monospace;
		font-size: 0.75rem;
	}
	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}
	h1 {
		margin: 0;
		font-size: 1.5rem;
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
		margin-top: 1rem;
		grid-template-columns: minmax(0, 1fr);
	}
	@media (min-width: 1024px) {
		.layout {
			grid-template-columns: minmax(0, 1fr) 340px;
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
	.hint {
		margin: 0.6rem 0 0;
		font-size: 0.78rem;
		line-height: 1.45;
		color: #4b5563;
	}
	.last-move {
		min-height: 1.25rem;
		margin: 0.5rem 0 0;
		font-size: 0.78rem;
		color: #4b5563;
	}
	.help {
		margin: 0;
		padding-left: 1.1rem;
		list-style: disc;
		font-size: 0.78rem;
		line-height: 1.5;
		color: #374151;
	}
	.help li {
		margin-bottom: 0.25rem;
	}
	.more {
		display: inline-block;
		margin-top: 0.5rem;
		font-size: 0.78rem;
		color: #4b3fb0;
		text-decoration: underline;
	}
	.btn {
		border: 1px solid #a9b1e8;
		background: #f3f0ff;
		color: #2b1f7a;
		border-radius: 0.55rem;
		padding: 0.5rem 0.9rem;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
	}
	.btn:hover:not(:disabled) {
		background: #e7e3ff;
	}
	.btn:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}
	.solution-panel {
		border-color: #fcd34d;
		background: #fffbeb;
	}
	kbd {
		border: 1px solid #d3d8f0;
		border-radius: 0.3rem;
		padding: 0 0.25rem;
		background: #f8f9ff;
		font-size: 0.7rem;
	}
</style>
