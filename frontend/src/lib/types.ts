// Hand-written types mirroring graph/schema.graphqls. Keep in sync with the
// schema; the Go side is the source of truth.

export type Status = 'READY' | 'PLAYING' | 'WON' | 'LOST';
export type LossReason = 'DEAD_END';
/** How an attempt changed the trail. */
export type StepKind = 'NONE' | 'DRAW' | 'START';
export type SessionSort = 'RECENT' | 'CREATED';

/** A node of the map. `level` is a layout band; x/y are 0..1000 layout units. */
export interface Node {
	id: number;
	level: number;
	x: number;
	y: number;
}

/** One edge. `count` is how many times it must be drawn. */
export interface Edge {
	index: number;
	a: number;
	b: number;
	count: number;
	oneWay: boolean;
	from: number | null;
	to: number | null;
}

/** Euler pre-flight over the map's graph. */
export interface Analysis {
	nodeCount: number;
	edgeCount: number;
	stepCount: number;
	connected: boolean;
	directional: boolean;
	oddNodes: number[];
	startNodes: number[];
	verdict: 'CIRCUIT' | 'TRAIL' | 'DIRECTED' | 'IMPOSSIBLE';
	message: string;
}

/** Public catalog entry: the graph, no solution. */
export interface GameMap {
	id: string;
	name: string;
	description: string;
	difficulty: string;
	nodeCount: number;
	edgeCount: number;
	stepCount: number;
	levelCount: number;
	maxMoves: number;
	/** Length of a complete trail; null when the map cannot be solved. */
	parMoves: number | null;
	analysis: Analysis;
	nodes: Node[];
	edges: Edge[];
}

/** One point as stored in a map definition, including layout-only spacers. */
export interface MapPoint {
	point: number;
	level: number;
	hidden: boolean;
	x: number;
	y: number;
}

export interface ConfigEdge {
	pointA: number;
	pointB: number;
	count: number;
	from: number | null;
	to: number | null;
}

export interface MapConfig {
	id: string;
	name: string;
	description: string;
	difficulty: string;
	maxMoves: number;
	points: MapPoint[];
	edges: ConfigEdge[];
}

/** Full map for the editor (admin only). */
export interface AdminMap extends Omit<GameMap, 'analysis'> {
	analysis: Analysis;
	config: MapConfig;
}

export interface GameState {
	pos: number | null;
	status: Status;
	lossReason: LossReason | null;
	/** Edges drawn so far (blocked attempts are free). */
	moves: number;
	/** Rejected attempts. */
	blocked: number;
	/** Traversals still owed. */
	remaining: number;
	path: number[];
	/** Traversals still owed per edge, indexed like GameMap.edges. */
	usage: number[];
}

export interface StepEvent {
	move: number;
	node: number;
	from: number | null;
	to: number | null;
	kind: StepKind;
	edge: number | null;
	blocked: boolean;
	reason: string | null;
	status: Status;
	lossReason: LossReason | null;
}

/** A completion plan from the solver. */
export interface Plan {
	start: number;
	steps: number[];
	found: boolean;
	computedAt?: string;
}

export interface Session {
	id: string;
	displayName: string;
	mapId: string;
	map: GameMap;
	state: GameState;
	seq: number;
	createdAt: string;
	lastActionAt: string;
	attempts: number;
	bestMoves: number | null;
	lastEvent: StepEvent | null;
	lastPlan: Plan | null;
	autoPlay: boolean;
}

export interface SessionUpdate {
	seq: number;
	session: Session;
	event: StepEvent | null;
}

/** Lightweight row for the sessions list. */
export interface SessionSummary {
	id: string;
	displayName: string;
	mapId: string;
	createdAt: string;
	lastActionAt: string;
	attempts: number;
	bestMoves: number | null;
	map: { name: string; parMoves: number | null; stepCount: number };
	state: { status: Status; moves: number; remaining: number };
}

export interface MapValidationResult {
	valid: boolean;
	issues: { message: string }[];
	solvable: boolean;
	parMoves: number | null;
	verdict: string | null;
}

export interface EdgeInput {
	pointA: number;
	pointB: number;
	count?: number;
	direction?: { from: number; to: number; unidirectional: boolean };
}

export interface PointInput {
	point: number;
	level?: number;
	hide?: boolean;
}

export interface MapInput {
	id: string;
	name: string;
	description?: string;
	difficulty?: string;
	maxMoves?: number;
	points?: PointInput[];
	edges: EdgeInput[];
}

/** WebSocket connection indicator for spectator/play views. */
export type ConnectionState = 'connecting' | 'live' | 'reconnecting' | 'offline';
