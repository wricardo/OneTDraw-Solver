import { createClient, cacheExchange, fetchExchange, subscriptionExchange, type Client } from '@urql/svelte';
import { createClient as createWsClient, type Client as WsClient } from 'graphql-ws';
import { getAdminKey } from './admin';

const GRAPHQL_URL =
	typeof window !== 'undefined' && import.meta.env?.PUBLIC_GRAPHQL_URL
		? import.meta.env.PUBLIC_GRAPHQL_URL
		: '/graphql';

const WS_URL =
	typeof window !== 'undefined' && import.meta.env?.PUBLIC_WS_URL
		? import.meta.env.PUBLIC_WS_URL
		: typeof window !== 'undefined'
			? `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/graphql`
			: 'ws://localhost:8090/graphql';

let wsClient: WsClient | null = null;

/** Returns the shared graphql-ws client, creating it lazily. */
export function getWsClient(): WsClient {
	if (!wsClient) {
		wsClient = createWsClient({ url: WS_URL });
	}
	return wsClient;
}

/** Creates the urql client. Admin mutations carry X-Admin-Key read from sessionStorage per request. */
export function makeClient(): Client {
	return createClient({
		url: GRAPHQL_URL,
		// Mutations/queries must always reflect server state; a document cache would hide moves.
		requestPolicy: 'network-only',
		// The Go server only accepts POST; urql defaults to GET for short queries.
		preferGetMethod: false,
		fetchOptions: () => {
			const key = getAdminKey();
			return key ? { headers: { 'X-Admin-Key': key } } : {};
		},
		exchanges: [
			cacheExchange,
			fetchExchange,
			subscriptionExchange({
				forwardSubscription(request) {
					const input = { ...request, query: request.query ?? '' };
					return {
						subscribe(sink) {
							const unsubscribe = getWsClient().subscribe(input, sink);
							return { unsubscribe };
						}
					};
				}
			})
		]
	});
}
