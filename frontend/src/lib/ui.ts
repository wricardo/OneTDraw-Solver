// Small shared UI helpers.

/** Joins class names, dropping falsy values. */
export function cx(...parts: (string | false | null | undefined)[]): string {
	return parts.filter(Boolean).join(' ');
}

/** Formats a timestamp as a short relative time ("3m ago"). */
export function timeAgo(iso: string, now = Date.now()): string {
	const t = Date.parse(iso);
	if (Number.isNaN(t)) return '';
	const secs = Math.max(0, Math.round((now - t) / 1000));
	if (secs < 60) return `${secs}s ago`;
	const mins = Math.round(secs / 60);
	if (mins < 60) return `${mins}m ago`;
	const hours = Math.round(mins / 60);
	if (hours < 24) return `${hours}h ago`;
	const days = Math.round(hours / 24);
	return `${days}d ago`;
}

/** Pluralises a count with a unit, e.g. plural(1, 'edge') -> '1 edge'. */
export function plural(n: number, unit: string, suffix = 's'): string {
	return `${n} ${unit}${n === 1 ? '' : suffix}`;
}
