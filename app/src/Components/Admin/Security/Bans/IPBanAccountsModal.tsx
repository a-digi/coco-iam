import React, { useEffect, useState } from 'react';
import { Modal } from '../../../../Shared/Components/Modal/Modal';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface FailedUsernameSummary {
    username: string;
    account_id?: string;
    attempts: number;
    last_attempt_at: string;
}

interface ApplicationFailedUsernameSummary extends FailedUsernameSummary {
    application_id: string;
    application_title: string;
}

interface IPBanAccountsResponse {
    admin_attempts: FailedUsernameSummary[] | null;
    application_attempts: ApplicationFailedUsernameSummary[] | null;
}

interface IPBanAccountsModalProps {
    ip: string | null;
    onClose: () => void;
}

const adminColumns: TableColumn<FailedUsernameSummary>[] = [
    { key: 'username', label: 'Username' },
    { key: 'attempts', label: 'Failed attempts' },
    {
        key: 'last_attempt_at',
        label: 'Last attempt',
        render: value => new Date(String(value)).toLocaleString(),
    },
];

const applicationColumns: TableColumn<ApplicationFailedUsernameSummary>[] = [
    { key: 'application_title', label: 'Application' },
    { key: 'username', label: 'Username' },
    { key: 'attempts', label: 'Failed attempts' },
    {
        key: 'last_attempt_at',
        label: 'Last attempt',
        render: value => new Date(String(value)).toLocaleString(),
    },
];

// IPBanAccountsModal shows which accounts a login-fraud-banned IP
// tried, fetched on demand (not inlined into the bans list, which
// would mean an expensive per-row lookup on every page load). See
// plan/ip-ban-accounts/plan.md.
//
// admin_attempts/application_attempts each being null (vs. an empty
// array) means the caller lacks that domain's own login-log read
// scope (admin:security:login-log:read / applications:login_log:read
// respectively) — distinct from "authorized, found nothing" —
// rendered as an explicit permission notice rather than silently
// showing an empty table.
export const IPBanAccountsModal: React.FC<IPBanAccountsModalProps> = ({ ip, onClose }) => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [data, setData] = useState<IPBanAccountsResponse | null>(null);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        if (!ip) {
            setData(null);
            return;
        }
        let cancelled = false;
        setLoading(true);
        (async () => {
            try {
                const resp = await get<{ message: IPBanAccountsResponse }>(
                    `admin/security/ip-bans/{ip:${ip}}/accounts`
                );
                if (!cancelled) setData(resp.message);
            } catch (err: unknown) {
                if (!cancelled) errorMessage(err instanceof Error ? err.message : 'Failed to load attempted accounts.');
            } finally {
                if (!cancelled) setLoading(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [ip, get, errorMessage]);

    return (
        <Modal isOpen={ip !== null} onClose={onClose} title={ip ? `Accounts targeted by ${ip}` : ''} maxWidth="lg">
            {loading ? (
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>
            ) : (
                <div className="space-y-6">
                    <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
                            Admin console logins
                        </h4>
                        {data?.admin_attempts === null || data?.admin_attempts === undefined ? (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                                You don't have permission to view attempted admin accounts
                                (requires admin:security:login-log:read).
                            </p>
                        ) : (
                            <Table
                                columns={adminColumns}
                                data={data.admin_attempts}
                                rowKey={row => row.username}
                                emptyText="No admin login attempts recorded for this IP."
                            />
                        )}
                    </div>

                    <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-gray-100 mb-2">
                            Application logins
                        </h4>
                        {data?.application_attempts === null || data?.application_attempts === undefined ? (
                            <p className="text-sm text-gray-500 dark:text-gray-400">
                                You don't have permission to view attempted application accounts
                                (requires applications:login_log:read).
                            </p>
                        ) : (
                            <Table
                                columns={applicationColumns}
                                data={data.application_attempts}
                                rowKey={row => `${row.application_id}-${row.username}`}
                                emptyText="No application login attempts recorded for this IP."
                            />
                        )}
                    </div>
                </div>
            )}
        </Modal>
    );
};

export default IPBanAccountsModal;
