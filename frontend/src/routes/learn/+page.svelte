<script lang="ts">
	import { pageTitle } from '$lib/brand';
</script>

<svelte:head><title>{pageTitle('How it works')}</title></svelte:head>

<div class="mx-auto max-w-3xl px-4 sm:px-6 py-8 prose">
	<h1>How One T Draw works</h1>

	<p>
		A map is a graph: dots (nodes) joined by lines (edges). Your pen starts on one dot and you draw
		one continuous trail. The rule is simple and unforgiving: <strong>every edge must be drawn
		exactly as many times as its number says.</strong> Most edges are drawn once; an edge labelled
		<code>2</code> must be drawn twice. You may cross a dot as often as you like.
	</p>

	<h2>The rules</h2>
	<ul>
		<li>Click a dot to start the trail. Starting is not counted as a move.</li>
		<li>Click a neighbour to draw the edge between them. Each drawn edge costs one move.</li>
		<li>An edge cannot be drawn more times than its count. When it is used up it turns grey.</li>
		<li>A one-way edge is drawn with an arrowhead near the node it leads to; it can only be crossed in that direction.</li>
		<li>A dot and a dot that do not share an edge cannot be connected directly.</li>
		<li>Draw the last outstanding edge and you win.</li>
		<li>If the dot you are on has no usable edge left while edges remain, you are stuck and the game ends. Undo or reset.</li>
	</ul>

	<h2>Why some maps are impossible</h2>
	<p>
		Euler solved this in 1736. Count how many edge-ends meet at each dot — its <em>degree</em>,
		counting a doubled edge twice. A single trail can cover every edge only if:
	</p>
	<ul>
		<li>the edges form one connected group, and</li>
		<li>either no dot has an odd degree (a closed tour: any dot may start), or exactly two do (an open trail: it must start at one of them).</li>
	</ul>
	<p>
		Four or more odd dots and no single trail exists, no matter how cleverly you draw. One bundled
		map, <strong>Jose</strong>, is like that on purpose: its analysis says <code>IMPOSSIBLE</code>
		and it has no solution. One-way edges break the simple parity rule, so maps containing them are
		classified <code>DIRECTED</code>, and the solver decides them by search.
	</p>

	<h2>Every solution is the same length</h2>
	<p>
		A complete trail draws every edge exactly its count of times, so it always makes exactly the
		same number of moves. That number is what the game calls the <strong>complete trail</strong>
		length. There is no "shortcut": solving a map means drawing exactly that many edges. What differs
		is <em>which order</em> you draw them in, and whether you strand a group of edges behind you.
	</p>

	<h2>The solver</h2>
	<p>
		Press <kbd>S</kbd> or <strong>Show solution</strong> and the same engine that judges your game
		searches for a completion from wherever your trail stands. It appears as a dashed overlay, and
		<strong>Draw it for me</strong> replays it. This is a hint, not a skip: it draws the edges through
		the ordinary move API, so a finished game is still a real game.
	</p>
	<p>
		Under the hood the search prunes aggressively. If the edges still owed can no longer all be
		reached from the pen, the branch is dead and is abandoned. If only undirected edges remain, the
		parity rule prunes further: a completion from here leaves at most two odd dots, and if two, the
		pen is on one of them. Neither test can discard a real solution.
	</p>

	<h2>Playing against an AI</h2>
	<p>
		Every session is a public GraphQL resource. Press <strong>Play with an AI</strong> on a game page
		to copy a prompt that hands the session to an agent with web access. The agent reads the graph,
		draws edges through the API, and you can watch it live on the session's spectator link. The
		complete agent guide lives at <a href="/llms.txt">/llms.txt</a>.
	</p>
</div>

<style>
	.prose {
		color: #1f2937;
		line-height: 1.7;
	}
	h1 {
		font-size: 1.75rem;
		color: #2b1f7a;
		margin-bottom: 1rem;
	}
	h2 {
		font-size: 1.15rem;
		color: #2b1f7a;
		margin: 1.75rem 0 0.5rem;
	}
	p {
		margin: 0 0 0.9rem;
		font-size: 0.95rem;
	}
	ul {
		margin: 0 0 1rem;
		padding-left: 1.35rem;
		font-size: 0.92rem;
	}
	li {
		margin-bottom: 0.3rem;
	}
	code {
		background: #eef0fa;
		border-radius: 0.25rem;
		padding: 0.05rem 0.3rem;
		font-size: 0.85em;
	}
	kbd {
		border: 1px solid #d3d8f0;
		border-radius: 0.3rem;
		padding: 0 0.3rem;
		background: #f8f9ff;
		font-size: 0.8em;
	}
</style>
