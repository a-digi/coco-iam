import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form/Input/FormInput';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import type { AttackBanRuleSettings } from './types';

// AttackBanRulesSettings is the admin UI for configuring the single,
// global ban rule for high-volume scan/probe traffic against
// nonexistent routes. Unlike login-ban-rules there's only one block —
// scan traffic isn't scoped to admin vs application logins — so, unlike
// LoginBanRulesSettings, this doesn't need a reusable per-domain fields
// component. Mirrors LoginBanRulesSettings.tsx's form-state handling,
// snackbar feedback, and re-read-after-save. See plan/attack-ban-rules/plan.md.
export const AttackBanRulesSettings: React.FC = () => {
    const { get, put } = useHttpClient();
    const { errorMessage, successMessage } = useSnackBar();

    const [enabled, setEnabled] = useState(false);
    const [threshold, setThreshold] = useState('50');
    const [windowSeconds, setWindowSeconds] = useState('60');
    const [banSeconds, setBanSeconds] = useState('3600');

    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const applySettings = (s: AttackBanRuleSettings) => {
        setEnabled(s.enabled);
        setThreshold(String(s.threshold));
        setWindowSeconds(String(s.window_seconds));
        setBanSeconds(String(s.ban_seconds));
    };

    const loadSettings = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: AttackBanRuleSettings }>('admin/security/attack-bans/settings');
            applySettings(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load attack ban rule settings.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadSettings();
    }, [loadSettings]);

    const canSubmit = !enabled || (
        Number(threshold) >= 1 && Number(windowSeconds) > 0 && Number(banSeconds) > 0
    );

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setSaving(true);
        try {
            const resp = await put<{ message: AttackBanRuleSettings }>('admin/security/attack-bans/settings', {
                enabled,
                threshold: Number(threshold),
                window_seconds: Number(windowSeconds),
                ban_seconds: Number(banSeconds),
            });
            applySettings(resp.message);
            successMessage('Attack ban rule settings saved.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save attack ban rule settings.');
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
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-2">Attack ban rules</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    Automatically bans an IP once it crosses a configured number of requests to
                    nonexistent routes within a time window — the "scan/probe" traffic shown on
                    the Attacks page. Bans go through the same mechanism as every other ban in
                    this system, so the IP allowlist and the Bans page apply automatically.
                </p>
            </div>

            <p className="text-xs text-amber-700 dark:text-amber-400">
                A low threshold can ban a trusted IP that happens to hit a handful of stale or
                mistyped URLs. Add trusted IPs to the{' '}
                <Link to="/admin/security/allowlist" className="underline">
                    IP allowlist
                </Link>{' '}
                before tightening this rule.
            </p>

            <form onSubmit={handleSubmit} className="space-y-8">
                <div className="space-y-4 max-w-md">
                    <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                        <input
                            type="checkbox"
                            className="accent-indigo-600"
                            checked={enabled}
                            onChange={e => setEnabled(e.target.checked)}
                        />
                        Enabled
                    </label>

                    <FormInput
                        id="attack-bans-threshold"
                        label="Probe hits"
                        type="number"
                        value={threshold}
                        onChange={setThreshold}
                        disabled={!enabled}
                        min={1}
                        description="Number of hits to nonexistent routes from one IP that triggers a ban."
                    />

                    <FormInput
                        id="attack-bans-window-seconds"
                        label="Time window (seconds)"
                        type="number"
                        value={windowSeconds}
                        onChange={setWindowSeconds}
                        disabled={!enabled}
                        min={1}
                        description="Hits are only counted since the start of the current scan episode; a burst spread over more than this many seconds is not caught."
                    />

                    <FormInput
                        id="attack-bans-ban-seconds"
                        label="Ban duration (seconds)"
                        type="number"
                        value={banSeconds}
                        onChange={setBanSeconds}
                        disabled={!enabled}
                        min={1}
                        description="How long the IP stays banned once triggered."
                    />
                </div>

                <div className="flex justify-end pt-2 max-w-md">
                    <Submit loading={saving} label="Save settings" disabled={!canSubmit || saving} />
                </div>
            </form>
        </div>
    );
};

export default AttackBanRulesSettings;
