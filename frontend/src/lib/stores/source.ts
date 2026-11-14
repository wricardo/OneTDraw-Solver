// The live sources a session store can read from: GraphQL query + websocket.

import type { Client } from '@urql/svelte';
import { unwrap } from '../errors';
import { SESSION_QUERY, SESSION_UPDATED_SUBSCRIPTION } from '../queries';
import type { ConnectionState, Session, SessionUpdate } from '../types';
import type { SessionSource } from './session';

/** A source backed by the real GraphQL API and its websocket. */
export function createSessionSource(client: Client, sessionID: string): SessionSource {
	return {
		async load(): Promise<Session> {
			const data = await unwrap<{ session: Session | null }>(client.query(SESSION_QUERY, { id: sessionID }));
			if (!data.session) throw new Error(`session ${sessionID} was not found`);
			return data.session;
		},
		subscribe(onUpdate, onState) {
			onState('connecting');
			const sub = client
				.subscription<{ sessionUpdated: SessionUpdate }>(SESSION_UPDATED_SUBSCRIPTION, { sessionID })
				.subscribe((result) => {
					if (result.error) {
						// urql's websocket exchange reports a closed socket as an error on
						// the subscription; treat it as a reconnect and let the store reload.
						onState('reconnecting');
						return;
					}
					if (result.data?.sessionUpdated) {
						onState('live');
						onUpdate(result.data.sessionUpdated);
					}
				});
			onState('live');
			return () => sub.unsubscribe();
		}
	};
}

export type { ConnectionState, SessionUpdate };
