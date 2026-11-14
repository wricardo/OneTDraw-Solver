<script lang="ts">
	import { pageTitle } from '$lib/brand';
	import { onMount } from 'svelte';
	import { getContextClient } from '@urql/svelte';
	import { describeError, unwrap } from '$lib/errors';
	import Board from '$lib/components/Board.svelte';
	import { ADMIN_MAP_QUERY, MAP_SUMMARIES_QUERY, UPDATE_MAP_MUTATION, CREATE_MAP_MUTATION, DELETE_MAP_MUTATION, VALIDATE_MAP_MUTATION } from '$lib/queries';
	import { getAdminKey, setAdminKey } from '$lib/admin';
	import { sortMaps } from '$lib/maps';
	import { blankMap, emptyState } from '$lib/preview';
	import type { AdminMap, GameMap, GameState, MapInput, MapValidationResult } from '$lib/types';

	// The editor is a text editor over the map's raw JSON, with the parsed graph
	// rendered beside it and the same validator the CLI uses behind the Validate
	// button. That keeps one definition of "a valid map" instead of a second GUI
	// form that could drift from it.

	const client = getContextClient();

	let summaries = $state<GameMap[]>([]);
	let selected = $state('');
	let draft = $state('');
	let admin = $state<AdminMap | null>(null);
	let validation = $state<MapValidationResult | null>(null);
	let error = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let busy = $state(false);
	let key = $state('');

	onMount(async () => {
		key = getAdminKey();
		try {
			const data = await unwrap<{ maps: GameMap[] }>(client.query(MAP_SUMMARIES_QUERY, {}));
			summaries = sortMaps(data.maps as unknown as GameMap[]);
			selected = summaries[0]?.id ?? '';
			if (selected) await load(selected);
		} catch (e) {
			error = describeError(e).message;
		}
	});

	async function load(id: string) {
		error = null;
		notice = null;
		validation = null;
		try {
			const data = await unwrap<{ adminMap: AdminMap | null }>(client.query(ADMIN_MAP_QUERY, { id }));
			if (!data.adminMap) {
				error = `Map ${id} was not found.`;
				return;
			}
			admin = data.adminMap;
			draft = JSON.stringify(data.adminMap.config, null, 2);
		} catch (e) {
			error = describeError(e).message;
			admin = null;
		}
	}

	function saveKey() {
		setAdminKey(key);
		notice = key ? 'Admin key saved for this tab.' : 'Admin key cleared.';
	}

	/** Parses the draft into a MapInput, reporting a syntax error instead of throwing. */
	function parseDraft(): MapInput | null {
		try {
			const raw = JSON.parse(draft) as {
				id?: string;
				name?: string;
				description?: string;
				difficulty?: string;
				maxMoves?: number;
				max_moves?: number;
				Points?: { Point: number; Level?: number; Hide?: boolean | number }[];
				points?: { point: number; level?: number; hidden?: boolean }[];
				Edges?: { PointA: number; PointB: number; Count?: number; Direction?: { From: number; To: number; Unidirectional: boolean } }[];
				edges?: { pointA: number; pointB: number; count?: number; from?: number; to?: number }[];
			};
			// Accept both the puzzle-file spelling (Points/Edges) and the GraphQL
			// spelling (points/edges), so a map can be pasted either way.
			const points =
				raw.points?.map((p) => ({ point: p.point, level: p.level, hide: p.hidden })) ??
				raw.Points?.map((p) => ({ point: p.Point, level: p.Level, hide: !!p.Hide }));
			const edges =
				raw.edges?.map((e) => ({
					pointA: e.pointA,
					pointB: e.pointB,
					count: e.count,
					direction: e.from !== undefined && e.to !== undefined ? { from: e.from, to: e.to, unidirectional: true } : undefined
				})) ??
				raw.Edges?.map((e) => ({
					pointA: e.PointA,
					pointB: e.PointB,
					count: e.Count,
					direction: e.Direction
						? { from: e.Direction.From, to: e.Direction.To, unidirectional: e.Direction.Unidirectional }
						: undefined
				}));
			return {
				id: raw.id ?? selected,
				name: raw.name ?? '',
				description: raw.description,
				difficulty: raw.difficulty,
				maxMoves: raw.maxMoves ?? raw.max_moves,
				points,
				edges: edges ?? []
			};
		} catch (e) {
			error = `That is not valid JSON: ${(e as Error).message}`;
			return null;
		}
	}

	async function validate() {
		const input = parseDraft();
		if (!input) return;
		error = null;
		notice = null;
		busy = true;
		validation = null;
		try {
			validation = (await unwrap<{ validateMap: MapValidationResult }>(client.mutation(VALIDATE_MAP_MUTATION, { map: input })))
				.validateMap;
		} catch (e) {
			error = describeError(e).message;
		} finally {
			busy = false;
		}
	}

	async function save(asNew: boolean) {
		const input = parseDraft();
		if (!input) return;
		error = null;
		notice = null;
		busy = true;
		try {
			const mutation = asNew ? CREATE_MAP_MUTATION : UPDATE_MAP_MUTATION;
			const field = asNew ? 'createMap' : 'updateMap';
			const data = await unwrap<Record<string, AdminMap>>(client.mutation(mutation, { map: input }));
			admin = data[field];
			notice = `Saved ${admin.name}. Existing sessions keep their own copy of the map.`;
			const list = await unwrap<{ maps: GameMap[] }>(client.query(MAP_SUMMARIES_QUERY, {}));
			summaries = sortMaps(list.maps as unknown as GameMap[]);
			selected = admin.id;
		} catch (e) {
			error = describeError(e).message;
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!admin) return;
		if (!confirm(`Delete ${admin.name}? Sessions already playing it keep their copy.`)) return;
		busy = true;
		error = null;
		try {
			await unwrap(client.mutation(DELETE_MAP_MUTATION, { id: admin.id }));
			notice = `Deleted ${admin.name}.`;
			admin = null;
			selected = '';
			draft = '';
			const list = await unwrap<{ maps: GameMap[] }>(client.query(MAP_SUMMARIES_QUERY, {}));
			summaries = sortMaps(list.maps as unknown as GameMap[]);
		} catch (e) {
			error = describeError(e).message;
		} finally {
			busy = false;
		}
	}

	/** A preview state over the saved map, so the canvas shows its shape. */
	const preview = $derived<GameState | null>(admin ? emptyState(admin as unknown as GameMap) : null);
</script>

<svelte:head><title>{pageTitle('Editor')}</title></svelte:head>

<div class="mx-auto max-w-7xl px-4 sm:px-6 py-8">
	<h1>Map editor</h1>
	<p class="lede">
		Maps are JSON: <code>points</code> (ids and layout bands) and <code>edges</code> (endpoints, how
		many times each must be drawn, and optional one-way constraints). Editing needs an admin key.
	</p>

	<div class="keybar">
		<label for="admin-key">Admin key</label>
		<input id="admin-key" type="password" bind:value={key} placeholder="X-Admin-Key" autocomplete="off" />
		<button type="button" class="btn" onclick={saveKey}>Use key</button>
		{#if notice}<span class="notice">{notice}</span>{/if}
	</div>

	{#if error}<p class="error" role="alert">{error}</p>{/if}

	<div class="layout">
		<section class="editor" aria-label="Map source">
			<div class="row">
				<label for="map">Map</label>
				<select id="map" bind:value={selected} onchange={() => selected && load(selected)}>
					{#each summaries as m (m.id)}<option value={m.id}>{m.name}</option>{/each}
				</select>
				<button type="button" class="btn" onclick={() => { selected = ''; admin = null; draft = blankMap(); validation = null; }}>New</button>
			</div>
			<textarea bind:value={draft} spellcheck="false" rows="26" aria-label="Map JSON" data-testid="map-json"></textarea>
			<div class="actions">
				<button type="button" class="btn" onclick={validate} disabled={busy} data-testid="validate">Validate</button>
				<button type="button" class="btn primary" onclick={() => save(false)} disabled={busy || !admin}>{busy ? 'Saving…' : 'Save'}</button>
				<button type="button" class="btn" onclick={() => save(true)} disabled={busy}>Save as new</button>
				{#if admin}<button type="button" class="btn danger" onclick={remove} disabled={busy}>Delete</button>{/if}
			</div>

			{#if validation}
				<div class="validation {validation.valid && validation.solvable ? 'ok' : 'bad'}" data-testid="validation">
					<p class="vh">{validation.valid ? 'Parses' : 'Invalid'}{validation.valid ? (validation.solvable ? ' and solvable' : ', but no single trail exists') : ''}</p>
					{#if validation.verdict}<p>Verdict: <strong>{validation.verdict}</strong>{#if validation.parMoves !== null}; complete trail {validation.parMoves} moves{/if}</p>{/if}
					{#if validation.issues.length}
						<ul>{#each validation.issues as issue (issue.message)}<li>{issue.message}</li>{/each}</ul>
					{/if}
				</div>
			{/if}
		</section>

		<section class="preview" aria-label="Preview">
			<h2>Preview</h2>
			{#if admin && preview}
				<Board map={admin as unknown as GameMap} game={preview} interactive={false} draggable />
				<p class="meta">Saved map: {admin.nodeCount} nodes, {admin.edgeCount} edges, {admin.stepCount} traversals.</p>
			{:else}
				<p class="muted">Save or select a map to see it here.</p>
			{/if}
		</section>
	</div>
</div>



<style>
	h1 {
		margin: 0 0 0.4rem;
		font-size: 1.75rem;
		color: #2b1f7a;
	}
	.lede {
		margin: 0 0 1rem;
		max-width: 48rem;
		font-size: 0.88rem;
		line-height: 1.6;
		color: #374151;
	}
	code {
		background: #eef0fa;
		border-radius: 0.25rem;
		padding: 0.05rem 0.3rem;
		font-size: 0.85em;
	}
	.keybar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 1rem;
		font-size: 0.82rem;
		color: #4b5563;
	}
	input[type='password'] {
		border: 1px solid #a9b1e8;
		border-radius: 0.5rem;
		padding: 0.4rem 0.6rem;
		font-size: 0.82rem;
		min-width: 16rem;
	}
	.notice {
		color: #15803d;
		font-size: 0.78rem;
	}
	.layout {
		display: grid;
		gap: 1.25rem;
		grid-template-columns: minmax(0, 1fr);
	}
	@media (min-width: 1024px) {
		.layout {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		}
	}
	.row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.6rem;
		font-size: 0.82rem;
		color: #4b5563;
	}
	select {
		border: 1px solid #a9b1e8;
		border-radius: 0.5rem;
		padding: 0.4rem 0.6rem;
		font-size: 0.82rem;
		background: #ffffff;
		min-width: 12rem;
	}
	textarea {
		width: 100%;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.74rem;
		line-height: 1.5;
		border: 1px solid #c7cdf5;
		border-radius: 0.6rem;
		padding: 0.6rem;
		background: #fbfbff;
		color: #1f2937;
		resize: vertical;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.6rem;
	}
	.btn {
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.5rem;
		padding: 0.4rem 0.85rem;
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
	.btn.danger {
		color: #b91c1c;
		border-color: #fca5a5;
	}
	.btn.danger:hover:not(:disabled) {
		background: #fef2f2;
	}
	.validation {
		margin-top: 0.75rem;
		border-radius: 0.6rem;
		padding: 0.6rem 0.8rem;
		font-size: 0.78rem;
	}
	.validation.ok {
		background: #ecfdf3;
		border: 1px solid #a7f3c6;
		color: #14532d;
	}
	.validation.bad {
		background: #fef2f2;
		border: 1px solid #fca5a5;
		color: #7f1d1d;
	}
	.vh {
		margin: 0 0 0.25rem;
		font-weight: 700;
	}
	.validation ul {
		margin: 0.3rem 0 0;
		padding-left: 1.2rem;
	}
	.preview h2 {
		margin: 0 0 0.5rem;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: #4b3fb0;
	}
	.meta {
		margin: 0.5rem 0 0;
		font-size: 0.75rem;
		color: #6b7280;
	}
	.muted {
		color: #6b7280;
		font-size: 0.85rem;
	}
	.error {
		border: 1px solid #fca5a5;
		background: #fef2f2;
		color: #991b1b;
		border-radius: 0.6rem;
		padding: 0.6rem 0.9rem;
		font-size: 0.85rem;
		margin-bottom: 0.75rem;
	}
</style>
