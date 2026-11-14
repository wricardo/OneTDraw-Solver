import{$ as e,F as t,H as n,L as r,O as i,P as a,Q as o,X as s,Y as c,j as l,k as u,m as d,q as f,st as p,tt as m}from"./BlrRlQ0e.js";import"./xihTtKlq.js";var h=l(`<section class="ai-card svelte-1v7kp0i" data-testid="ai-prompt"><h2 class="svelte-1v7kp0i">Play with an AI</h2> <p class="svelte-1v7kp0i"> </p> <button type="button" class="copy svelte-1v7kp0i" data-testid="copy-prompt"> </button> <details class="svelte-1v7kp0i"><summary class="svelte-1v7kp0i">Show the prompt</summary> <textarea readonly="" rows="10" aria-label="Prompt for an AI assistant" class="svelte-1v7kp0i"></textarea></details></section>`);function g(a,l){let g=e(!1);async function _(){try{await navigator.clipboard.writeText(l.prompt),o(g,!0),setTimeout(()=>o(g,!1),2e3)}catch{o(g,!1)}}var v=h(),y=s(f(v),2),b=c(y,!0),x=s(y,2),S=c(x,!0),C=s(x,2),w=s(f(C),2);m(w),p(C),p(v),n(()=>{i(b,l.blurb),i(S,r(g)?`Copied`:`Copy prompt`),d(w,l.prompt)}),t(`click`,x,_),u(a,v)}a([`click`]);function _(e,t){let n=e.replace(/\/$/,``),r=`${n}/graphql`,i=t.map,a=i.analysis.verdict===`TRAIL`?`This map's parity requires starting at node ${i.analysis.oddNodes.join(` or `)}.`:i.analysis.verdict===`CIRCUIT`?`Any node may start (closed tour).`:i.analysis.verdict===`IMPOSSIBLE`?`This map cannot be solved in one trail; say so rather than guessing.`:`One-way edges decide the start; any node with an edge may work.`;return`You are playing One T Draw: draw ONE continuous trail that covers every edge of the graph
exactly as many times as its count says. You may revisit nodes. Drawing an exhausted edge is a
blocked attempt (free, not counted). If you get stuck with edges still owed, you lose.

GraphQL endpoint: ${r}
A human is watching this session live: ${n}/watch/${t.id}

Session: ${t.id}
Map: ${i.name} (${i.id}) — ${i.edgeCount} edges, ${i.stepCount} traversals required.
A complete trail is exactly ${i.parMoves??`?`} moves long.
${a}

Rules of the API:
- Nodes are integers. Start with start(sessionID, node), which is not a counted move.
- Then move(sessionID, to) to draw each edge. Prefer bulkMove(sessionID, nodes: [...]) for a plan;
  it stops at the first blocked step by default, so check lastEvent.blocked afterwards.
- state.usage[i] is how many times edge i may still be drawn. state.pos is where you are.
- status WON means you covered every edge. status LOST means you are stuck; undo or reset.
- Read the map's edges first: query { map(id: "${i.id}") { nodes { id } edges { index a b count oneWay from to } } }

Steps:
1. Read the map edges.
2. Work out a route that draws every edge the right number of times.
3. mutation { start(sessionID: "${t.id}", node: <first node>) { state { pos status } } }
4. mutation { bulkMove(sessionID: "${t.id}", nodes: [<rest of route>]) {
     state { pos status moves remaining } lastEvent { blocked reason } } }
5. If blocked, read lastEvent.reason and state.pos, correct the route, and continue with bulkMove.
6. Stop when state.status is WON.

If you want a hint without changing the game:
query { hint(sessionID: "${t.id}") { start steps found } }
`}function v(e){let t=e.replace(/\/$/,``);return`Play One T Draw, a single-stroke graph puzzle, through its GraphQL API.

Endpoint: ${t}/graphql
Rules and every call: ${t}/llms.txt

1. Query { maps { id name difficulty stepCount parMoves analysis { verdict oddNodes startNodes } } }
   and pick a map (start with an easy one).
2. mutation { createSession(mapID: "<id>") { id } }
3. Follow ${t}/llms.txt: start a node, then draw every edge exactly count times.
4. Reply with the session id so a human can watch at ${t}/watch/<id>.
`}export{_ as n,g as r,v as t};