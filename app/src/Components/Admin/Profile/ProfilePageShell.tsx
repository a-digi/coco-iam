import React from 'react';
import Title from '../../../Shared/Components/Font/Title';
import ProfileMenu from './ProfileMenu';

interface Props {
    children: React.ReactNode;
}

/**
 * Profile shell. Renders a left-side vertical menu (General / Security)
 * and the supplied children on the right — modeled on SettingsPage.tsx.
 * No nested-route Outlet: each leaf route wraps itself in this shell.
 */
const ProfilePageShell: React.FC<Props> = ({ children }) => {
    return (
        <div>
            <Title>Profile</Title>
            <div className="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-6 mt-6">
                <aside className="md:border-r md:border-gray-200 dark:md:border-surface-800 md:pr-4">
                    <ProfileMenu />
                </aside>
                <main className="min-w-0">
                    {children}
                </main>
            </div>
        </div>
    );
};

export default ProfilePageShell;
