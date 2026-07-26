import React from 'react';
import Title from '../../../Shared/Components/Font/Title';
import SecurityMenu from './SecurityMenu';
import FirewallStatusBanner from './FirewallStatusBanner';

interface Props {
    children: React.ReactNode;
}

/**
 * Admin Security shell. Renders the firewall status banner, a left-side
 * vertical menu (Bans / Allowlist / Attacks), and the supplied children on
 * the right — modeled on SettingsPage.tsx. No nested-route Outlet: each leaf
 * route wraps itself in this shell.
 */
const SecurityPage: React.FC<Props> = ({ children }) => {
    return (
        <div>
            <Title>Security</Title>
            <FirewallStatusBanner />
            <div className="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-6 mt-6">
                <aside className="md:border-r md:border-gray-200 dark:md:border-surface-800 md:pr-4">
                    <SecurityMenu />
                </aside>
                <main className="min-w-0">
                    {children}
                </main>
            </div>
        </div>
    );
};

export default SecurityPage;
