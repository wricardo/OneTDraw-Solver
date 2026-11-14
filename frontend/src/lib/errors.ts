// Unwrapping urql results and turning failures into a uniform, user-facing shape.
//
// urql reports transport problems through `error` and server-side GraphQL
// problems through `error.graphQLErrors`; both have to end up in the same place
// so a page can show one message. This module is the only place that knows the
// difference.

/** A description of a failed call, ready to render. */
export interface DescribedError {
	/** Short heading, e.g. "Trail is already won". */
	title: string;
	/** Longer explanation; empty when the title says it all. */
	message: string;
	/** Server error code from extensions.code, when present. */
	code?: string;
}

/** Error codes the API uses, matching game/service and the schema docs. */
const CODE_TITLES: Record<string, string> = {
	NOT_FOUND: 'Not found',
	SESSION_TERMINAL: 'Game already finished',
	INVALID_ARGUMENT: 'That is not allowed',
	FORBIDDEN: 'Not allowed',
	LIMIT_REACHED: 'Too many games',
	INTERNAL: 'Server error',
	GRAPHQL_VALIDATION_FAILED: 'Invalid request'
};

interface GraphQLErrorLike {
	message?: string;
	extensions?: { code?: string } | null;
}

interface CombinedErrorLike {
	message?: string;
	graphQLErrors?: GraphQLErrorLike[];
	networkError?: Error | null;
}

/** Describes any thrown value as something a page can show. */
export function describeError(err: unknown): DescribedError {
	const e = err as CombinedErrorLike | undefined;
	const gqlErrors = e?.graphQLErrors ?? [];
	if (gqlErrors.length > 0) {
		const first = gqlErrors[0];
		const code = first.extensions?.code;
		const title = (code && CODE_TITLES[code]) ?? 'Request failed';
		return { title, message: first.message ?? '', code: code ?? undefined };
	}
	if (e?.networkError) {
		return { title: 'Cannot reach the server', message: e.networkError.message || 'The API did not respond.' };
	}
	if (e?.message) {
		return { title: 'Request failed', message: e.message };
	}
	return { title: 'Request failed', message: String(err ?? 'unknown error') };
}

/** The subset of an urql operation this module needs. */
interface OperationSourceLike<T> {
	toPromise(): Promise<{ data?: T | null; error?: CombinedErrorLike | null }>;
}

/**
 * Awaits an urql operation and resolves its data, rejecting with the combined
 * error so describeError can shape it.
 *
 * urql's query()/mutation() return a *source*, not a promise: it must be awaited
 * through toPromise(), and skipping that is a common source of silent
 * no-ops, so every call site goes through here.
 */
export async function unwrap<T>(op: OperationSourceLike<T>): Promise<T> {
	const res = await op.toPromise();
	if (res.error) throw res.error;
	if (res.data == null) throw new Error('the server returned no data');
	return res.data;
}

/** Human sentence for a loss reason. */
export function lossReasonText(reason: string | null): string {
	if (reason === 'DEAD_END') return 'You got stuck with edges still to draw.';
	return 'The trail ended early.';
}
