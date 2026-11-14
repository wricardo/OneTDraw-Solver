<script lang="ts">
	import type { ConnectionState } from '$lib/types';

	let { state }: { state: ConnectionState } = $props();

	const label = $derived(
		state === 'live'
			? 'Live'
			: state === 'connecting'
				? 'Connecting…'
				: state === 'reconnecting'
					? 'Reconnecting…'
					: 'Offline'
	);
</script>

<span class="badge {state}" data-testid="connection" title="Live updates: {label}">
	<span class="dot"></span>{label}
</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		border-radius: 999px;
		padding: 0.2rem 0.6rem;
		font-size: 0.7rem;
		font-weight: 600;
		border: 1px solid #d3d8f0;
		background: #f8f9ff;
		color: #4b5563;
	}
	.dot {
		width: 0.45rem;
		height: 0.45rem;
		border-radius: 999px;
		background: #9ca3af;
	}
	.live {
		background: #ecfdf3;
		border-color: #a7f3c6;
		color: #15803d;
	}
	.live .dot {
		background: #16a34a;
	}
	.reconnecting .dot,
	.connecting .dot {
		background: #f59e0b;
	}
</style>
