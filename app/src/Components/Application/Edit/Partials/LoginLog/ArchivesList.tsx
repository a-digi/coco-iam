import React, { useCallback, useEffect, useState } from 'react';
import Table from '../../../../../Shared/Components/Table/Table';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import Pagination from '../../../../../Shared/Components/Pagination/Pagination';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { formatDate } from '../../../../../config/data/date/date';
import { ApplicationResource } from '../../../model/application';
import type { ArchiveSummary, ArchiveListResponse } from './types';

const PAGE_SIZE = 20;

const formatBytes = (n: number): string => {
    if (n <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
    const value = n / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
};

interface ArchivesListProps {
    applicationId: string;
    onSelectArchive: (archiveId: string) => void;
}

// ArchivesList lists this application's rotated-out <slug>_login.db
// generations — mirrors AdminLoginLogArchivesDashboard.tsx's shape,
// but "Browse" calls onSelectArchive instead of a route Link, since
// this section has no routes of its own (see LoginLogSection.tsx).
// See plan/login-audit-log/plan.md Step 13.
export const ArchivesList: React.FC<ArchivesListProps> = ({ applicationId, onSelectArchive }) => {
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

            const resp = await get<{ message: ArchiveListResponse }>(
                `applications/{${ApplicationResource}}/{id:${applicationId}}/login-log/archives?${params.toString()}`
            );
            setArchives(resp.message.archives);
            setTotal(resp.message.total);
        } catch (err: unknown) {
            errorMessage(err instanceof Error ? err.message : 'Failed to load archives.');
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage, page, applicationId]);

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
                <button
                    type="button"
                    onClick={() => onSelectArchive(row.id)}
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    Browse
                </button>
            ),
        },
    ];

    return (
        <div>
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

export default ArchivesList;
