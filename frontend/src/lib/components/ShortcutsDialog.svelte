<script lang="ts">
	import { cx } from '$lib/ui';

	let { open = false, onclose }: { open: boolean; onclose: () => void } = $props();

	const rows: { keys: string; what: string }[] = [
		{ keys: 'Click a node', what: 'Pick it as the start, or draw the edge to it' },
		{ keys: 'Z / Backspace', what: 'Take back the last drawn edge' },
		{ keys: 'R', what: 'Reset the trail to the start' },
		{ keys: 'S', what: 'Show the solver’s completion' },
		{ keys: '?', what: 'This dialog' },
		{ keys: 'Esc', what: 'Close a dialog' }
	];
</script>

{#if open}
	<div class="backdrop" role="presentation" onclick={onclose}>
		<div
			class="dialog"
			role="dialog"
			aria-modal="true"
			aria-labelledby="shortcuts-title"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => {
				if (e.key === 'Escape') onclose();
			}}
			data-testid="shortcuts"
		>
			<h2 id="shortcuts-title">Shortcuts</h2>
			<table>
				<tbody>
					{#each rows as row (row.keys)}
						<tr>
							<th scope="row"><kbd class={cx('kbd')}>{row.keys}</kbd></th>
							<td>{row.what}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<button type="button" onclick={onclose} class="close" data-testid="shortcuts-close">Close</button>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(30, 27, 75, 0.5);
		display: grid;
		place-items: center;
		padding: 1rem;
		z-index: 60;
	}
	.dialog {
		background: #ffffff;
		border-radius: 1rem;
		padding: 1.25rem;
		width: 100%;
		max-width: 30rem;
	}
	h2 {
		margin: 0 0 0.75rem;
		font-size: 1.1rem;
		color: #2b1f7a;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		text-align: left;
		padding: 0.35rem 0.4rem;
		font-size: 0.85rem;
		color: #374151;
		vertical-align: top;
	}
	th {
		width: 45%;
		font-weight: 500;
	}
	.kbd {
		background: #f3f0ff;
		border: 1px solid #d3d8f0;
		border-radius: 0.3rem;
		padding: 0.1rem 0.35rem;
		font-size: 0.75rem;
	}
	.close {
		margin-top: 0.75rem;
		border: 1px solid #a9b1e8;
		background: #ffffff;
		color: #2b1f7a;
		border-radius: 0.5rem;
		padding: 0.4rem 0.9rem;
		font-size: 0.85rem;
		cursor: pointer;
	}
</style>
