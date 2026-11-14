// Session state for one page: an authoritative snapshot plus a live subscription.
//
// The page renders from a store rather than directly from a query result so a
// websocket update and a mutation reply take exactly the same path. Updates are
// merged by `seq`: anything not newer than what we already have is dropped, so
// an out-of-order delivery or a slow mutation reply cannot roll the board back.

import type { Readable } from 'svelte/store';
import type { ConnectionState, Session, SessionUpdate } from '../types';

/** Merges incoming into current, keeping the newer by seq. */
export function mergeSession(current: Session | null, incoming: Session): Session {
	if (!current) return incoming;
	if (incoming.seq < current.seq) return current;
	return incoming;
}

/** Applies one subscription update to a session. */
export function applyUpdate(current: Session | null, update: SessionUpdate): Session {
	return mergeSession(current, update.session);
}

export interface SessionState {
	session: Session | null;
	connection: ConnectionState;
	loading: boolean;
	error: { title: string; message: string } | null;
}

export interface SessionStore extends Readable<SessionState> {
	/** Applies an authoritative snapshot from a mutation or query. */
	apply(session: Session): void;
	/** Replaces the session wholesale (e.g. after a reset the seq can go back). */
	replace(session: Session): void;
	/** Marks the store as failed to load. */
	fail(title: string, message: string): void;
	/** Stops the subscription. */
	destroy(): void;
}

export interface SessionSource {
	/** Fetches the current snapshot once. */
	load(): Promise<Session>;
	/** Subscribes to updates; returns an unsubscribe function. */
	subscribe(onUpdate: (u: SessionUpdate) => void, onState: (c: ConnectionState) => void): () => void;
}

/**
 * Creates the store for one session. It loads the snapshot, then subscribes; on
 * reconnect it re-subscribes and re-loads, because updates missed while the
 * socket was down are not replayed.
 */
export function createSessionStore(source: SessionSource, sessionID: string, autoStart = true): SessionStore {
	let state: SessionState = { session: null, connection: 'connecting', loading: true, error: null };
	const listeners = new Set<(s: SessionState) => void>();

	function set(patch: Partial<SessionState>): void {
		state = { ...state, ...patch };
		for (const l of listeners) l(state);
	}

	let unsubscribe: (() => void) | null = null;
	let destroyed = false;

	function connect(): void {
		if (destroyed) return;
		set({ connection: 'connecting' });
		unsubscribe = source.subscribe(
			(u) => {
				if (destroyed) return;
				set({ session: applyUpdate(state.session, u), connection: 'live', loading: false, error: null });
			},
			(c) => {
				if (destroyed) return;
				set({ connection: c });
				if (c === 'reconnecting' || c === 'live') {
					// A reconnect may have missed updates: refresh the truth.
					void reload();
				}
			}
		);
	}

	async function reload(): Promise<void> {
		try {
			const s = await source.load();
			if (destroyed) return;
			set({ session: mergeSession(state.session, s), loading: false, error: null });
		} catch (e) {
			if (destroyed) return;
			const err = e as { message?: string };
			set({ loading: false, error: { title: 'Could not load the game', message: err?.message ?? String(e) } });
		}
	}

	if (autoStart) {
		void reload();
		connect();
	}

	return {
		subscribe(run) {
			run(state);
			listeners.add(run);
			return () => {
				listeners.delete(run);
			};
		},
		apply(session) {
			set({ session: mergeSession(state.session, session), loading: false });
		},
		replace(session) {
			set({ session, loading: false, error: null });
		},
		fail(title, message) {
			set({ loading: false, error: { title, message } });
		},
		destroy() {
			destroyed = true;
			unsubscribe?.();
			unsubscribe = null;
			listeners.clear();
		}
	};
}
