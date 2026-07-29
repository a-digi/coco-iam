import React, { useCallback, useEffect, useState } from 'react';
import { Submit } from '../../../../../Shared/Components/Button';
import { FormInput } from '../../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';

interface SettingsResponse {
    enabled: boolean;
    maxmind_account_id: string;
    maxmind_license_key_mask?: string;
    check_interval_seconds: number;
    pull_interval_hours: number;
}

// SettingsTab owns the MaxMind credentials + interval form — relocated
// verbatim (no logic changes) out of the old single-page
// GeoIPSettings.tsx as part of the Settings / Executable / IP search
// tab split. See plan/geoip-enrichment/plan.md's "Frontend redesign"
// section.
export const SettingsTab: React.FC = () => {
    const { get, put } = useHttpClient();
    const { errorMessage, successMessage } = useSnackBar();

    const [enabled, setEnabled] = useState(false);
    const [accountId, setAccountId] = useState('');
    const [licenseKey, setLicenseKey] = useState('');
    const [licenseKeyMask, setLicenseKeyMask] = useState('');
    const [checkIntervalSeconds, setCheckIntervalSeconds] = useState('600');
    const [pullIntervalHours, setPullIntervalHours] = useState('24');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const loadSettings = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: SettingsResponse }>('admin/security/geoip/settings');
            const s = resp.message;
            setEnabled(s.enabled);
            setAccountId(s.maxmind_account_id);
            setLicenseKeyMask(s.maxmind_license_key_mask ?? '');
            setCheckIntervalSeconds(String(s.check_interval_seconds));
            setPullIntervalHours(String(s.pull_interval_hours));
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load GeoIP settings.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadSettings();
    }, [loadSettings]);

    const canSubmit = Number(checkIntervalSeconds) > 0 && Number(pullIntervalHours) > 0;

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!canSubmit) return;
        setSaving(true);
        try {
            const resp = await put<{ message: SettingsResponse }>('admin/security/geoip/settings', {
                enabled,
                maxmind_account_id: accountId.trim(),
                maxmind_license_key: licenseKey.trim(),
                check_interval_seconds: Number(checkIntervalSeconds),
                pull_interval_hours: Number(pullIntervalHours),
            });
            const s = resp.message;
            setEnabled(s.enabled);
            setAccountId(s.maxmind_account_id);
            setLicenseKeyMask(s.maxmind_license_key_mask ?? '');
            setCheckIntervalSeconds(String(s.check_interval_seconds));
            setPullIntervalHours(String(s.pull_interval_hours));
            setLicenseKey('');
            successMessage('GeoIP settings saved.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to save GeoIP settings.');
        } finally {
            setSaving(false);
        }
    };

    if (loading) {
        return <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>;
    }

    return (
        <div className="space-y-5 max-w-md">
            <p className="text-xs text-amber-700 dark:text-amber-400">
                Changing these settings takes effect the next time the geoip-updater process is
                (re)started, not while it is already running.
            </p>

            <form onSubmit={handleSubmit} className="space-y-5">
                <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <input
                        type="checkbox"
                        className="accent-indigo-600"
                        checked={enabled}
                        onChange={e => setEnabled(e.target.checked)}
                    />
                    Enabled
                </label>

                <p className="text-xs text-gray-500 dark:text-gray-400 -mt-2">
                    Don't have an account ID or license key yet?{' '}
                    <a
                        href="https://www.maxmind.com/en/geolite2/signup"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-indigo-600 dark:text-indigo-400 hover:underline"
                    >
                        Sign up for a free MaxMind GeoLite2 account
                    </a>
                    , then generate a license key from your account's{' '}
                    <a
                        href="https://www.maxmind.com/en/accounts/current/license-key"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-indigo-600 dark:text-indigo-400 hover:underline"
                    >
                        license key page
                    </a>
                    .
                </p>

                <FormInput
                    id="geoip-account-id"
                    label="MaxMind account ID"
                    value={accountId}
                    onChange={setAccountId}
                    placeholder="123456"
                />

                <FormInput
                    id="geoip-license-key"
                    label="MaxMind license key"
                    type="password"
                    value={licenseKey}
                    onChange={setLicenseKey}
                    placeholder={licenseKeyMask ? `Leave blank to keep ${licenseKeyMask}` : 'a1b2c3d4e5f6'}
                    description={licenseKeyMask ? `Currently set (${licenseKeyMask}). Only fill in to change it.` : 'No key stored yet.'}
                />

                <FormInput
                    id="geoip-check-interval"
                    label="Check interval (seconds)"
                    type="number"
                    value={checkIntervalSeconds}
                    onChange={setCheckIntervalSeconds}
                    min={1}
                    description="How often the updater checks whether it's time to pull fresh data."
                />

                <FormInput
                    id="geoip-pull-interval"
                    label="Pull interval (hours)"
                    type="number"
                    value={pullIntervalHours}
                    onChange={setPullIntervalHours}
                    min={1}
                    description="Minimum time between actual data pulls from MaxMind."
                />

                <div className="flex justify-end pt-2">
                    <Submit loading={saving} label="Save settings" disabled={!canSubmit || saving} />
                </div>
            </form>
        </div>
    );
};

export default SettingsTab;
