// Admin key storage. It is a local development convenience: the key lives in
// sessionStorage for the tab's lifetime and is sent as X-Admin-Key on every
// GraphQL request, so admin mutations work without a separate login flow.
const KEY = 'one-t-draw-admin-key';

/** Returns the stored admin key, or '' when none is set. */
export function getAdminKey(): string {
	if (typeof sessionStorage === 'undefined') return '';
	try {
		return sessionStorage.getItem(KEY) ?? '';
	} catch {
		return '';
	}
}

/** Stores (or, with '', clears) the admin key. */
export function setAdminKey(key: string): void {
	if (typeof sessionStorage === 'undefined') return;
	try {
		if (key) sessionStorage.setItem(KEY, key);
		else sessionStorage.removeItem(KEY);
	} catch {
		// Storage can be unavailable (private mode, quota). The key just won't
		// persist, which the editor reports when an admin call is refused.
	}
}
