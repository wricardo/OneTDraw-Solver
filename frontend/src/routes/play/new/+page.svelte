<script lang="ts">
	// /play/new?map=<id> creates a session for that map and redirects to it, so a
	// map link can be a plain anchor. It is a route (not a component) so the
	// browser's back button and the URL both behave sensibly.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContextClient } from '@urql/svelte';
	import { describeError } from '$lib/errors';
	import { startSession } from '$lib/start';

	const client = getContextClient();
	let error = $state<string | null>(null);

	onMount(async () => {
		const mapID = $page.url.searchParams.get('map') ?? '';
		if (!mapID) {
			error = 'No map was chosen.';
			return;
		}
		try {
			await goto(`/play/${await startSession(client, mapID)}`, { replaceState: true });
		} catch (e) {
			error = describeError(e).message;
		}
	});
</script>

<div class="wrap">
	{#if error}
		<p role="alert" class="error">{error}. <a href="/maps" class="underline">Pick a map</a></p>
	{:else}
		<p role="status" class="muted">Starting a new game…</p>
	{/if}
</div>

<style>
	.wrap {
		max-width: 40rem;
		margin: 3rem auto;
		padding: 0 1rem;
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
