import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { DEFAULT_SETTINGS_MENU, type SettingsMenuEntry } from './settingsMenuEntries';

interface Props {
    items?: SettingsMenuEntry[];
}

const SettingsMenu: React.FC<Props> = ({ items = DEFAULT_SETTINGS_MENU }) => {
    const location = useLocation();
    return (
        <nav className="space-y-1" aria-label="Settings">
            {items.map(entry => {
                const prefixes = entry.matchPrefixes ?? [entry.href];
                // Segment-boundary match: "/admin/settings/email" must not be
                // considered active when the path is "/admin/settings/email-templates".
                const active = prefixes.some(p => location.pathname === p || location.pathname.startsWith(p + '/'));
                const node = (
                    <Link
                        key={entry.href}
                        to={entry.href}
                        className={[
                            'block px-3 py-2 rounded-md text-sm transition-colors',
                            active
                                ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200 font-medium'
                                : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800',
                        ].join(' ')}
                    >
                        {entry.label}
                    </Link>
                );
                if (entry.scopes && entry.scopes.length > 0) {
                    return (
                        <ScopeBasedComponentAccess key={entry.href} requiredScopes={entry.scopes}>
                            {node}
                        </ScopeBasedComponentAccess>
                    );
                }
                return node;
            })}
        </nav>
    );
};

export default SettingsMenu;
