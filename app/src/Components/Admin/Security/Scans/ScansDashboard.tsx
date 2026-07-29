import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Pagination from '../../../../Shared/Components/Pagination/Pagination';
import { LinkAction } from '../../../../Shared/Components/Actions/Link';
import { FormInput } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatGeoIPSummary } from '../geoipInfo';

interface Scan {
    id: string;
    ip: string;
    started_at: string;
    last_seen_at: string;
    ended_at?: string;
    distinct_ports: number;
    hit_count: number;
    sample_ports: string;
    geoip_info?: string;
}

interface ScanListResponse {
    scans: Scan[];
    total: number;
    limit: number;
    offset: number;
}

const PAGE_SIZE = 20;

// ScansDashboard mirrors AttacksDashboard.tsx's real-pagination
// pattern (this is genuinely growing history, same as attacks) — no
// tier filter, since every row here is the same kind of episode. See
// plan/port-scan-detection/plan.md Phase B.
export const ScansDashboard: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [scans, setScans] = useState<Scan[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);

    const [ipFilter, setIpFilter] = useState('');
    const [activeOnly, setActiveOnly] = useState(false);
    const [appliedIp, setAppliedIp] = useState('');

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            params.set('limit', String(PAGE_SIZE));
            params.set('offset', String((page - 1) * PAGE_SIZE));
            if (appliedIp) params.set('ip', appliedIp);
            if (activeOnly) params.set('active', 'true');

            const resp = await get<{ message: ScanListResponse }>(`admin/security/scans?${params.toString()}`);
            setScans(resp.message.scans);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load scans.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page, appliedIp, activeOnly]);

    useEffect(() => {
        void load();
    }, [load]);

    const applyFilters = () => {
        setPage(1);
        setAppliedIp(ipFilter.trim());
    };

    const columns: TableColumn<Scan>[] = [
        { key: 'ip', label: 'IP address' },
        { key: 'distinct_ports', label: 'Distinct ports' },
        { key: 'hit_count', label: 'Hits' },
        {
            key: 'geoip_info',
            label: 'Country / ISP',
            render: (_value, row) => <span className="text-xs">{formatGeoIPSummary(row.geoip_info)}</span>,
        },
        {
            key: 'sample_ports',
            label: 'Sample ports',
            render: value => <span className="font-mono text-xs">{String(value)}</span>,
        },
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
                <LinkAction to={`/admin/security/scans/${row.id}`} label="View details" />
            ),
        },
    ];

    return (
        <div>
            <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4">Port-scan episodes</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                An episode opens once an IP touches several distinct ports within a short window — a single
                probed port is noise, not a scan signature.
            </p>

            <div className="flex flex-wrap items-end gap-3 mb-4">
                <FormInput
                    id="scans-ip-filter"
                    label="IP address"
                    value={ipFilter}
                    onChange={setIpFilter}
                    placeholder="203.0.113.7"
                    className="w-56"
                />
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
                    <Table columns={columns} data={scans} rowKey={row => row.id} emptyText="No port-scan episodes recorded." />
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

export default ScansDashboard;
