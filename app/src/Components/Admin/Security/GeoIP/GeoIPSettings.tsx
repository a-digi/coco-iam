import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../config/data/date/date';

interface SettingsResponse {
    enabled: boolean;
    maxmind_account_id: string;
    maxmind_license_key_mask?: string;
    check_interval_seconds: number;
    pull_interval_hours: number;
}

interface StatusResponse {
    running: boolean;
    pid?: number;
    enabled: boolean;
    last_pulled_at?: string;
    country_range_count: number;
    asn_range_count: number;
}

const STATUS_POLL_MS = 5000;

// GeoIPSettings is the admin UI for configuring MaxMind GeoLite2
// credentials and controlling the geoip-updater process. Deliberately
// lives inside Components/Admin/Security/GeoIP (not e.g. Settings/)
// to mirror the backend's "everything within api/src/security, no
// shared code" placement for this feature — see
// plan/geoip-enrichment/plan.md.
export const GeoIPSettings: React.FC = () => {
    const { get, put, post } = useHttpClient();
    const { errorMessage, successMessage } = useSnackBar();

    const [enabled, setEnabled] = useState(false);
    const [accountId, setAccountId] = useState('');
    const [licenseKey, setLicenseKey] = useState('');
    const [licenseKeyMask, setLicenseKeyMask] = useState('');
    const [checkIntervalSeconds, setCheckIntervalSeconds] = useState('600');
    const [pullIntervalHours, setPullIntervalHours] = useState('24');
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    const [status, setStatus] = useState<StatusResponse | null>(null);
    const [starting, setStarting] = useState(false);
    const [stopping, setStopping] = useState(false);
    const [syncing, setSyncing] = useState(false);

    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

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

    const loadStatus = useCallback(async () => {
        try {
            const resp = await get<{ message: StatusResponse }>('admin/security/geoip/status');
            setStatus(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load GeoIP updater status.');
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadSettings();
        void loadStatus();
        pollRef.current = setInterval(() => void loadStatus(), STATUS_POLL_MS);
        return () => {
            if (pollRef.current) clearInterval(pollRef.current);
        };
    }, [loadSettings, loadStatus]);

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

    const handleStart = async () => {
        setStarting(true);
        try {
            const resp = await post<{ message: StatusResponse }>('admin/security/geoip/start');
            setStatus(resp.message);
            successMessage('geoip-updater started.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to start geoip-updater.');
        } finally {
            setStarting(false);
            void loadStatus();
        }
    };

    const handleStop = async () => {
        setStopping(true);
        try {
            const resp = await post<{ message: StatusResponse }>('admin/security/geoip/stop');
            setStatus(resp.message);
            successMessage('geoip-updater stopped.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to stop geoip-updater.');
        } finally {
            setStopping(false);
            void loadStatus();
        }
    };

    // Forces an out-of-cycle pull on the already-running updater
    // (bypasses pull_interval_hours) — see
    // plan/geoip-enrichment/plan.md's "Extension: database stats +
    // manual sync" section. The pull itself is async: this response
    // reflects the still-current (not yet refreshed) stats, and the
    // existing 5s poll picks up the real result once it lands.
    const handleSync = async () => {
        setSyncing(true);
        try {
            const resp = await post<{ message: StatusResponse }>('admin/security/geoip/sync');
            setStatus(resp.message);
            successMessage('Sync requested — check back shortly.');
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to request a sync.');
        } finally {
            setSyncing(false);
            void loadStatus();
        }
    };

    if (loading) {
        return <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>;
    }

    return (
        <div className="space-y-8">
            <div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-2">GeoIP enrichment</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    Enriches attack and port-scan episodes with country/ISP data from MaxMind GeoLite2. A
                    separate geoip-updater process pulls fresh data on a loop and rebuilds a standalone
                    database — no historical IP data is retained.
                </p>
                <p className="text-xs text-amber-700 dark:text-amber-400 mt-2">
                    Changing these settings takes effect the next time the geoip-updater process is
                    (re)started, not while it is already running. Starting/stopping the updater affects a
                    real background process on this server.
                </p>
            </div>

            <form onSubmit={handleSubmit} className="space-y-5 max-w-md">
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

            <div className="border-t border-gray-200 dark:border-surface-800 pt-6 max-w-md">
                <h3 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-3">Updater process</h3>
                <div className="grid grid-cols-2 gap-y-2 text-sm mb-4">
                    <div className="text-gray-500 dark:text-gray-400">Status</div>
                    <div className={status?.running ? 'text-green-600 dark:text-green-400' : 'text-gray-700 dark:text-gray-300'}>
                        {status?.running ? 'Running' : 'Stopped'}
                    </div>
                    {status?.running && status.pid && (
                        <>
                            <div className="text-gray-500 dark:text-gray-400">PID</div>
                            <div className="font-mono text-gray-900 dark:text-gray-100">{status.pid}</div>
                        </>
                    )}
                    <div className="text-gray-500 dark:text-gray-400">Last pulled</div>
                    <div className="text-gray-900 dark:text-gray-100">
                        {status?.last_pulled_at ? formatDate(status.last_pulled_at) : '—'}
                    </div>
                    <div className="text-gray-500 dark:text-gray-400">Country ranges</div>
                    <div className="text-gray-900 dark:text-gray-100">
                        {status?.last_pulled_at ? status.country_range_count.toLocaleString() : '—'}
                    </div>
                    <div className="text-gray-500 dark:text-gray-400">ASN ranges</div>
                    <div className="text-gray-900 dark:text-gray-100">
                        {status?.last_pulled_at ? status.asn_range_count.toLocaleString() : '—'}
                    </div>
                </div>
                <div className="flex gap-2">
                    <button
                        type="button"
                        onClick={() => void handleStart()}
                        disabled={starting || stopping || syncing || !!status?.running}
                        className="px-4 py-2 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500 disabled:opacity-60 disabled:cursor-not-allowed"
                    >
                        {starting ? 'Starting…' : 'Start'}
                    </button>
                    <button
                        type="button"
                        onClick={() => void handleStop()}
                        disabled={starting || stopping || syncing || !status?.running}
                        className="px-4 py-2 rounded-md bg-gray-200 dark:bg-surface-700 text-gray-900 dark:text-gray-100 text-sm font-medium hover:bg-gray-300 dark:hover:bg-surface-600 disabled:opacity-60 disabled:cursor-not-allowed"
                    >
                        {stopping ? 'Stopping…' : 'Stop'}
                    </button>
                    <button
                        type="button"
                        onClick={() => void handleSync()}
                        disabled={starting || stopping || syncing || !status?.running}
                        title={!status?.running ? 'Start the updater before requesting a manual sync' : undefined}
                        className="px-4 py-2 rounded-md bg-gray-200 dark:bg-surface-700 text-gray-900 dark:text-gray-100 text-sm font-medium hover:bg-gray-300 dark:hover:bg-surface-600 disabled:opacity-60 disabled:cursor-not-allowed"
                    >
                        {syncing ? 'Syncing…' : 'Sync now'}
                    </button>
                </div>
            </div>
        </div>
    );
};

export default GeoIPSettings;
