import React, { useEffect, useState, useMemo } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../Shared/Components/Font/Title';
import TableView from '../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../Shared/Components/Table/Table';
import NoEntriesFound from '../../../Shared/Components/NoEntries/NoEntriesFound';
import { LinkAction } from '../../../Shared/Components/Actions/Link';
import Dropdown, { type DropdownOption } from '../../../Shared/Components/Dropdown/Dropdown';
import { formatDate } from '../../../config/data/date/date';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { buildFilterQueryString } from '../../../config/data/resource/filters';
import { type QueueTask, QueueTaskSchema, QueueTaskResource, type QueueTaskStatus } from '../model/queueTask';
import { type QueueWithCounts, type QueueWithCountsRaw, toQueueWithCounts } from '../model/queue';
import { StatusBadge } from '../Shared/StatusBadge';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const STATUS_OPTIONS: DropdownOption[] = [
    { label: 'All statuses', value: 'all' },
    { label: 'Pending', value: 'pending' },
    { label: 'In progress', value: 'in_progress' },
    { label: 'Completed', value: 'completed' },
    { label: 'Failed', value: 'failed' },
    { label: 'Dead-lettered', value: 'dead_lettered' },
];

const QueueDetail: React.FC = () => {
    const { queueName } = useParams<{ queueName: string }>();
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Queue', href: '/admin/queue' }, { label: queueName ?? '' }]);
    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const [tasks, setTasks] = useState<QueueTask[]>([]);
    const [queue, setQueue] = useState<QueueWithCounts | null>(null);
    const [loading, setLoading] = useState(true);
    const [status, setStatus] = useState<string>('all');

    const fetchQueue = React.useCallback(async () => {
        if (!queueName) return;
        try {
            const response = await get<{ message?: QueueWithCountsRaw[] }>('admin/queue/queues');
            const data = response?.message || [];
            if (Array.isArray(data)) {
                const match = data.find(q => q.name === queueName);
                setQueue(match ? toQueueWithCounts(match) : null);
            }
        } catch {
            // Non-critical; header just won't show metadata.
        }
    }, [queueName, get]);

    const fetchTasks = React.useCallback(async () => {
        if (!queueName) return;
        setLoading(true);
        try {
            const filters = [{ field: 'queue_name', operator: 'exact' as const, value: queueName }];
            if (status !== 'all') filters.push({ field: 'status', operator: 'exact' as const, value: status });
            const qs = buildFilterQueryString(filters);
            const response = await get<{ message?: unknown }>(`admin/queue/{${QueueTaskResource}}?${qs}`);
            const data = response?.message || [];
            if (Array.isArray(data)) {
                const mapped = mapObjects(QueueTaskSchema, data) as unknown as QueueTask[];
                setTasks(mapped);
            } else {
                setTasks([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to load queue tasks';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [queueName, status, get, errorMessage]);

    useEffect(() => {
        void fetchTasks();
        void fetchQueue();
    }, [fetchTasks, fetchQueue]);

    const columns = useMemo<TableColumn<QueueTask>[]>(() => [
        {
            key: 'id',
            label: 'Task ID',
            render: (value) => <span className="font-mono text-xs">{String(value).slice(0, 8)}…</span>
        },
        {
            key: 'status',
            label: 'Status',
            render: (value) => <StatusBadge status={value as QueueTaskStatus} />
        },
        {
            key: 'attempts',
            label: 'Attempts',
            render: (_value, row) => `${row.attempts} / ${row.maxAttempts}`
        },
        {
            key: 'createdAt',
            label: 'Created',
            render: (value) => formatDate(value as string)
        },
        {
            key: 'updatedAt',
            label: 'Updated',
            render: (value) => formatDate(value as string)
        },
        {
            key: 'id',
            label: 'View',
            render: (_value, row) => (
                <LinkAction to={`/admin/queue/tasks/${row.id}`} label="View" />
            ),
        },
    ], []);

    if (!queueName) return <div>Missing queue name.</div>;

    return (
        <div>
            <div className="mb-4">
                <Link to="/admin/queue" className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400">
                    ← All queues
                </Link>
            </div>

            {queue && (
                <div className="mb-6 p-4 rounded-lg bg-gradient-to-r from-indigo-50 to-white dark:from-surface-800 dark:to-surface-900 border border-indigo-100 dark:border-surface-800">
                    <div className="text-xs uppercase tracking-wide text-indigo-600 dark:text-indigo-400 mb-1">Queue</div>
                    <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 font-mono">{queue.name}</h2>
                    <div className="text-xs text-gray-500 mt-1 font-mono">ID: {queue.id}</div>
                    {queue.description && (
                        <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">{queue.description}</p>
                    )}
                    <div className="mt-3 text-xs text-gray-500">
                        <span className="font-medium text-gray-700 dark:text-gray-300">Consumers: </span>
                        {queue.consumers.length === 0 ? (
                            <span className="italic">none attached</span>
                        ) : (
                            <span className="font-mono">
                                {queue.consumers.length} ({queue.consumers.map(c => c.id.slice(0, 8)).join(', ')})
                            </span>
                        )}
                    </div>
                </div>
            )}

            <div className="flex justify-between items-center mb-6 flex-wrap gap-3">
                <Title>Tasks{!queue && <> — <span className="font-mono">{queueName}</span></>}</Title>
                <div className="w-full sm:w-64">
                    <Dropdown
                        options={STATUS_OPTIONS}
                        value={status}
                        onChange={(opt) => setStatus(String(opt.value))}
                        className="w-full"
                    />
                </div>
            </div>

            {loading && tasks.length === 0 ? (
                <div>Loading tasks...</div>
            ) : tasks.length === 0 ? (
                <NoEntriesFound
                    title="No Tasks"
                    message="No tasks matching the current filter."
                />
            ) : (
                <TableView
                    columns={columns}
                    data={tasks}
                    total={tasks.length}
                    page={1}
                    pageSize={tasks.length || 1}
                    onPageChange={() => { }}
                    onFilterChange={() => { }}
                />
            )}
        </div>
    );
};

export default QueueDetail;
