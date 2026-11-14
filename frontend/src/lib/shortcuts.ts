// Keyboard shortcuts for the play page.
//
// Only four things are bound: undo, reset, solve, and escape to dismiss. Node
// picking is by click, because a graph has no natural arrow-key geometry.

export type Action =
	| { type: 'undo' }
	| { type: 'reset' }
	| { type: 'solve' }
	| { type: 'dismiss' }
	| { type: 'help' };

/** Maps a keydown event to an action, or null when it is not a shortcut. */
export function keyToAction(e: KeyboardEvent): Action | null {
	if (e.metaKey || e.ctrlKey || e.altKey) return null;
	switch (e.key.toLowerCase()) {
		case 'z':
		case 'backspace':
			return { type: 'undo' };
		case 'r':
			return { type: 'reset' };
		case 's':
			return { type: 'solve' };
		case '?':
			return { type: 'help' };
		case 'escape':
			return { type: 'dismiss' };
		default:
			return null;
	}
}

/** True when the event targets a text field, where shortcuts must not fire. */
export function isTypingTarget(target: EventTarget | null): boolean {
	const el = target as HTMLElement | null;
	if (!el) return false;
	const tag = el.tagName?.toLowerCase();
	return tag === 'input' || tag === 'textarea' || tag === 'select' || el.isContentEditable === true;
}
