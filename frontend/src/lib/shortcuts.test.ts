import { describe, expect, it } from 'vitest';
import { isTypingTarget, keyToAction } from './shortcuts';

function key(k: string, mods: Partial<KeyboardEvent> = {}): KeyboardEvent {
	return { key: k, metaKey: false, ctrlKey: false, altKey: false, ...mods } as KeyboardEvent;
}

describe('shortcuts', () => {
	it('maps undo to z and backspace', () => {
		expect(keyToAction(key('z'))).toEqual({ type: 'undo' });
		expect(keyToAction(key('Backspace'))).toEqual({ type: 'undo' });
	});

	it('maps reset, solve, help and dismiss', () => {
		expect(keyToAction(key('r'))).toEqual({ type: 'reset' });
		expect(keyToAction(key('s'))).toEqual({ type: 'solve' });
		expect(keyToAction(key('?'))).toEqual({ type: 'help' });
		expect(keyToAction(key('Escape'))).toEqual({ type: 'dismiss' });
	});

	it('ignores modified keys so browser shortcuts still work', () => {
		expect(keyToAction(key('z', { metaKey: true }))).toBeNull();
		expect(keyToAction(key('r', { ctrlKey: true }))).toBeNull();
	});

	it('ignores unrelated keys', () => {
		expect(keyToAction(key('q'))).toBeNull();
	});
});

describe('typing detection', () => {
	it('recognises form fields', () => {
		const make = (tag: string) => ({ tagName: tag, isContentEditable: false }) as unknown as HTMLElement;
		expect(isTypingTarget(make('INPUT'))).toBe(true);
		expect(isTypingTarget(make('TEXTAREA'))).toBe(true);
		expect(isTypingTarget(make('SELECT'))).toBe(true);
		expect(isTypingTarget(make('DIV'))).toBe(false);
	});

	it('recognises contenteditable', () => {
		const el = { tagName: 'DIV', isContentEditable: true } as unknown as HTMLElement;
		expect(isTypingTarget(el)).toBe(true);
	});

	it('handles a null target', () => {
		expect(isTypingTarget(null)).toBe(false);
	});
});
