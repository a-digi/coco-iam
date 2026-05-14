import { AppScopes } from '../../../config/security/scopes';

export interface SettingsMenuEntry {
    label: string;
    href: string;
    /**
     * Paths that count as "active" when rendered (prefix-matched). Defaults to
     * [href] if omitted.
     */
    matchPrefixes?: string[];
    scopes?: string[];
}

export const DEFAULT_SETTINGS_MENU: SettingsMenuEntry[] = [
    {
        label: 'General',
        href: '/admin/settings/general',
        matchPrefixes: ['/admin/settings/general'],
        scopes: [AppScopes.AdminSettingsGeneralRead, AppScopes.AdminSettingsGeneral, AppScopes.SuperAdmin],
    },
    {
        label: 'User rules',
        href: '/admin/settings/user-rules',
        matchPrefixes: ['/admin/settings/user-rules'],
        scopes: [AppScopes.AdminSettingsUserRulesRead, AppScopes.AdminSettingsUserRules, AppScopes.SuperAdmin],
    },
    {
        label: 'Email',
        href: '/admin/settings/email',
        // Explicit prefix without trailing segment-continuation so sibling
        // paths like /email-accounts don't light this entry up.
        matchPrefixes: ['/admin/settings/email'],
        scopes: [AppScopes.AdminMailSettingsRead, AppScopes.AdminMailSettings, AppScopes.SuperAdmin],
    },
    {
        label: 'Email accounts',
        href: '/admin/settings/email-accounts',
        matchPrefixes: ['/admin/settings/email-accounts'],
        scopes: [AppScopes.AdminMailSettingsRead, AppScopes.AdminMailSettings, AppScopes.SuperAdmin],
    },
    {
        label: 'Email templates',
        href: '/admin/settings/email-templates',
        matchPrefixes: ['/admin/settings/email-templates'],
        scopes: [AppScopes.AdminMailTemplatesRead, AppScopes.AdminMailTemplates, AppScopes.SuperAdmin],
    },
];
