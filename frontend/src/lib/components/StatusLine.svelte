<script lang="ts">
	import { progressPercent, statusLabel } from '$lib/hud';
	import type { Session } from '$lib/types';

	let { session }: { session: Session } = $props();
	const state = $derived(session.state);
	const pct = $derived(progressPercent(state));
	const total = $derived(state.moves + state.remaining);
</script>

<div class="status-line" data-testid="status-line">
	<p class="status {state.status.toLowerCase()}">{statusLabel(state)}</p>
	<div class="progress" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={state.moves} aria-label="Traversals drawn">
		<div class="progress-fill" style="width: {pct}%"></div>
	</div>
	<p class="detail">
		<span data-testid="moves">{state.moves}</span> / {total} edges drawn
		{#if state.pos !== null}
			· at node <strong data-testid="pos">{state.pos}</strong>
		{:else}
			· no node placed
		{/if}
	</p>
</div>

<style>
	.status-line {
		border: 1px solid #c7cdf5;
		border-radius: 0.75rem;
		background: #ffffff;
		padding: 0.75rem 1rem;
	}
	.status {
		margin: 0 0 0.5rem;
		font-size: 0.95rem;
		font-weight: 600;
		color: #2b1f7a;
	}
	.status.won {
		color: #15803d;
	}
	.status.lost {
		color: #b91c1c;
	}
	.progress {
		height: 0.5rem;
		background: #e5e7f5;
		border-radius: 999px;
		overflow: hidden;
	}
	.progress-fill {
		height: 100%;
		background: linear-gradient(90deg, #6d5ce0, #16a34a);
		transition: width 180ms ease-out;
	}
	.detail {
		margin: 0.5rem 0 0;
		font-size: 0.8rem;
		color: #4b5563;
	}
</style>
