<script lang="ts">
	let { prompt, blurb }: { prompt: string; blurb: string } = $props();
	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(prompt);
			copied = true;
			setTimeout(() => (copied = false), 2000);
		} catch {
			// Clipboard access can be refused; the textarea is still selectable.
			copied = false;
		}
	}
</script>

<section class="ai-card" data-testid="ai-prompt">
	<h2>Play with an AI</h2>
	<p>{blurb}</p>
	<button type="button" class="copy" onclick={copy} data-testid="copy-prompt">
		{copied ? 'Copied' : 'Copy prompt'}
	</button>
	<details>
		<summary>Show the prompt</summary>
		<textarea readonly rows="10" aria-label="Prompt for an AI assistant">{prompt}</textarea>
	</details>
</section>

<style>
	.ai-card {
		border: 1px solid #c7cdf5;
		border-radius: 0.75rem;
		background: #ffffff;
		padding: 0.85rem 1rem;
	}
	h2 {
		margin: 0 0 0.35rem;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: #4b3fb0;
	}
	p {
		margin: 0 0 0.6rem;
		font-size: 0.8rem;
		color: #4b5563;
	}
	.copy {
		border: 1px solid #a9b1e8;
		background: #f3f0ff;
		color: #2b1f7a;
		border-radius: 0.5rem;
		padding: 0.4rem 0.8rem;
		font-size: 0.8rem;
		font-weight: 600;
		cursor: pointer;
	}
	details {
		margin-top: 0.6rem;
	}
	summary {
		font-size: 0.75rem;
		cursor: pointer;
		color: #4b5563;
	}
	textarea {
		width: 100%;
		margin-top: 0.4rem;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.7rem;
		border: 1px solid #d3d8f0;
		border-radius: 0.5rem;
		padding: 0.5rem;
		background: #fbfbff;
		color: #1f2937;
	}
</style>
