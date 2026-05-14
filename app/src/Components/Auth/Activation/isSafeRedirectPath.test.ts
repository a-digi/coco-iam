import { describe, it, expect } from 'vitest';
import { isSafeRedirectPath } from './isSafeRedirectPath';

describe('isSafeRedirectPath', () => {
    it('accepts a plain same-origin absolute path', () => {
        expect(isSafeRedirectPath('/login')).toBe(true);
    });

    it('accepts a nested same-origin path', () => {
        expect(isSafeRedirectPath('/login/a/org/ws/client')).toBe(true);
    });

    it('accepts paths with query strings and fragments', () => {
        expect(isSafeRedirectPath('/login/a/org/ws/client?foo=bar#section')).toBe(true);
    });

    it('rejects an empty string', () => {
        expect(isSafeRedirectPath('')).toBe(false);
    });

    it('rejects a single slash (no path to navigate to)', () => {
        // "/" alone is technically same-origin but not a meaningful
        // redirect target and the helper treats it as invalid. Pins
        // that intent so a future edit doesn't silently start
        // bouncing activations to the dashboard root.
        expect(isSafeRedirectPath('/')).toBe(false);
    });

    it('rejects a protocol-relative URL', () => {
        // "//evil.com" is interpreted as a full URL by the browser —
        // the scheme is inherited from the current page. The guard
        // must reject it.
        expect(isSafeRedirectPath('//evil.com/login')).toBe(false);
    });

    it('rejects a backslash-prefixed path (browser normalises to /)', () => {
        expect(isSafeRedirectPath('/\\evil.com')).toBe(false);
    });

    it('rejects a fully qualified URL', () => {
        expect(isSafeRedirectPath('https://evil.com/login')).toBe(false);
    });

    it('rejects a javascript: URL', () => {
        expect(isSafeRedirectPath('javascript:alert(1)')).toBe(false);
    });

    it('rejects a relative path without a leading slash', () => {
        expect(isSafeRedirectPath('login/a/org/ws/app')).toBe(false);
    });

    it('rejects non-string inputs defensively', () => {
        // The backend could in principle return a non-string; the
        // public interface is typed as string but a malformed JSON
        // response shouldn't crash the activate flow.
        // @ts-expect-error verifying the runtime guard for undefined
        expect(isSafeRedirectPath(undefined)).toBe(false);
        // @ts-expect-error verifying the runtime guard for null
        expect(isSafeRedirectPath(null)).toBe(false);
        // @ts-expect-error verifying the runtime guard for number
        expect(isSafeRedirectPath(42)).toBe(false);
    });
});
