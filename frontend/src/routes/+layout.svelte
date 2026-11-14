<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import { setContextClient } from '@urql/svelte';
	import { makeClient } from '$lib/graphql';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';
	import { GAME, SITE, TITLE } from '$lib/brand';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Play' },
		{ href: '/maps', label: 'Maps' },
		{ href: '/solver', label: 'Solver' },
		{ href: '/sessions', label: 'Sessions' },
		{ href: '/learn', label: 'How it works' },
		{ href: '/editor', label: 'Editor' }
	];

	/** Home and /play/* are "Play"; watching a session belongs to "Sessions". */
	function isActive(href: string, pathname: string): boolean {
		if (href === '/') return pathname === '/' || pathname.startsWith('/play/');
		if (href === '/sessions' && pathname.startsWith('/watch/')) return true;
		return pathname === href || pathname.startsWith(`${href}/`);
	}

	const footer: { title: string; links: { href: string; label: string; external?: boolean }[] }[] = [
		{
			title: 'Play',
			links: [
				{ href: '/', label: 'Home' },
				{ href: '/maps', label: 'Maps' },
				{ href: '/sessions', label: 'Sessions' },
				{ href: '/editor', label: 'Editor' }
			]
		},
		{
			title: 'Docs',
			links: [
				{ href: '/learn', label: 'How it works' },
				{ href: '/llms.txt', label: '/llms.txt', external: true },
				{ href: '/graphql', label: 'GraphQL endpoint', external: true }
			]
		},
		{ title: 'Tools', links: [{ href: '/playground', label: 'GraphQL Playground', external: true }] }
	];

	// ssr=false so this always runs in the browser; safe to set the client synchronously.
	if (browser) {
		setContextClient(makeClient());
	}
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{TITLE}</title>
</svelte:head>

<div class="min-h-screen flex flex-col">
	<a
		href="#main"
		class="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:bg-white focus:px-3 focus:py-2 focus:rounded"
		>Skip to content</a
	>
	<header class="border-b border-indigo-200 bg-white px-4 sm:px-6 py-3 flex flex-wrap items-center justify-between gap-2">
		<a href="/" class="flex items-center gap-2 no-underline">
			<img src={favicon} alt="" class="h-7 w-7" />
			<span class="text-lg font-semibold tracking-wide text-indigo-900">{SITE}</span>
			<span class="hidden sm:inline border-l border-indigo-200 pl-2 text-sm text-indigo-600">{GAME}</span>
		</a>
		<nav class="flex flex-wrap items-center gap-1 text-sm" aria-label="Main">
			{#each navItems as item}
				{@const active = isActive(item.href, $page.url.pathname)}
				<a
					href={item.href}
					aria-current={active ? 'page' : undefined}
					class="px-3 py-1.5 rounded-full transition-colors {active
						? 'bg-indigo-600 text-white'
						: 'text-indigo-900 hover:bg-indigo-100'}">{item.label}</a
				>
			{/each}
		</nav>
	</header>

	<main id="main" class="flex-1">
		<!-- Keyed on the path so /play/a -> /play/b (e.g. "Next map") mounts a fresh page. -->
		{#key $page.url.pathname}
			{@render children()}
		{/key}
	</main>

	<footer class="border-t border-indigo-200 bg-white">
		<div class="mx-auto grid max-w-6xl gap-8 px-4 sm:px-6 py-8 text-sm sm:grid-cols-2 lg:grid-cols-4">
			<div>
				<p class="flex items-center gap-2 font-semibold text-indigo-950"><img src={favicon} alt="" class="h-5 w-5" /> {TITLE}</p>
				<p class="mt-2 text-xs leading-relaxed text-slate-600">Draw every edge of the figure in one stroke.</p>
			</div>
			{#each footer as col (col.title)}
				<nav aria-label="Footer: {col.title}">
					<p class="text-xs font-semibold uppercase tracking-widest text-indigo-900">{col.title}</p>
					<ul class="mt-2 space-y-1.5">
						{#each col.links as l (l.href)}
							<li>
								<a
									href={l.href}
									target={l.external ? '_blank' : undefined}
									rel={l.external ? 'noreferrer' : undefined}
									class="text-slate-600 hover:text-indigo-800 hover:underline">{l.label}</a
								>
							</li>
						{/each}
					</ul>
				</nav>
			{/each}
		</div>
	</footer>
</div>
