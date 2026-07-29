import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import Table from '../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Pagination from '../../../../Shared/Components/Pagination/Pagination';
import { LinkAction } from '../../../../Shared/Components/Actions/Link';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../config/data/date/date';

interface ArchiveSummary {
    id: string;
    started_at: string;
    archived_at: string;
    row_count: number;
    size_bytes: number;
}

interface ArchiveListResponse {
    archives: ArchiveSummary[];
    total: number;
    limit: number;
    offset: number;
}

const PAGE_SIZE = 20;

const formatBytes = (n: number): string => {
    if (n <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
    const value = n / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
};

// AdminLoginLogArchivesDashboard lists rotated-out admin_login.db
// generations — nothing here is ever deleted, so each row is a
// permanent, still-browsable generation rather than a log destined to
// age out. Mirrors ArchivesDashboard.tsx's real-pagination pattern; no
// filters, same reasoning (archive count grows in the dozens over
// months, not per-request). "Browse" links straight to the
// attempts-browse view — there's no intermediate per-archive detail
// page/route here (unlike ip-attacks' ArchiveDetail), since the
// backend never built a single-archive-detail endpoint for this
// domain and the 4 metadata fields already shown in this list would
// just be re-displayed a second time for no benefit. See
// plan/login-audit-log/plan.md Step 11.
export const AdminLoginLogArchivesDashboard: React.FC = () => {
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [archives, setArchives] = useState<ArchiveSummary[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            params.set('limit', String(PAGE_SIZE));
            params.set('offset', String((page - 1) * PAGE_SIZE));

            const resp = await get<{ message: ArchiveListResponse }>(`admin/security/login-log/admin/archives?${params.toString()}`);
            setArchives(resp.message.archives);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load archives.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page]);

    useEffect(() => {
        void load();
    }, [load]);

    const columns: TableColumn<ArchiveSummary>[] = [
        {
            key: 'started_at',
            label: 'Started',
            render: value => formatDate(String(value)),
        },
        {
            key: 'archived_at',
            label: 'Archived',
            render: value => formatDate(String(value)),
        },
        {
            key: 'row_count',
            label: 'Rows',
            render: value => Number(value).toLocaleString(),
        },
        {
            key: 'size_bytes',
            label: 'Size',
            render: value => formatBytes(Number(value)),
        },
        {
            key: 'actions',
            label: '',
            render: (_value, row) => (
                <LinkAction to={`/admin/security/login-log/archives/${row.id}/attempts`} label="Browse" />
            ),
        },
    ];

    return (
        <div>
            <div className="mb-4">
                <Link
                    to="/admin/security/login-log"
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to login log
                </Link>
            </div>

            <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100 mb-4">admin_login.db archives</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                Rotated-out generations of the admin login-attempt database, kept indefinitely and still browsable.
            </p>

            {loading ? (
                <div className="text-sm text-gray-500 dark:text-gray-400">Loading…</div>
            ) : (
                <>
                    <Table columns={columns} data={archives} rowKey={row => row.id} emptyText="No archives yet — nothing has been rotated out." />
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

export default AdminLoginLogArchivesDashboard;
