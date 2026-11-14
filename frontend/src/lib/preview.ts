// Preview states: a renderable GameState for a map that has no session.
//
// The board and mini-map take a GameState so one drawing path serves real games
// and previews. A preview is simply a state where nothing has been drawn yet.

import type { GameMap, GameState } from './types';

/** A fresh, unstarted state for map. */
export function emptyState(map: GameMap): GameState {
	return {
		pos: null,
		status: 'READY',
		lossReason: null,
		moves: 0,
		blocked: 0,
		remaining: map.stepCount,
		path: [],
		usage: map.edges.map((e) => e.count)
	};
}

/** A minimal valid map, as the JSON a new map in the editor starts from. */
export function blankMap(): string {
	return JSON.stringify(
		{
			id: 'new_map',
			name: 'New Map',
			description: '',
			difficulty: 'easy',
			Points: [
				{ Point: 1, Level: 1 },
				{ Point: 2, Level: 1 },
				{ Point: 3, Level: 1 }
			],
			Edges: [
				{ PointA: 1, PointB: 2, Count: 1 },
				{ PointA: 2, PointB: 3, Count: 1 },
				{ PointA: 3, PointB: 1, Count: 1 }
			]
		},
		null,
		2
	);
}
