import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../../config/data/date/date';

interface StatusResponse {
    running: boolean;
    pid?: number;
    enabled: boolean;
    last_pulled_at?: string;
    country_range_count: number;
    asn_range_count: number;
}

const STATUS_POLL_MS = 5000;

// ExecutableTab owns the geoip-updater process control panel
// (status/PID/last-pulled/range counts + Start/Stop/Sync now) —
// relocated verbatim (no logic changes) out of the old single-page
// GeoIPSettings.tsx as part of the Settings / Executable / IP search
// tab split. See plan/geoip-enrichment/plan.md's "Frontend redesign"
// section.
export const ExecutableTab: React.FC = () => {
    const { post, get } = useHttpClient();
    const { errorMessage, successMessage } = useSnackBar();

    const [status, setStatus] = useState<StatusResponse | null>(null);
    const [starting, setStarting] = useState(false);
    const [stopping, setStopping] = useState(false);
    const [syncing, setSyncing] = useState(false);

    const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

    const loadStatus = useCallback(async () => {
        try {
            const resp = await get<{ message: StatusResponse }>('admin/security/geoip/status');
            setStatus(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load GeoIP updater status.');
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void loadStatus();
        pollRef.current = setInterval(() => void loadStatus(), STATUS_POLL_MS);
        return () => {
            if (pollRef.current) clearInterval(pollRef.current);
        };
    }, [loadStatus]);

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

    return (
        <div className="max-w-md space-y-4">
            <p className="text-xs text-amber-700 dark:text-amber-400">
                Starting/stopping the updater affects a real background process on this server.
            </p>

            <div className="grid grid-cols-2 gap-y-2 text-sm">
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
    );
};

export default ExecutableTab;
