<script lang="ts">
	let {
		onundo,
		onreset,
		onsolve,
		disabled = false,
		solveDisabled = false
	}: {
		onundo: () => void;
		onreset: () => void;
		onsolve: () => void;
		disabled?: boolean;
		solveDisabled?: boolean;
	} = $props();
</script>

<div class="controls" data-testid="controls">
	<button type="button" class="btn" onclick={onundo} {disabled} data-testid="undo">
		Undo <kbd>Z</kbd>
	</button>
	<button type="button" class="btn" onclick={onreset} {disabled} data-testid="reset">
		Reset <kbd>R</kbd>
	</button>
	<button type="button" class="btn primary" onclick={onsolve} disabled={solveDisabled} data-testid="solve">
		Show solution <kbd>S</kbd>
	</button>
</div>

<style>
	.controls {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
	}
	.btn {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.6rem;
		padding: 0.55rem 0.9rem;
		font-size: 0.875rem;
		font-weight: 600;
		cursor: pointer;
	}
	.btn:hover:not(:disabled) {
		background: #f3f0ff;
	}
	.btn:disabled {
		opacity: 0.5;
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
	kbd {
		font-size: 0.7rem;
		border: 1px solid #d3d8f0;
		border-radius: 0.3rem;
		padding: 0 0.25rem;
		background: #f8f9ff;
		color: #4b5563;
	}
	.btn.primary kbd {
		background: #5b4fc4;
		border-color: #6d5ce0;
		color: #e8e6ff;
	}
</style>
