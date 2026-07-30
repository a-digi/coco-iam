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
    {
        label: 'Archives',
        href: '/admin/security/archives',
        matchPrefixes: ['/admin/security/archives'],
        scopes: [AppScopes.AdminSecurityArchivesRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Port scans',
        href: '/admin/security/scans',
        matchPrefixes: ['/admin/security/scans'],
        scopes: [AppScopes.AdminSecurityScansRead, AppScopes.SuperAdmin],
    },
    {
        label: 'GeoIP',
        href: '/admin/security/geoip',
        matchPrefixes: ['/admin/security/geoip'],
        scopes: [AppScopes.AdminSecurityGeoipRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Login log',
        href: '/admin/security/login-log',
        matchPrefixes: ['/admin/security/login-log'],
        scopes: [AppScopes.AdminSecurityLoginLogRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Login ban rules',
        href: '/admin/security/login-bans',
        matchPrefixes: ['/admin/security/login-bans'],
        scopes: [AppScopes.AdminSecurityLoginBansRead, AppScopes.SuperAdmin],
    },
    {
        label: 'Firewall',
        href: '/admin/security/firewall',
        matchPrefixes: ['/admin/security/firewall'],
        scopes: [AppScopes.AdminSecurityFirewallRead, AppScopes.SuperAdmin],
    },
];
