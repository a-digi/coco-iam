import React, { useEffect, useState, useCallback } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../Shared/Components/Font/Title';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { Submit } from '../../../Shared/Components/Button';
import { AppScopes } from '../../../config/security/scopes';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { type QueueTask, QueueTaskSchema, QueueTaskResource } from '../model/queueTask';
import { StatusBadge } from '../Shared/StatusBadge';
import { formatDate } from '../../../config/data/date/date';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const TaskDetail: React.FC = () => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Queue', href: '/admin/queue' }, { label: 'Task' }]);
    const { taskId } = useParams<{ taskId: string }>();
    const { get, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [task, setTask] = useState<QueueTask | null>(null);
    const [payload, setPayload] = useState<string | null>(null);
    const [payloadError, setPayloadError] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [retrying, setRetrying] = useState(false);

    const fetchTask = useCallback(async () => {
        if (!taskId) return;
        setLoading(true);
        try {
            const response = await get<{ message: unknown }>(`admin/queue/{${QueueTaskResource}}/{id:${taskId}}`);
            const raw = response?.message || response;
            if (raw) {
                const mapped = mapObjects(QueueTaskSchema, [raw]) as unknown as QueueTask[];
                setTask(mapped[0] ?? null);
            } else {
                errorMessage('Task not found');
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to load task';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [taskId, get, errorMessage]);

    const fetchPayload = useCallback(async () => {
        if (!taskId) return;
        setPayload(null);
        setPayloadError(null);
        try {
            const raw = await get<unknown>(`admin/queue/tasks/{id:${taskId}}/payload`);
            setPayload(typeof raw === 'string' ? raw : JSON.stringify(raw));
        } catch (err: unknown) {
            let errorMsg = 'Failed to load payload';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            setPayloadError(errorMsg);
        }
    }, [taskId, get]);

    useEffect(() => {
        void fetchTask();
        void fetchPayload();
    }, [fetchTask, fetchPayload]);

    const handleRetry = useCallback(async () => {
        if (!taskId) return;
        setRetrying(true);
        try {
            await post(`admin/queue/retry/{id:${taskId}}`, {});
            successMessage('Task re-enqueued.');
            await fetchTask();
        } catch (err: unknown) {
            let errorMsg = 'Failed to retry task';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setRetrying(false);
        }
    }, [taskId, post, successMessage, errorMessage, fetchTask]);

    let prettyPayload = payload || '';
    if (prettyPayload) {
        try {
            prettyPayload = JSON.stringify(JSON.parse(prettyPayload), null, 2);
        } catch {
            // leave as-is if not valid JSON
        }
    }

    const canRetry = !!task && (task.status === 'dead_lettered' || task.status === 'failed');

    return (
        <div>
            <div className="mb-4">
                <Link
                    to={task ? `/admin/queue/${encodeURIComponent(task.queueName)}` : '/admin/queue'}
                    className="text-sm text-indigo-600 hover:text-indigo-800 dark:text-indigo-400"
                >
                    ← Back to queue
                </Link>
            </div>

            <Title>Queue Task</Title>

            {loading && !task ? (
                <div className="mt-6 text-sm text-gray-500">Loading...</div>
            ) : !task ? (
                <div className="mt-6 text-sm text-red-500">Task not found.</div>
            ) : (
                <div className="mt-6 space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <Field label="ID" value={<span className="font-mono text-xs break-all">{task.id}</span>} />
                        <Field label="Queue" value={<span className="font-mono text-sm">{task.queueName}</span>} />
                        <Field label="Status" value={<StatusBadge status={task.status} />} />
                        <Field label="Attempts" value={`${task.attempts} / ${task.maxAttempts}`} />
                        <Field label="Created" value={formatDate(task.createdAt)} />
                        <Field label="Updated" value={formatDate(task.updatedAt)} />
                        {task.nextAttemptAt && <Field label="Next attempt" value={formatDate(task.nextAttemptAt)} />}
                        {task.completedAt && <Field label="Completed" value={formatDate(task.completedAt)} />}
                    </div>

                    {task.lastError && (
                        <div>
                            <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Last error</div>
                            <div className="p-3 border border-red-200 dark:border-red-800 rounded bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-sm whitespace-pre-wrap">
                                {task.lastError}
                            </div>
                        </div>
                    )}

                    <div>
                        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">Payload</div>
                        {payloadError ? (
                            <div className="p-3 border border-red-200 dark:border-red-800 rounded bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 text-sm">
                                {payloadError}
                            </div>
                        ) : (
                            <pre className="p-3 border border-gray-200 dark:border-surface-800 rounded bg-gray-50 dark:bg-surface-900 text-xs overflow-x-auto">
                                {prettyPayload || '(empty)'}
                            </pre>
                        )}
                    </div>

                    {canRetry && (
                        <div>
                            <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminQueueWrite, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
                                <Submit
                                    type="button"
                                    onClick={handleRetry}
                                    loading={retrying}
                                    label="Retry task"
                                />
                            </ScopeBasedComponentAccess>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
};

const Field: React.FC<{ label: string; value: React.ReactNode }> = ({ label, value }) => (
    <div>
        <div className="text-xs uppercase tracking-wide text-gray-500 mb-1">{label}</div>
        <div className="text-sm text-gray-900 dark:text-gray-100">{value}</div>
    </div>
);

export default TaskDetail;
