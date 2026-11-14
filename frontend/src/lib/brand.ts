/** Site brand: the Dr Brain collection of reimplemented games. */
export const SITE = 'Dr Brain';
/** This game within the collection. */
export const GAME = 'One T Draw';
/** Full product name used in titles, prompts and docs. */
export const TITLE = `${SITE} - ${GAME}`;

/** Document title for a page: "<page> — Dr Brain - One T Draw". */
export function pageTitle(page?: string): string {
	return page ? `${page} — ${TITLE}` : TITLE;
}
