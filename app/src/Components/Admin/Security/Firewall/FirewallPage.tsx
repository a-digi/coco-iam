import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import Alert from '../../../../Shared/Components/Alert/Alert';
import { Submit } from '../../../../Shared/Components/Button';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

// Same shape FirewallStatusBanner.tsx already fetches from
// GET admin/security/status — this page reuses that existing
// endpoint rather than adding a second one just for a fuller display.
interface SecurityStatus {
    os: string;
    firewall_backend: string;
    firewall_available: boolean;
    firewall_detail?: string;
}

interface FirewallResyncResponse {
    synced: number;
    skipped_expired: number;
    failed: number;
}

interface FirewallRulesResponse {
    backend: string;
    rules: string[];
}

const WRITE_SCOPES = [AppScopes.AdminSecurityFirewallWrite, AppScopes.SuperAdmin];

// FirewallPage is the dedicated Security > Firewall view — a fuller
// display of the same OS-level enforcement status
// FirewallStatusBanner.tsx already shows compressed into a banner,
// plus the one new capability this page adds: manually resyncing
// every currently-active DB ban back into the OS firewall (useful
// after a host reboot or a manual `iptables -F`/pf reload). See
// plan/firewall-page/plan.md.
export const FirewallPage: React.FC = () => {
    const { get, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [status, setStatus] = useState<SecurityStatus | null>(null);
    const [rules, setRules] = useState<FirewallRulesResponse | null>(null);
    const [loading, setLoading] = useState(true);
    const [resyncing, setResyncing] = useState(false);

    const loadRules = useCallback(async () => {
        try {
            const resp = await get<{ message: FirewallRulesResponse }>('admin/security/firewall/rules');
            setRules(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load firewall rules.');
        }
    }, [get, errorMessage]);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const resp = await get<{ message: SecurityStatus }>('admin/security/status');
            setStatus(resp.message);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load firewall status.');
        } finally {
            setLoading(false);
        }
        await loadRules();
    }, [get, errorMessage, loadRules]);

    useEffect(() => {
        void load();
    }, [load]);

    const handleResync = async () => {
        setResyncing(true);
        try {
            const resp = await post<{ message: FirewallResyncResponse }>('admin/security/firewall/resync', {});
            const r = resp.message;
            successMessage(
                `Resync complete — ${r.synced} synced, ${r.skipped_expired} skipped (expired), ${r.failed} failed.`
            );
            await loadRules();
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to resync firewall.');
        } finally {
            setResyncing(false);
        }
    };

    if (loading) {
        return <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>;
    }
    if (!status) {
        return null;
    }

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-2">Firewall</h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                    OS-level enforcement of IP bans — additive to the always-on application-layer rate
                    limiting, not a replacement for it.
                </p>
            </div>

            <div className="max-w-2xl space-y-4">
                <div className="grid grid-cols-2 gap-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                    <div>
                        <div className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1">
                            Operating system
                        </div>
                        <div className="text-sm text-gray-900 dark:text-gray-100">{status.os}</div>
                    </div>
                    <div>
                        <div className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1">
                            Backend
                        </div>
                        <div className="text-sm text-gray-900 dark:text-gray-100">{status.firewall_backend}</div>
                    </div>
                </div>

                {status.firewall_available ? (
                    <Alert variant="success" title="Firewall enforcement active">
                        Banned IPs are blocked at the OS level via <strong>{status.firewall_backend}</strong> (
                        {status.os}), in addition to application-layer rate limiting.
                    </Alert>
                ) : (
                    <>
                        <Alert variant="warning" title="Firewall enforcement unavailable">
                            Only application-layer rate limiting is active on this host ({status.os}) — banned IPs
                            are still rejected here, but not blocked at the network level.
                            {status.firewall_detail ? ` ${status.firewall_detail}.` : ''}
                        </Alert>

                        {status.os === 'darwin' && (
                            <div className="text-xs text-gray-500 dark:text-gray-400 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                                <p className="mb-2">
                                    One-time setup on macOS — add to <code>/etc/pf.conf</code>, then run{' '}
                                    <code>pfctl -e</code>:
                                </p>
                                <pre className="whitespace-pre-wrap font-mono bg-white dark:bg-surface-800 p-2 rounded border border-gray-200 dark:border-gray-700">
{`table <coco_iam_banned> persist
block drop from <coco_iam_banned> to any`}
                                </pre>
                                <p className="mt-2">
                                    Not automated by this app — rewriting <code>/etc/pf.conf</code> programmatically
                                    risks clobbering the host's existing firewall rules.
                                </p>
                            </div>
                        )}

                        {status.os === 'linux' && (
                            <div className="text-xs text-gray-500 dark:text-gray-400 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                                <code>iptables</code> was not found in PATH. On Alpine: <code>apk add iptables</code>.
                            </div>
                        )}
                    </>
                )}

                {rules && (
                    <div className="p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                        <div className="text-xs uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-2">
                            Currently blocked IPs (OS level)
                        </div>
                        {rules.rules.length === 0 ? (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                                No IPs are currently blocked at the OS level via {rules.backend}.
                            </p>
                        ) : (
                            <ul className="text-sm text-gray-900 dark:text-gray-100 font-mono space-y-1">
                                {rules.rules.map(ip => (
                                    <li key={ip}>{ip}</li>
                                ))}
                            </ul>
                        )}
                        <p className="text-xs text-gray-500 dark:text-gray-400 mt-2">
                            Read live from {rules.backend} — informational, not authoritative. The{' '}
                            <Link to="/admin/security/bans" className="underline">
                                Bans
                            </Link>{' '}
                            page is the source of truth for what should be banned.
                        </p>
                    </div>
                )}

                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <div className="flex items-center gap-3 pt-2">
                        <Submit
                            loading={resyncing}
                            label="Resync now"
                            onClick={() => void handleResync()}
                            disabled={resyncing}
                        />
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                            Re-applies every currently-active ban into the OS firewall — useful after a host
                            reboot or a manual firewall flush.
                        </span>
                    </div>
                </ScopeBasedComponentAccess>
            </div>
        </div>
    );
};

export default FirewallPage;
