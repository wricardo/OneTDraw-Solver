// GraphQL documents. Fields requested here must exist in graph/schema.graphqls.

const NODE_FIELDS = `id level x y`;
const EDGE_FIELDS = `index a b count oneWay from to`;
const ANALYSIS_FIELDS = `nodeCount edgeCount stepCount connected directional oddNodes startNodes verdict message`;

/** Full map: graph plus the analysis the board draws from. */
const MAP_FIELDS = `
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
  analysis { ${ANALYSIS_FIELDS} }
  nodes { ${NODE_FIELDS} }
  edges { ${EDGE_FIELDS} }
`;

const STEP_EVENT_FIELDS = `
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
`;

const PLAN_FIELDS = `start steps found computedAt`;

/** Everything the board, counters and solver panel need. */
export const SESSION_FIELDS = `
  id
  displayName
  mapId
  map { ${MAP_FIELDS} }
  state { pos status lossReason moves blocked remaining path usage }
  seq
  createdAt
  lastActionAt
  attempts
  bestMoves
  lastEvent { ${STEP_EVENT_FIELDS} }
  lastPlan { ${PLAN_FIELDS} }
  autoPlay
`;

export const MAPS_QUERY = `
  query Maps {
    maps { ${MAP_FIELDS} }
  }
`;

export const MAP_QUERY = `
  query Map($id: ID!) {
    map(id: $id) { ${MAP_FIELDS} }
  }
`;

/** Lightweight catalog for list pages: no per-edge detail. */
export const MAP_SUMMARIES_QUERY = `
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
`;

export const ADMIN_MAP_FIELDS = `
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
  analysis { ${ANALYSIS_FIELDS} }
  nodes { ${NODE_FIELDS} }
  edges { ${EDGE_FIELDS} }
  config {
    id
    name
    description
    difficulty
    maxMoves
    points { point level hidden x y }
    edges { pointA pointB count from to }
  }
`;

export const ADMIN_MAP_QUERY = `
  query AdminMap($id: ID!) {
    adminMap(id: $id) { ${ADMIN_MAP_FIELDS} }
  }
`;

export const SESSION_QUERY = `
  query Session($id: ID!) {
    session(id: $id) { ${SESSION_FIELDS} }
  }
`;

export const SESSIONS_QUERY = `
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
`;

export const CREATE_SESSION_MUTATION = `
  mutation CreateSession($mapID: ID!, $displayName: String) {
    createSession(mapID: $mapID, displayName: $displayName) { ${SESSION_FIELDS} }
  }
`;

export const DELETE_SESSION_MUTATION = `
  mutation DeleteSession($id: ID!) {
    deleteSession(id: $id)
  }
`;

export const RENAME_SESSION_MUTATION = `
  mutation RenameSession($id: ID!, $displayName: String!) {
    renameSession(id: $id, displayName: $displayName) { ${SESSION_FIELDS} }
  }
`;

export const START_MUTATION = `
  mutation Start($sessionID: ID!, $node: Int!) {
    start(sessionID: $sessionID, node: $node) { ${SESSION_FIELDS} }
  }
`;

export const MOVE_MUTATION = `
  mutation Move($sessionID: ID!, $node: Int!) {
    move(sessionID: $sessionID, node: $node) { ${SESSION_FIELDS} }
  }
`;

export const BULK_MOVE_MUTATION = `
  mutation BulkMove($sessionID: ID!, $nodes: [Int!]!, $stopOnBlocked: Boolean) {
    bulkMove(sessionID: $sessionID, nodes: $nodes, stopOnBlocked: $stopOnBlocked) { ${SESSION_FIELDS} }
  }
`;

export const UNDO_MUTATION = `
  mutation Undo($sessionID: ID!) {
    undo(sessionID: $sessionID) { ${SESSION_FIELDS} }
  }
`;

export const RESET_MUTATION = `
  mutation Reset($sessionID: ID!) {
    reset(sessionID: $sessionID) { ${SESSION_FIELDS} }
  }
`;

export const SOLVE_MUTATION = `
  mutation Solve($sessionID: ID!) {
    solve(sessionID: $sessionID) { ${SESSION_FIELDS} }
  }
`;

export const HINT_QUERY = `
  query Hint($sessionID: ID!) {
    hint(sessionID: $sessionID) { ${PLAN_FIELDS} }
  }
`;

export const SESSION_UPDATED_SUBSCRIPTION = `
  subscription SessionUpdated($sessionID: ID!) {
    sessionUpdated(sessionID: $sessionID) {
      seq
      event { ${STEP_EVENT_FIELDS} }
      session { ${SESSION_FIELDS} }
    }
  }
`;

export const CREATE_MAP_MUTATION = `
  mutation CreateMap($map: MapInput!) {
    createMap(map: $map) { ${ADMIN_MAP_FIELDS} }
  }
`;

export const UPDATE_MAP_MUTATION = `
  mutation UpdateMap($map: MapInput!) {
    updateMap(map: $map) { ${ADMIN_MAP_FIELDS} }
  }
`;

export const DELETE_MAP_MUTATION = `
  mutation DeleteMap($id: ID!) {
    deleteMap(id: $id)
  }
`;

export const VALIDATE_MAP_MUTATION = `
  mutation ValidateMap($map: MapInput!) {
    validateMap(map: $map) {
      valid
      issues { message }
      solvable
      parMoves
      verdict
    }
  }
`;
