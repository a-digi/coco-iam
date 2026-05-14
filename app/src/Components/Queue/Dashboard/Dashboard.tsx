import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../Shared/Components/Font/Title';
import { Submit } from '../../../Shared/Components/Button';
import TableView from '../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../Shared/Components/Table/Table';
import NoEntriesFound from '../../../Shared/Components/NoEntries/NoEntriesFound';
import { LinkAction } from '../../../Shared/Components/Actions/Link';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';
import { type QueueWithCounts, type QueueWithCountsRaw, toQueueWithCounts } from '../model/queue';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const QueueDashboard: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Queue' }]);
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();
    const navigate = useNavigate();
    const [queues, setQueues] = useState<QueueWithCounts[]>([]);
    const [loading, setLoading] = useState(true);

    const fetchQueues = React.useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: QueueWithCountsRaw[] }>('admin/queue/queues');
            const data = response?.message || [];
            setQueues(Array.isArray(data) ? data.map(toQueueWithCounts) : []);
        } catch (err: unknown) {
            let errorMsg = 'Failed to load queues';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void fetchQueues();
    }, [fetchQueues]);

    const columns = useMemo<TableColumn<QueueWithCounts>[]>(() => [
        {
            key: 'id',
            label: 'ID',
            render: (value) => <span className="font-mono text-xs" title={String(value)}>{String(value).slice(0, 8)}…</span>,
        },
        {
            key: 'name',
            label: 'Name',
            render: (value) => <span className="font-mono text-sm">{String(value)}</span>,
        },
        { key: 'description', label: 'Description' },
        {
            key: 'consumers',
            label: 'Consumers',
            render: (_value, row) => {
                if (row.consumers.length === 0) {
                    return <span className="text-xs text-gray-400 italic">none</span>;
                }
                return (
                    <div>
                        <span className="font-medium">{row.consumers.length}</span>
                        <div className="text-xs text-gray-500 font-mono">
                            {row.consumers.map(c => c.id.slice(0, 8)).join(', ')}
                        </div>
                    </div>
                );
            },
        },
        { key: 'counts.pending', label: 'Pending', render: (_v, row) => row.counts.pending },
        { key: 'counts.inProgress', label: 'In progress', render: (_v, row) => row.counts.inProgress },
        { key: 'counts.completed', label: 'Completed', render: (_v, row) => row.counts.completed },
        { key: 'counts.failed', label: 'Failed', render: (_v, row) => row.counts.failed },
        {
            key: 'counts.deadLettered',
            label: 'Dead-lettered',
            render: (_v, row) => (
                row.counts.deadLettered > 0
                    ? <span className="text-red-600 font-medium">{row.counts.deadLettered}</span>
                    : <span>{row.counts.deadLettered}</span>
            ),
        },
        {
            key: 'actions',
            label: 'View',
            render: (_value, row) => (
                <LinkAction to={`/admin/queue/${encodeURIComponent(row.name)}`} label="View" />
            ),
        },
    ], []);

    return (
        <div>
            <div className="flex justify-between items-center mb-6">
                <Title>Async Queues</Title>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminQueueWrite, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
                    <Submit
                        type="button"
                        onClick={() => navigate('/admin/queue/create')}
                        label="Create Queue"
                    />
                </ScopeBasedComponentAccess>
            </div>

            {loading && queues.length === 0 ? (
                <div>Loading queues...</div>
            ) : queues.length === 0 ? (
                <NoEntriesFound
                    title="No Queues"
                    message="No queues have been registered or created yet. Create one to get started."
                />
            ) : (
                <TableView
                    columns={columns}
                    data={queues}
                    total={queues.length}
                    page={1}
                    pageSize={queues.length || 1}
                    onPageChange={() => { }}
                    onFilterChange={() => { }}
                />
            )}
        </div>
    );
};

export default QueueDashboard;
