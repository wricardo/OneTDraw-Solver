// Catalog ordering and grouping, shared by the home page, the maps page and the
// "next map" button so every list agrees on what "next" means.

import type { GameMap } from './types';

/** Difficulty ranks, easiest first. Unknown values sort last. */
const RANK: Record<string, number> = { easy: 0, medium: 1, hard: 2 };

/** Orders maps by difficulty, then by name, so a list reads as a curriculum. */
export function sortMaps(maps: GameMap[]): GameMap[] {
	return [...maps].sort((a, b) => {
		const ra = RANK[a.difficulty] ?? 9;
		const rb = RANK[b.difficulty] ?? 9;
		if (ra !== rb) return ra - rb;
		return a.name.localeCompare(b.name);
	});
}

/** Returns the map after mapID in the given order, wrapping around; null when unknown. */
export function nextMap(maps: GameMap[], mapID: string): GameMap | null {
	const ordered = sortMaps(maps);
	const i = ordered.findIndex((m) => m.id === mapID);
	if (i < 0) return null;
	// Skip maps with no single-trail solution: offering one as "next" after a win
	// suggests it can be beaten, which the parity analysis proves it cannot.
	for (let step = 1; step <= ordered.length; step++) {
		const candidate = ordered[(i + step) % ordered.length];
		if (candidate && candidate.parMoves !== null) return candidate;
	}
	return null;
}

/** Groups maps by difficulty, preserving the curriculum order. */
export function groupByDifficulty(maps: GameMap[]): { difficulty: string; maps: GameMap[] }[] {
	const order = ['easy', 'medium', 'hard'];
	const groups = new Map<string, GameMap[]>();
	for (const m of sortMaps(maps)) {
		const key = m.difficulty || 'other';
		const list = groups.get(key);
		if (list) list.push(m);
		else groups.set(key, [m]);
	}
	const keys = [...groups.keys()].sort((a, b) => {
		const ra = order.indexOf(a);
		const rb = order.indexOf(b);
		return (ra < 0 ? 9 : ra) - (rb < 0 ? 9 : rb);
	});
	return keys.map((difficulty) => ({ difficulty, maps: groups.get(difficulty) ?? [] }));
}

/** Human label for a difficulty value. */
export function difficultyLabel(d: string): string {
	const s = (d || '').trim();
	if (!s) return 'Unrated';
	return s.charAt(0).toUpperCase() + s.slice(1);
}
