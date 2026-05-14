// isSafeRedirectPath decides whether a server-returned URL is safe to
// use as a client-side navigation target without rewriting the origin.
//
// We accept only **same-origin absolute paths**: strings that start
// with "/" but NOT "//" (protocol-relative URLs bypass same-origin
// naively). Any other shape — full URLs, relative paths, empty
// strings, javascript: schemes — is rejected.
//
// This guard sits at the boundary between the activate response and
// the `<Link to={...}>` href; a tampered backend response (or a
// future bug that returns an absolute URL) must not be able to bounce
// a freshly-activated user to an attacker-controlled host.
export function isSafeRedirectPath(input: string): boolean {
    if (typeof input !== 'string') return false;
    if (input.length < 2) return false;
    if (!input.startsWith('/')) return false;
    // "//evil.com/..." → protocol-relative URL. Reject.
    if (input.startsWith('//')) return false;
    // "/\\evil.com" — some browsers normalise backslashes into slashes.
    if (input.startsWith('/\\')) return false;
    return true;
}
