// Starting a game: creating a session and handing the browser its id.
import type { Client } from '@urql/svelte';
import { unwrap } from './errors';
import { CREATE_SESSION_MUTATION } from './queries';
import type { Session } from './types';

/** Creates a session on mapID and returns its id. */
export async function startSession(client: Client, mapID: string, displayName?: string): Promise<string> {
	const data = await unwrap<{ createSession: Session }>(
		client.mutation(CREATE_SESSION_MUTATION, { mapID, displayName: displayName || null })
	);
	return data.createSession.id;
}
