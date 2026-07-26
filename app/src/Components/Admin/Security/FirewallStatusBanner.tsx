import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Alert from '../../../Shared/Components/Alert/Alert';
import { ProgressLine } from '../../../Shared/Components/Progress';

interface SecurityStatus {
    os: string;
    firewall_backend: string;
    firewall_available: boolean;
    firewall_detail?: string;
    ip_attacks_entry_count: number;
    ip_attacks_threshold: number;
    ip_attacks_archives_count: number;
    scan_watch_source: string;
    scan_watch_available: boolean;
    scan_watch_detail?: string;
}

// ArchiveProgress shows how close ip-attacks.db is to its next
// rotation — see plan/ip-attacks-db-archiving/plan.md. A separate,
// small element from the firewall Alert above it (per that plan's
// "next to the existing firewall-availability banner" design), since
// it's informational rather than a warning.
const ArchiveProgress: React.FC<{ status: SecurityStatus }> = ({ status }) => {
    if (status.ip_attacks_threshold <= 0) {
        return null;
    }
    const remaining = Math.max(0, status.ip_attacks_threshold - status.ip_attacks_entry_count);
    return (
        <div className="mb-4 text-sm text-gray-600 dark:text-gray-400">
            <div className="flex items-center justify-between mb-1">
                <span>
                    ip-attacks.db: {status.ip_attacks_entry_count.toLocaleString()} / {status.ip_attacks_threshold.toLocaleString()} rows
                    until the next archive rotation
                </span>
                <span>{status.ip_attacks_archives_count.toLocaleString()} archive{status.ip_attacks_archives_count === 1 ? '' : 's'} so far</span>
            </div>
            <ProgressLine
                segments={[
                    { value: status.ip_attacks_entry_count, color: 'info', label: 'Current entries' },
                    { value: remaining, color: 'neutral', label: 'Remaining until rotation' },
                ]}
            />
        </div>
    );
};

// ScanWatchStatus reports whether port-scan detection actually has a
// log source on this host — see plan/port-scan-detection/plan.md
// Phase B. When unavailable, there is zero visibility into scanning
// against ports coco-iam doesn't listen on at all, which is worth
// surfacing the same way the firewall-unavailable warning is.
const ScanWatchStatus: React.FC<{ status: SecurityStatus }> = ({ status }) => {
    if (status.scan_watch_available) {
        return (
            <Alert variant="success" title="Port-scan detection active" className="mb-4">
                Ingesting kernel firewall log lines via <strong>{status.scan_watch_source}</strong> to detect scanning
                against ports this app doesn't listen on.
            </Alert>
        );
    }
    return (
        <Alert variant="warning" title="Port-scan detection unavailable" className="mb-4">
            No log source was found for port-scan detection — scanning against ports this app doesn't listen on
            is currently invisible.{status.scan_watch_detail ? ` ${status.scan_watch_detail}.` : ''}
        </Alert>
    );
};

// FirewallStatusBanner is fetched once by SecurityPage and shown above
// whichever tab is active — see plan/ip-abuse-protection/plan.md
// sections 13-14: don't let the page silently imply full network-level
// blocking when only application-layer rate limiting (429s) is
// actually protecting this host.
export const FirewallStatusBanner: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();
    const [status, setStatus] = useState<SecurityStatus | null>(null);

    useEffect(() => {
        (async () => {
            try {
                const resp = await get<{ message: SecurityStatus }>('admin/security/status');
                setStatus(resp.message);
            } catch (err: unknown) {
                errorMessage(err instanceof Error ? err.message : 'Failed to load firewall status.');
            }
        })();
    }, [get, errorMessage]);

    if (!status) {
        return null;
    }

    return (
        <>
            {status.firewall_available ? (
                <Alert variant="success" title="Firewall enforcement active" className="mb-4">
                    Banned IPs are blocked at the OS level via <strong>{status.firewall_backend}</strong> ({status.os}),
                    in addition to application-layer rate limiting.
                </Alert>
            ) : (
                <Alert variant="warning" title="Firewall enforcement unavailable" className="mb-4">
                    Only application-layer rate limiting is active on this host ({status.os}) — banned IPs are still
                    rejected here, but not blocked at the network level.{status.firewall_detail ? ` ${status.firewall_detail}.` : ''}
                </Alert>
            )}
            <ArchiveProgress status={status} />
            <ScanWatchStatus status={status} />
        </>
    );
};

export default FirewallStatusBanner;
