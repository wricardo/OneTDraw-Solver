var e={NOT_FOUND:`Not found`,SESSION_TERMINAL:`Game already finished`,INVALID_ARGUMENT:`That is not allowed`,FORBIDDEN:`Not allowed`,LIMIT_REACHED:`Too many games`,INTERNAL:`Server error`,GRAPHQL_VALIDATION_FAILED:`Invalid request`};function t(t){let n=t,r=n?.graphQLErrors??[];if(r.length>0){let t=r[0],n=t.extensions?.code;return{title:(n&&e[n])??`Request failed`,message:t.message??``,code:n??void 0}}return n?.networkError?{title:`Cannot reach the server`,message:n.networkError.message||`The API did not respond.`}:n?.message?{title:`Request failed`,message:n.message}:{title:`Request failed`,message:String(t??`unknown error`)}}async function n(e){let t=await e.toPromise();if(t.error)throw t.error;if(t.data==null)throw Error(`the server returned no data`);return t.data}function r(e){return e===`DEAD_END`?`You got stuck with edges still to draw.`:`The trail ended early.`}var i=`id level x y`,a=`index a b count oneWay from to`,o=`nodeCount edgeCount stepCount connected directional oddNodes startNodes verdict message`,s=`
  id
  name
  description
  difficulty
  nodeCount
  edgeCount
  stepCount
  levelCount
  maxMoves
  parMoves
  analysis { ${o} }
  nodes { ${i} }
  edges { ${a} }
`,c=`
  move
  node
  from
  to
  kind
  edge
  blocked
  reason
  status
  lossReason
`,l=`
  id
  displayName
  mapId
  map { ${s} }
  state { pos status lossReason moves blocked remaining path usage }
  seq
  createdAt
  lastActionAt
  attempts
  bestMoves
  lastEvent { ${c} }
  lastPlan { start steps found computedAt }
  autoPlay
`,u=`
  query Maps {
    maps { ${s} }
  }
`,d=`
  query MapSummaries {
    maps {
      id
      name
      description
      difficulty
      nodeCount
      edgeCount
      stepCount
      levelCount
      parMoves
      analysis { verdict message oddNodes startNodes directional }
    }
  }
`,f=`
  id
  name
  description
  difficulty
  nodeCount
  edgeCount
  stepCount
  levelCount
  maxMoves
  parMoves
  analysis { ${o} }
  nodes { ${i} }
  edges { ${a} }
  config {
    id
    name
    description
    difficulty
    maxMoves
    points { point level hidden x y }
    edges { pointA pointB count from to }
  }
`,p=`
  query AdminMap($id: ID!) {
    adminMap(id: $id) { ${f} }
  }
`,m=`
  query Session($id: ID!) {
    session(id: $id) { ${l} }
  }
`,h=`
  query Sessions($sort: SessionSort, $limit: Int) {
    sessions(sort: $sort, limit: $limit) {
      id
      displayName
      mapId
      createdAt
      lastActionAt
      attempts
      bestMoves
      map { name parMoves stepCount }
      state { status moves remaining }
    }
  }
`,g=`
  mutation CreateSession($mapID: ID!, $displayName: String) {
    createSession(mapID: $mapID, displayName: $displayName) { ${l} }
  }
`,_=`
  mutation DeleteSession($id: ID!) {
    deleteSession(id: $id)
  }
`,v=`
  mutation Start($sessionID: ID!, $node: Int!) {
    start(sessionID: $sessionID, node: $node) { ${l} }
  }
`,y=`
  mutation Move($sessionID: ID!, $node: Int!) {
    move(sessionID: $sessionID, node: $node) { ${l} }
  }
`,b=`
  mutation BulkMove($sessionID: ID!, $nodes: [Int!]!, $stopOnBlocked: Boolean) {
    bulkMove(sessionID: $sessionID, nodes: $nodes, stopOnBlocked: $stopOnBlocked) { ${l} }
  }
`,x=`
  mutation Undo($sessionID: ID!) {
    undo(sessionID: $sessionID) { ${l} }
  }
`,S=`
  mutation Reset($sessionID: ID!) {
    reset(sessionID: $sessionID) { ${l} }
  }
`,C=`
  mutation Solve($sessionID: ID!) {
    solve(sessionID: $sessionID) { ${l} }
  }
`,w=`
  subscription SessionUpdated($sessionID: ID!) {
    sessionUpdated(sessionID: $sessionID) {
      seq
      event { ${c} }
      session { ${l} }
    }
  }
`,T=`
  mutation CreateMap($map: MapInput!) {
    createMap(map: $map) { ${f} }
  }
`,E=`
  mutation UpdateMap($map: MapInput!) {
    updateMap(map: $map) { ${f} }
  }
`,D=`
  mutation DeleteMap($id: ID!) {
    deleteMap(id: $id)
  }
`,O=`
  mutation ValidateMap($map: MapInput!) {
    validateMap(map: $map) {
      valid
      issues { message }
      solvable
      parMoves
      verdict
    }
  }
`;export{E as _,D as a,r as b,d as c,h as d,m as f,x as g,v as h,g as i,y as l,C as m,b as n,_ as o,w as p,T as r,u as s,p as t,S as u,O as v,n as x,t as y};