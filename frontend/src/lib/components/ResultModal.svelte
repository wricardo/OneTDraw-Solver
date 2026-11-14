<script lang="ts">
	import { lossReasonText } from '$lib/errors';
	import type { Status } from '$lib/types';

	let {
		status,
		reason = null,
		moves,
		par = null,
		best = null,
		nextName = null,
		nextBusy = false,
		onreset,
		onnext,
		onclose
	}: {
		status: Status;
		reason?: string | null;
		moves: number;
		par?: number | null;
		best?: number | null;
		nextName?: string | null;
		nextBusy?: boolean;
		onreset: () => void;
		onnext?: () => void;
		onclose: () => void;
	} = $props();

	const won = $derived(status === 'WON');
</script>

<div class="backdrop" role="presentation" onclick={onclose}>
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="result-title"
		tabindex="-1"
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => {
			if (e.key === 'Escape') onclose();
		}}
		data-testid="result-modal"
	>
		<h2 id="result-title" class:won data-testid="result-title">{won ? 'Solved' : 'Stuck'}</h2>
		<p>
			{#if won}
				You drew every edge in {moves} moves{#if par !== null}
					{par === moves ? ' — the perfect length.' : ` (a complete trail is ${par}).`}{/if}
				{#if best !== null && best < moves}
					Your best on this game is {best}.
				{/if}
			{:else}
				{lossReasonText(reason)} Edges remained after {moves} moves.
			{/if}
		</p>
		<div class="actions">
			<button type="button" class="btn primary" onclick={onreset} data-testid="modal-reset">Try again</button>
			{#if nextName && onnext}
				<button type="button" class="btn" onclick={onnext} disabled={nextBusy} data-testid="modal-next">
					{nextBusy ? 'Loading…' : `Next: ${nextName}`}
				</button>
			{/if}
			<button type="button" class="btn ghost" onclick={onclose} data-testid="modal-close">Close</button>
		</div>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(30, 27, 75, 0.55);
		display: grid;
		place-items: center;
		padding: 1rem;
		z-index: 50;
	}
	.dialog {
		background: #ffffff;
		border-radius: 1rem;
		padding: 1.5rem;
		max-width: 26rem;
		width: 100%;
		box-shadow: 0 24px 60px rgba(30, 27, 75, 0.35);
	}
	h2 {
		margin: 0 0 0.5rem;
		font-size: 1.35rem;
		color: #b91c1c;
	}
	h2.won {
		color: #15803d;
	}
	p {
		margin: 0 0 1rem;
		font-size: 0.9rem;
		color: #374151;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.btn {
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.6rem;
		padding: 0.5rem 0.9rem;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
	}
	.btn.primary {
		background: #4b3fb0;
		border-color: #3a3090;
		color: #ffffff;
	}
	.btn.ghost {
		background: transparent;
		border-color: transparent;
		color: #4b5563;
	}
</style>
