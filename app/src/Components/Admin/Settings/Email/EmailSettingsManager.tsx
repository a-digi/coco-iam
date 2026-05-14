import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import EventBindingsSection from './sections/EventBindingsSection';
import ActivationSection from './sections/ActivationSection';
import type { EmailAccount } from '../EmailAccounts/model/emailAccount';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import {
    type EmailSettings,
    type EventBinding,
    type ActivationSettings,
    EMPTY_ACTIVATION,
} from './model/emailSettings';

export interface EmailSettingsManagerProps {
    className?: string;
}

/**
 * EmailSettingsManager owns the Admin Settings → Email screen. SMTP
 * connection details now live under Email accounts; this page summarises
 * which account is active and manages the event→template bindings.
 */
export const EmailSettingsManager: React.FC<EmailSettingsManagerProps> = ({
    className = '',
}) => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Settings' }, { label: 'Email' }]);
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [activeAccount, setActiveAccount] = useState<EmailAccount | null>(null);
    const [events, setEvents] = useState<EventBinding[]>([]);
    const [activation, setActivation] = useState<ActivationSettings>(EMPTY_ACTIVATION);
    const [loading, setLoading] = useState(true);

    const fetchSettings = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: EmailSettings }>('admin/mail/settings');
            const data = response?.message;
            if (data) {
                setActiveAccount(data.active_account ?? null);
                setEvents(Array.isArray(data.events) ? data.events : []);
                setActivation(data.activation ?? EMPTY_ACTIVATION);
            }
        } catch (err: unknown) {
            let msg = 'Failed to load email settings';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void fetchSettings();
    }, [fetchSettings]);

    if (loading) {
        return <div className={className}><div className="text-sm text-gray-500 py-2">Loading email settings…</div></div>;
    }

    return (
        <div className={`space-y-10 ${className}`}>
            <ActiveAccountBanner account={activeAccount} />
            <ActivationSection initial={activation} onSaved={setActivation} />
            <div className="border-t border-gray-200 dark:border-surface-800" />
            <EventBindingsSection initial={events} onSaved={setEvents} />
        </div>
    );
};

const ActiveAccountBanner: React.FC<{ account: EmailAccount | null }> = ({ account }) => {
    if (!account) {
        return (
            <div className="p-4 border border-amber-200 dark:border-amber-700/50 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
                <div className="text-sm font-medium text-amber-900 dark:text-amber-200">No active SMTP account</div>
                <p className="text-xs text-amber-800 dark:text-amber-300 mt-1">
                    Outbound mail will fall back to environment variables.{' '}
                    <Link to="/admin/settings/email-accounts" className="underline">Create an account</Link>{' '}
                    to manage SMTP settings from the UI.
                </p>
            </div>
        );
    }
    return (
        <div className="p-4 border border-indigo-200 dark:border-indigo-800 bg-indigo-50 dark:bg-indigo-900/20 rounded-lg">
            <div className="flex items-center justify-between flex-wrap gap-2">
                <div>
                    <div className="text-sm font-medium text-indigo-900 dark:text-indigo-200">
                        Active SMTP account: <span className="font-mono">{account.name}</span>
                    </div>
                    <div className="text-xs text-indigo-800 dark:text-indigo-300 mt-1">
                        {account.host}:{account.port} · From: {account.from_email || '(unset)'} {account.use_tls ? '· TLS' : ''}
                    </div>
                </div>
                <Link
                    to="/admin/settings/email-accounts"
                    className="text-xs text-indigo-700 dark:text-indigo-300 hover:underline"
                >
                    Manage accounts →
                </Link>
            </div>
        </div>
    );
};

export default EmailSettingsManager;
