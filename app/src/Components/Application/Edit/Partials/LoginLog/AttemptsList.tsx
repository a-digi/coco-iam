import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import Pagination from '../../../../../Shared/Components/Pagination/Pagination';
import Dropdown from '../../../../../Shared/Components/Dropdown/Dropdown';
import { FormInput } from '../../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../../config/data/date/date';
import { ApplicationResource } from '../../../model/application';
import type { ApplicationLoginAttempt, ApplicationLoginAttemptListResponse } from './types';

const PAGE_SIZE = 20;

const SUCCESS_OPTIONS = [
    { label: 'All', value: '' },
    { label: 'Success', value: 'true' },
    { label: 'Failed', value: 'false' },
];

// dateInputToFrom/dateInputToTo convert an <input type="date"> value
// ("2026-07-01") into an RFC3339 instant the backend's parseTimeFilter
// accepts — start-of-day for "from", end-of-day for "to" so the
// filter is inclusive of the whole selected day. Same helper as
// AdminLoginLogDashboard's own (each component keeps its own copy,
// matching this codebase's convention).
const dateInputToFrom = (v: string): string => (v ? `${v}T00:00:00Z` : '');
const dateInputToTo = (v: string): string => (v ? `${v}T23:59:59Z` : '');

interface AttemptsListProps {
    applicationId: string;
    /** When set, browses a rotated-out archive instead of the live database. */
    archiveId?: string;
}

// AttemptsList shows one application's end-user login attempts —
// same filter/pagination shape as AdminLoginLogDashboard.tsx, adapted
// from a routed useParams()-driven component to a plain prop-driven
// one, since per-application sections here are SideMenu items, not
// routes (see LoginLogSection.tsx's own doc comment for why). See
// plan/login-audit-log/plan.md Step 12.
export const AttemptsList: React.FC<AttemptsListProps> = ({ applicationId, archiveId }) => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [attempts, setAttempts] = useState<ApplicationLoginAttempt[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);

    const [usernameFilter, setUsernameFilter] = useState('');
    const [userIdFilter, setUserIdFilter] = useState('');
    const [successFilter, setSuccessFilter] = useState('');
    const [ipFilter, setIpFilter] = useState('');
    const [fromFilter, setFromFilter] = useState('');
    const [toFilter, setToFilter] = useState('');

    const [appliedUsername, setAppliedUsername] = useState('');
    const [appliedUserId, setAppliedUserId] = useState('');
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
            if (appliedUserId) params.set('application_user_id', appliedUserId);
            if (appliedSuccess) params.set('success', appliedSuccess);
            if (appliedIp) params.set('ip', appliedIp);
            if (appliedFrom) params.set('from', appliedFrom);
            if (appliedTo) params.set('to', appliedTo);

            const base = archiveId
                ? `applications/{${ApplicationResource}}/{id:${applicationId}}/login-log/archives/{archiveId:${archiveId}}/attempts`
                : `applications/{${ApplicationResource}}/{id:${applicationId}}/login-log`;
            const resp = await get<{ message: ApplicationLoginAttemptListResponse }>(`${base}?${params.toString()}`);
            setAttempts(resp.message.attempts);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load login attempts.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page, applicationId, archiveId, appliedUsername, appliedUserId, appliedSuccess, appliedIp, appliedFrom, appliedTo]);

    useEffect(() => {
        void load();
    }, [load]);

    const applyFilters = () => {
        setPage(1);
        setAppliedUsername(usernameFilter.trim());
        setAppliedUserId(userIdFilter.trim());
        setAppliedSuccess(successFilter);
        setAppliedIp(ipFilter.trim());
        setAppliedFrom(dateInputToFrom(fromFilter));
        setAppliedTo(dateInputToTo(toFilter));
    };

    const columns: TableColumn<ApplicationLoginAttempt>[] = [
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
            <div className="flex flex-wrap items-end gap-3 mb-4">
                <FormInput
                    id="app-login-log-username-filter"
                    label="Username"
                    value={usernameFilter}
                    onChange={setUsernameFilter}
                    placeholder="jdoe"
                    className="w-44"
                />
                <FormInput
                    id="app-login-log-user-id-filter"
                    label="User ID"
                    value={userIdFilter}
                    onChange={setUserIdFilter}
                    placeholder="a3bead61-..."
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
                    id="app-login-log-ip-filter"
                    label="IP address"
                    value={ipFilter}
                    onChange={setIpFilter}
                    placeholder="203.0.113.7"
                    className="w-44"
                />
                <FormInput
                    id="app-login-log-from-filter"
                    type="date"
                    label="From"
                    value={fromFilter}
                    onChange={setFromFilter}
                    className="w-40"
                />
                <FormInput
                    id="app-login-log-to-filter"
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

export default AttemptsList;
