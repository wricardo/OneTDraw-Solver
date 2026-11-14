<script lang="ts">
	import type { Session } from '$lib/types';

	let { session }: { session: Session } = $props();
	const map = $derived(session.map);
	const par = $derived(map.parMoves);
	const best = $derived(session.bestMoves);
</script>

<div class="stats" data-testid="stats">
	<dl>
		<div>
			<dt>Moves</dt>
			<dd data-testid="stat-moves">{session.state.moves}</dd>
		</div>
		<div>
			<dt>Complete trail</dt>
			<dd data-testid="stat-par">{par ?? '—'}</dd>
		</div>
		<div>
			<dt>Your best</dt>
			<dd data-testid="stat-best">{best ?? '—'}</dd>
		</div>
		<div>
			<dt>Attempts</dt>
			<dd data-testid="stat-attempts">{session.attempts}</dd>
		</div>
	</dl>
	<p class="note">
		{#if par === null}
			This map has no complete trail: the parity analysis proves it cannot be drawn in one stroke.
		{:else}
			Every complete trail is exactly {par} moves long, because it draws every edge exactly its count of times.
		{/if}
	</p>
</div>

<style>
	.stats {
		border: 1px solid #c7cdf5;
		border-radius: 0.75rem;
		background: #ffffff;
		padding: 0.75rem 1rem;
	}
	dl {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.5rem;
		margin: 0;
	}
	dt {
		font-size: 0.6rem;
		line-height: 1.2;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: #6b7280;
		min-height: 1.5em;
	}
	dd {
		margin: 0.15rem 0 0;
		font-size: 1.25rem;
		font-weight: 700;
		color: #2b1f7a;
	}
	.note {
		margin: 0.6rem 0 0;
		font-size: 0.75rem;
		color: #4b5563;
	}
</style>
