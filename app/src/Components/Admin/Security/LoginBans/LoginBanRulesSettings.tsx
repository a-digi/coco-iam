import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Submit } from '../../../../Shared/Components/Button';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { DomainRuleFields } from './DomainRuleFields';
import type { LoginBanRulesResponse } from './types';

// LoginBanRulesSettings is the admin UI for configuring the two
// failed-login IP ban rules (admin console, application end-users).
// Both domain blocks are always read/written together as one settings
// object — see plan/login-ban-rules/plan.md. Mirrors GeoIP's own
// SettingsTab.tsx: local form state as strings, snackbar feedback,
// re-read-after-save.
export const LoginBanRulesSettings: React.FC = () => {
    const { get, put } = useHttpClient();
    const { errorMessage, successMessage } = useSnackBar();

    const [adminEnabled, setAdminEnabled] = useState(false);
    const [adminThreshold, setAdminThreshold] = useState('5');
    const [adminWindowSeconds, setAdminWindowSeconds] = useState('600');
    const [adminBanSeconds, setAdminBanSeconds] = useState('3600');

    const [applicationEnabled, setApplicationEnabled] = useState(false);
    const [applicationThreshold, setApplicationThreshold] = useState('5');
    const [applicationWindowSeconds, setApplicationWindowSeconds] = useState('600');
    const [applicationBanSeconds, setApplicationBanSeconds] = useState('3600');

    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const applySettings = (s: LoginBanRulesResponse) => {
        setAdminEnabled(s.admin.enabled);
        setAdminThreshold(String(s.admin.threshold));
        setAdminWindowSeconds(String(s.admin.window_seconds));
        setAdminBanSeconds(String(s.admin.ban_seconds));

        setApplicationEnabled(s.application.enabled);
        setApplicationThreshold(String(s.application.threshold));
        setApplicationWindowSeconds(String(s.application.window_seconds));
        setApplicationBanSeconds(String(s.application.ban_seconds));
    };

    const loadSettings = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: LoginBanRulesResponse }>('admin/security/login-bans/settings');
            applySettings(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load login ban rule settings.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadSettings();
    }, [loadSettings]);

    const adminValid = !adminEnabled || (
        Number(adminThreshold) >= 1 && Number(adminWindowSeconds) > 0 && Number(adminBanSeconds) > 0
    );
    const applicationValid = !applicationEnabled || (
        Number(applicationThreshold) >= 1 && Number(applicationWindowSeconds) > 0 && Number(applicationBanSeconds) > 0
    );
    const canSubmit = adminValid && applicationValid;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setSaving(true);
        try {
            const resp = await put<{ message: LoginBanRulesResponse }>('admin/security/login-bans/settings', {
                admin: {
                    enabled: adminEnabled,
                    threshold: Number(adminThreshold),
                    window_seconds: Number(adminWindowSeconds),
                    ban_seconds: Number(adminBanSeconds),
                },
                application: {
                    enabled: applicationEnabled,
                    threshold: Number(applicationThreshold),
                    window_seconds: Number(applicationWindowSeconds),
                    ban_seconds: Number(applicationBanSeconds),
                },
            });
            applySettings(resp.message);
            successMessage('Login ban rule settings saved.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save login ban rule settings.');
        } finally {
            setSaving(false);
        }
    };

    if (loading) {
        return <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>;
    }

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-2">Login ban rules</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    Automatically bans an IP once it crosses a configured number of failed login
                    attempts within a time window — independent for the admin console and
                    application end-user logins. Bans go through the same mechanism as every other
                    ban in this system, so the IP allowlist and the Bans page apply automatically.
                </p>
            </div>

            <p className="text-xs text-amber-700 dark:text-amber-400">
                A low threshold can ban an admin's own IP after a handful of mistyped passwords. Add
                trusted IPs to the{' '}
                <Link to="/admin/security/allowlist" className="underline">
                    IP allowlist
                </Link>{' '}
                before tightening these rules.
            </p>

            <form onSubmit={handleSubmit} className="space-y-8">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-8 max-w-3xl">
                    <DomainRuleFields
                        title="Admin console logins"
                        idPrefix="admin"
                        enabled={adminEnabled}
                        threshold={adminThreshold}
                        windowSeconds={adminWindowSeconds}
                        banSeconds={adminBanSeconds}
                        onEnabledChange={setAdminEnabled}
                        onThresholdChange={setAdminThreshold}
                        onWindowSecondsChange={setAdminWindowSeconds}
                        onBanSecondsChange={setAdminBanSeconds}
                    />

                    <DomainRuleFields
                        title="Application logins"
                        idPrefix="application"
                        enabled={applicationEnabled}
                        threshold={applicationThreshold}
                        windowSeconds={applicationWindowSeconds}
                        banSeconds={applicationBanSeconds}
                        onEnabledChange={setApplicationEnabled}
                        onThresholdChange={setApplicationThreshold}
                        onWindowSecondsChange={setApplicationWindowSeconds}
                        onBanSecondsChange={setApplicationBanSeconds}
                    />
                </div>

                <div className="flex justify-end pt-2 max-w-3xl">
                    <Submit loading={saving} label="Save settings" disabled={!canSubmit || saving} />
                </div>
            </form>
        </div>
    );
};

export default LoginBanRulesSettings;
