import { describe, expect, it } from 'vitest';
import { difficultyLabel, groupByDifficulty, nextMap, sortMaps } from './maps';
import type { GameMap } from './types';

function m(id: string, name: string, difficulty: string, parMoves: number | null = 10): GameMap {
	return {
		id,
		name,
		difficulty,
		description: '',
		nodeCount: 1,
		edgeCount: 1,
		stepCount: 1,
		levelCount: 1,
		maxMoves: 1,
		parMoves,
		analysis: {
			nodeCount: 1,
			edgeCount: 1,
			stepCount: 1,
			connected: true,
			directional: false,
			oddNodes: [],
			startNodes: [],
			verdict: parMoves === null ? 'IMPOSSIBLE' : 'CIRCUIT',
			message: ''
		},
		nodes: [],
		edges: []
	};
}

describe('map ordering', () => {
	it('sorts by difficulty then name', () => {
		const got = sortMaps([m('h1', 'Zeta', 'hard'), m('e1', 'Beta', 'easy'), m('m1', 'Alpha', 'medium'), m('e2', 'Alpha', 'easy')]);
		expect(got.map((x) => x.id)).toEqual(['e2', 'e1', 'm1', 'h1']);
	});

	it('groups by difficulty in curriculum order', () => {
		const groups = groupByDifficulty([m('a', 'A', 'hard'), m('b', 'B', 'easy'), m('c', 'C', 'medium')]);
		expect(groups.map((g) => g.difficulty)).toEqual(['easy', 'medium', 'hard']);
	});

	it('labels a difficulty, defaulting to Unrated', () => {
		expect(difficultyLabel('hard')).toBe('Hard');
		expect(difficultyLabel('')).toBe('Unrated');
	});
});

describe('nextMap', () => {
	it('returns the following map and wraps around', () => {
		const maps = [m('a', 'A', 'easy'), m('b', 'B', 'easy')];
		expect(nextMap(maps, 'a')?.id).toBe('b');
		expect(nextMap(maps, 'b')?.id).toBe('a');
	});

	it('skips a map that cannot be solved', () => {
		const maps = [m('a', 'A', 'easy'), m('x', 'Broken', 'easy', null), m('b', 'B', 'easy')];
		expect(nextMap(maps, 'a')?.id).toBe('b');
	});

	it('returns null for an unknown map', () => {
		expect(nextMap([m('a', 'A', 'easy')], 'zzz')).toBeNull();
	});
});
