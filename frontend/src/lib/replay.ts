// Replaying a state change tile by tile.
//
// A bulkMove arrives as one snapshot, but a viewer should see the pen travel.
// This walks the newly appended part of the trail on a timer and hands back a
// synthetic session whose path is only as long as the animation has got to, so
// the board draws the same thing it would have drawn move by move.

import type { Session } from './types';

export interface ReplayFrame {
	/** The session as it should look right now. */
	session: Session;
	/** True while the animation still has steps to show. */
	replaying: boolean;
}

/** How long each replayed step takes, in milliseconds. */
const STEP_MS = 220;

export interface Replay {
	/** Feeds a new authoritative session; returns nothing. */
	sync(next: Session): void;
	/** The frame to render now. */
	frame(): ReplayFrame | null;
	/** Clears the pending animation and the stored session. */
	destroy(): void;
}

/**
 * Creates a replay controller. It stores the latest authoritative session and,
 * when more steps arrive than it had seen, animates the difference.
 */
export function createReplay(startedAt = 0): Replay {
	let authoritative: Session | null = null;
	let shownLength = 0;
	let animatingUntil = startedAt;
	let scheduled = false;

	function frame(): ReplayFrame | null {
		if (!authoritative) return null;
		const now = Date.now();
		const target = authoritative.state.path.length;
		if (shownLength >= target || now >= animatingUntil) {
			shownLength = target;
			return { session: authoritative, replaying: false };
		}
		// Extrapolate how far the animation has got.
		const elapsed = now - (animatingUntil - (target - shownLength) * STEP_MS);
		const steps = Math.max(0, Math.min(target - shownLength, Math.floor(elapsed / STEP_MS)));
		if (steps === 0) return { session: withPathLength(authoritative, shownLength), replaying: true };
		shownLength += steps;
		return { session: withPathLength(authoritative, shownLength), replaying: shownLength < target };
	}

	function sync(next: Session): void {
		const previousTarget = authoritative ? authoritative.state.path.length : 0;
		authoritative = next;
		const target = next.state.path.length;
		if (target < shownLength || target < previousTarget) {
			// Reset or undo: jump straight to the truth rather than animating back.
			shownLength = target;
			animatingUntil = 0;
			return;
		}
		if (target > shownLength && shownLength === 0) {
			// The very first snapshot is the whole stored trail: do not animate it.
			shownLength = target;
			animatingUntil = 0;
			return;
		}
		if (target > shownLength) {
			if (!scheduled) {
				scheduled = true;
				animatingUntil = Date.now() + (target - shownLength) * STEP_MS;
			}
		}
	}

	return {
		sync,
		frame,
		destroy() {
			authoritative = null;
			shownLength = 0;
			animatingUntil = 0;
			scheduled = false;
		}
	};
}

/** A copy of s whose trail is cut to len nodes. */
function withPathLength(s: Session, len: number): Session {
	if (len >= s.state.path.length) return s;
	return {
		...s,
		state: { ...s.state, path: s.state.path.slice(0, len), pos: s.state.path[len - 1] ?? s.state.pos }
	};
}

/** True when the user asked the OS to reduce motion. */
export function prefersReducedMotion(): boolean {
	if (typeof matchMedia !== 'function') return false;
	return matchMedia('(prefers-reduced-motion: reduce)').matches;
}
