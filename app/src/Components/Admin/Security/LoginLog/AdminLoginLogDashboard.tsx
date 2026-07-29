import React, { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Pagination from '../../../../Shared/Components/Pagination/Pagination';
import Dropdown from '../../../../Shared/Components/Dropdown/Dropdown';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../config/data/date/date';

interface AdminLoginAttempt {
    id: string;
    admin_user_id?: string;
    username: string;
    success: boolean;
    failure_reason?: string;
    ip: string;
    user_agent?: string;
    created_at: string;
}

interface AdminLoginAttemptListResponse {
    attempts: AdminLoginAttempt[];
    total: number;
    limit: number;
    offset: number;
}

const PAGE_SIZE = 20;

const SUCCESS_OPTIONS = [
    { label: 'All', value: '' },
    { label: 'Success', value: 'true' },
    { label: 'Failed', value: 'false' },
];

// dateInputToFrom/dateInputToTo convert an <input type="date"> value
// ("2026-07-01") into an RFC3339 instant the backend's parseTimeFilter
// accepts — start-of-day for "from", end-of-day for "to" so the
// filter is inclusive of the whole selected day (see
// plan/login-audit-log/plan.md Step 4's date-range handling).
const dateInputToFrom = (v: string): string => (v ? `${v}T00:00:00Z` : '');
const dateInputToTo = (v: string): string => (v ? `${v}T23:59:59Z` : '');

// AdminLoginLogDashboard lists admin-console login attempts (success
// and failure), newest first — mirrors AttacksDashboard.tsx's
// real-pagination pattern. Reused for browsing a rotated-out archive
// too (see plan/login-audit-log/plan.md Step 1's generalized
// archiver), mounted on /admin/security/login-log/archives/:archiveId/attempts
// instead of /admin/security/login-log — purely additive, archiveId
// is always undefined on the live route.
export const AdminLoginLogDashboard: React.FC = () => {
    const { archiveId } = useParams<{ archiveId?: string }>();
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [attempts, setAttempts] = useState<AdminLoginAttempt[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);

    const [usernameFilter, setUsernameFilter] = useState('');
    const [adminUserIdFilter, setAdminUserIdFilter] = useState('');
    const [successFilter, setSuccessFilter] = useState('');
    const [ipFilter, setIpFilter] = useState('');
    const [fromFilter, setFromFilter] = useState('');
    const [toFilter, setToFilter] = useState('');

    // Only the applied values drive the fetch — same reasoning as
    // AttacksDashboard's ip/tier: an explicit "Filter" click commits
    // whatever's currently typed, so typing doesn't refetch per
    // keystroke.
    const [appliedUsername, setAppliedUsername] = useState('');
    const [appliedAdminUserId, setAppliedAdminUserId] = useState('');
    const [appliedSuccess, setAppliedSuccess] = useState('');
    const [appliedIp, setAppliedIp] = useState('');
    const [appliedFrom, setAppliedFrom] = useState('');
    const [appliedTo, setAppliedTo] = useState('');

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            params.set('limit', String(PAGE_SIZE));
            params.set('offset', String((page - 1) * PAGE_SIZE));
            if (appliedUsername) params.set('username', appliedUsername);
            if (appliedAdminUserId) params.set('admin_user_id', appliedAdminUserId);
            if (appliedSuccess) params.set('success', appliedSuccess);
            if (appliedIp) params.set('ip', appliedIp);
            if (appliedFrom) params.set('from', appliedFrom);
            if (appliedTo) params.set('to', appliedTo);

            // Path param is named "id" on this route (not "archiveId")
            // — matches AdminLoginArchiveAttemptsHandler's own
            // uri.ExtractKeyAndValueFromURI(r.URL.Path) check, which
            // requires key == "id".
            const base = archiveId
                ? `admin/security/login-log/admin/archives/{id:${archiveId}}/attempts`
                : 'admin/security/login-log/admin';
            const resp = await get<{ message: AdminLoginAttemptListResponse }>(`${base}?${params.toString()}`);
            setAttempts(resp.message.attempts);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load login attempts.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page, appliedUsername, appliedAdminUserId, appliedSuccess, appliedIp, appliedFrom, appliedTo, archiveId]);

    useEffect(() => {
        void load();
    }, [load]);

    const applyFilters = () => {
        setPage(1);
        setAppliedUsername(usernameFilter.trim());
        setAppliedAdminUserId(adminUserIdFilter.trim());
        setAppliedSuccess(successFilter);
        setAppliedIp(ipFilter.trim());
        setAppliedFrom(dateInputToFrom(fromFilter));
        setAppliedTo(dateInputToTo(toFilter));
    };

    const columns: TableColumn<AdminLoginAttempt>[] = [
        { key: 'username', label: 'Username' },
        {
            key: 'success',
            label: 'Result',
            render: (_value, row) => (
                <span className={row.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'}>
                    {row.success ? 'Success' : 'Failed'}
                </span>
            ),
        },
        {
            key: 'failure_reason',
            label: 'Reason',
            render: (_value, row) => row.failure_reason || '—',
        },
        { key: 'ip', label: 'IP address' },
        {
            key: 'user_agent',
            label: 'User agent',
            render: (_value, row) => <span className="text-xs">{row.user_agent || '—'}</span>,
        },
        {
            key: 'created_at',
            label: 'When',
            render: value => formatDate(String(value)),
        },
    ];

    return (
        <div>
            {archiveId && (
                <div className="mb-4">
                    <Link
                        to="/admin/security/login-log/archives"
                        className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                    >
                        ← Back to archives
                    </Link>
                </div>
            )}

            <div className="flex items-center justify-between mb-4">
                <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">
                    {archiveId ? 'Archived admin login attempts' : 'Admin login attempts'}
                </h2>
                {!archiveId && (
                    <Link
                        to="/admin/security/login-log/archives"
                        className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                    >
                        View archives →
                    </Link>
                )}
            </div>

            <div className="flex flex-wrap items-end gap-3 mb-4">
                <FormInput
                    id="login-log-username-filter"
                    label="Username"
                    value={usernameFilter}
                    onChange={setUsernameFilter}
                    placeholder="jdoe"
                    className="w-44"
                />
                <FormInput
                    id="login-log-admin-user-id-filter"
                    label="Admin user ID"
                    value={adminUserIdFilter}
                    onChange={setAdminUserIdFilter}
                    placeholder="793424dd-..."
                    className="w-56"
                />
                <div>
                    <div className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Result</div>
                    <Dropdown
                        options={SUCCESS_OPTIONS}
                        value={successFilter}
                        onChange={opt => setSuccessFilter(String(opt.value))}
                        className="w-36"
                    />
                </div>
                <FormInput
                    id="login-log-ip-filter"
                    label="IP address"
                    value={ipFilter}
                    onChange={setIpFilter}
                    placeholder="203.0.113.7"
                    className="w-44"
                />
                <FormInput
                    id="login-log-from-filter"
                    type="date"
                    label="From"
                    value={fromFilter}
                    onChange={setFromFilter}
                    className="w-40"
                />
                <FormInput
                    id="login-log-to-filter"
                    type="date"
                    label="To"
                    value={toFilter}
                    onChange={setToFilter}
                    className="w-40"
                />
                <button
                    type="button"
                    onClick={applyFilters}
                    className="h-10 px-4 rounded-md bg-indigo-600 text-white text-sm font-medium hover:bg-indigo-500"
                >
                    Filter
                </button>
            </div>

            {loading ? (
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>
            ) : (
                <>
                    <Table columns={columns} data={attempts} rowKey={row => row.id} emptyText="No login attempts recorded." />
                    <Pagination
                        currentPage={page}
                        totalPages={Math.max(1, Math.ceil(total / PAGE_SIZE))}
                        onPageChange={setPage}
                    />
                </>
            )}
        </div>
    );
};

export default AdminLoginLogDashboard;
