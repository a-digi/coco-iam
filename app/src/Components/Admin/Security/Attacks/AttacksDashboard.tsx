import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Pagination from '../../../../Shared/Components/Pagination/Pagination';
import Dropdown from '../../../../Shared/Components/Dropdown/Dropdown';
import { LinkAction } from '../../../../Shared/Components/Actions/Link';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';

interface Attack {
    id: string;
    ip: string;
    tier: string;
    started_at: string;
    last_seen_at: string;
    ended_at?: string;
    hit_count: number;
    ban_count: number;
}

interface AttackListResponse {
    attacks: Attack[];
    total: number;
    limit: number;
    offset: number;
}

const PAGE_SIZE = 20;

const TIER_OPTIONS = [
    { label: 'All tiers', value: '' },
    { label: 'Global', value: 'global' },
    { label: 'Sensitive', value: 'sensitive' },
    { label: 'Manual', value: 'manual' },
];

// AttacksDashboard is the first real caller of Pagination.tsx — every
// other admin list fakes single-page pagination over a fully-fetched
// collection, but attack history is a genuinely growing log with real
// server-side limit/offset/total (see
// plan/ip-abuse-protection/frontend-plan.md's "Attacks list needs
// real pagination" decision).
export const AttacksDashboard: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [attacks, setAttacks] = useState<Attack[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);

    const [ipFilter, setIpFilter] = useState('');
    const [tierFilter, setTierFilter] = useState('');
    const [activeOnly, setActiveOnly] = useState(false);

    // Only the applied values drive the fetch, so typing in the IP
    // box doesn't refetch on every keystroke — an explicit "Filter"
    // click (or toggling the checkbox) applies them.
    const [appliedIp, setAppliedIp] = useState('');
    const [appliedTier, setAppliedTier] = useState('');

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            params.set('limit', String(PAGE_SIZE));
            params.set('offset', String((page - 1) * PAGE_SIZE));
            if (appliedIp) params.set('ip', appliedIp);
            if (appliedTier) params.set('tier', appliedTier);
            if (activeOnly) params.set('active', 'true');

            const resp = await get<{ message: AttackListResponse }>(`admin/security/attacks?${params.toString()}`);
            setAttacks(resp.message.attacks);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load attacks.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page, appliedIp, appliedTier, activeOnly]);

    useEffect(() => {
        void load();
    }, [load]);

    const applyFilters = () => {
        setPage(1);
        setAppliedIp(ipFilter.trim());
        setAppliedTier(tierFilter);
    };

    const columns: TableColumn<Attack>[] = [
        { key: 'ip', label: 'IP address' },
        { key: 'tier', label: 'Tier' },
        { key: 'hit_count', label: 'Hits' },
        { key: 'ban_count', label: 'Bans' },
        {
            key: 'started_at',
            label: 'Started',
            render: value => new Date(String(value)).toLocaleString(),
        },
        {
            key: 'ended_at',
            label: 'Status',
            render: value => (value ? 'Closed' : 'Active'),
        },
        {
            key: 'actions',
            label: '',
            render: (_value, row) => (
                <LinkAction to={`/admin/security/attacks/${row.id}`} label="View details" />
            ),
        },
    ];

    return (
        <div>
            <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4">Attack episodes</h2>

            <div className="flex flex-wrap items-end gap-3 mb-4">
                <FormInput
                    id="attacks-ip-filter"
                    label="IP address"
                    value={ipFilter}
                    onChange={setIpFilter}
                    placeholder="203.0.113.7"
                    className="w-56"
                />
                <div>
                    <div className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Tier</div>
                    <Dropdown
                        options={TIER_OPTIONS}
                        value={tierFilter}
                        onChange={opt => setTierFilter(String(opt.value))}
                        className="w-40"
                    />
                </div>
                <label className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 h-10">
                    <input
                        type="checkbox"
                        className="accent-indigo-600"
                        checked={activeOnly}
                        onChange={e => {
                            setActiveOnly(e.target.checked);
                            setPage(1);
                        }}
                    />
                    Active only
                </label>
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
                    <Table columns={columns} data={attacks} rowKey={row => row.id} emptyText="No attack episodes recorded." />
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

export default AttacksDashboard;
