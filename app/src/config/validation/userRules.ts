import type {
    EmailRules,
    PasswordRules,
    RuleSet,
    UsernameRules,
} from '../../Components/UserRules/model/userRules';

// Pure client-side validators. The backend is the source of truth and
// re-validates on every mutating request — these helpers exist to give
// instant feedback in forms. Return ordered, human-readable messages.

export function validatePassword(
    rules: PasswordRules,
    password: string,
    context: { username?: string; email?: string } = {},
): string[] {
    const out: string[] = [];
    if (rules.min_length > 0 && password.length < rules.min_length) {
        out.push(`Password must be at least ${rules.min_length} characters.`);
    }
    if (rules.max_length > 0 && password.length > rules.max_length) {
        out.push(`Password must be at most ${rules.max_length} characters.`);
    }
    if (rules.require_upper && !/\p{Lu}/u.test(password)) {
        out.push('Password must contain at least one uppercase letter.');
    }
    if (rules.require_lower && !/\p{Ll}/u.test(password)) {
        out.push('Password must contain at least one lowercase letter.');
    }
    if (rules.require_digit && !/\p{N}/u.test(password)) {
        out.push('Password must contain at least one digit.');
    }
    if (rules.require_special && !/[^\p{L}\p{N}\s]/u.test(password)) {
        out.push('Password must contain at least one special character.');
    }
    if (rules.disallow_username && context.username) {
        if (password.toLowerCase().includes(context.username.toLowerCase())) {
            out.push('Password must not contain the username.');
        }
    }
    if (rules.disallow_email && context.email) {
        const local = context.email.split('@')[0] ?? '';
        if (local && password.toLowerCase().includes(local.toLowerCase())) {
            out.push('Password must not contain the email address.');
        }
    }
    return out;
}

export function validateUsername(rules: UsernameRules, username: string): string[] {
    const out: string[] = [];
    const u = username.trim();
    if (rules.min_length > 0 && u.length < rules.min_length) {
        out.push(`Username must be at least ${rules.min_length} characters.`);
    }
    if (rules.max_length > 0 && u.length > rules.max_length) {
        out.push(`Username must be at most ${rules.max_length} characters.`);
    }
    if (rules.regex) {
        try {
            const re = new RegExp(rules.regex);
            if (!re.test(u)) {
                out.push('Username contains characters that are not allowed.');
            }
        } catch {
            // Invalid configured regex — silently ignore so we don't
            // lock users out over a broken admin setting.
        }
    }
    for (const r of rules.reserved ?? []) {
        if (u.toLowerCase() === r.toLowerCase()) {
            out.push(`Username "${u}" is reserved.`);
            break;
        }
    }
    return out;
}

export function validateEmail(rules: EmailRules, email: string): string[] {
    const out: string[] = [];
    const at = email.indexOf('@');
    if (at < 0 || at === email.length - 1) {
        out.push('Email address is not valid.');
        return out;
    }
    const domain = email.slice(at + 1).toLowerCase();
    for (const b of rules.blocked_domains ?? []) {
        if (domain === b.toLowerCase()) {
            out.push(`Email domain "${domain}" is not permitted.`);
            return out;
        }
    }
    if ((rules.allowed_domains ?? []).length > 0) {
        const allowed = rules.allowed_domains!.some(a => domain === a.toLowerCase());
        if (!allowed) {
            out.push(`Email domain "${domain}" is not permitted.`);
        }
    }
    return out;
}

export function validateAll(
    rules: RuleSet,
    input: { username?: string; email?: string; password?: string },
): string[] {
    const out: string[] = [];
    if (input.username) out.push(...validateUsername(rules.username, input.username));
    if (input.email) out.push(...validateEmail(rules.email, input.email));
    if (input.password) out.push(...validatePassword(rules.password, input.password, {
        username: input.username,
        email: input.email,
    }));
    return out;
}
