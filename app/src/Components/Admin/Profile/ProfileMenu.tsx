import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { DEFAULT_PROFILE_MENU, type ProfileMenuEntry } from './profileMenuEntries';

interface Props {
    items?: ProfileMenuEntry[];
}

// Exact-match active detection only (no prefix matching, unlike
// SettingsMenu) — "/profile" is itself a prefix of "/profile/security",
// so prefix matching would light up "General" while on "Security" too.
const ProfileMenu: React.FC<Props> = ({ items = DEFAULT_PROFILE_MENU }) => {
    const location = useLocation();
    return (
        <nav className="space-y-1" aria-label="Profile">
            {items.map(entry => {
                const active = location.pathname === entry.href;
                return (
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
            })}
        </nav>
    );
};

export default ProfileMenu;
