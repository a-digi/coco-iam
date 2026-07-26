import { AppScopes } from '../../../config/security/scopes';

export interface SecurityMenuEntry {
    label: string;
    href: string;
    /**
     * Paths that count as "active" when rendered (prefix-matched). Defaults to
     * [href] if omitted.
     */
    matchPrefixes?: string[];
    scopes?: string[];
}

export const DEFAULT_SECURITY_MENU: SecurityMenuEntry[] = [
    {
        label: 'Bans',
        href: '/admin/security/bans',
        matchPrefixes: ['/admin/security/bans'],
        scopes: [AppScopes.AdminSecurityIpBansRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Allowlist',
        href: '/admin/security/allowlist',
        matchPrefixes: ['/admin/security/allowlist'],
        scopes: [AppScopes.AdminSecurityIpAllowlistRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Attacks',
        href: '/admin/security/attacks',
        matchPrefixes: ['/admin/security/attacks'],
        scopes: [AppScopes.AdminSecurityAttacksRead, AppScopes.SuperAdmin],
    },
];
