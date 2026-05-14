import React from 'react';
import Title from '../../../Shared/Components/Font/Title';
import SettingsMenu from './SettingsMenu';

interface Props {
    children: React.ReactNode;
}

/**
 * Admin Settings shell. Renders a left-side vertical menu and the supplied
 * children on the right. Kept framework-agnostic (no nested-route Outlet)
 * so routes can remain flat and each leaf route wraps itself in this shell.
 */
const SettingsPage: React.FC<Props> = ({ children }) => {
    return (
        <div>
            <Title>Admin Settings</Title>
            <div className="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-6 mt-6">
                <aside className="md:border-r md:border-gray-200 dark:md:border-surface-800 md:pr-4">
                    <SettingsMenu />
                </aside>
                <main className="min-w-0">
                    {children}
                </main>
            </div>
        </div>
    );
};

export default SettingsPage;
