import React from 'react';
import type { QueueTaskStatus } from '../model/queueTask';

interface StatusBadgeProps {
    status: QueueTaskStatus | string;
}

const STYLE: Record<string, string> = {
    pending: 'bg-gray-100 text-gray-700 border-gray-300 dark:bg-gray-800 dark:text-gray-300 dark:border-gray-700',
    in_progress: 'bg-blue-100 text-blue-700 border-blue-300 dark:bg-blue-900/40 dark:text-blue-300 dark:border-blue-800',
    completed: 'bg-green-100 text-green-700 border-green-300 dark:bg-green-900/40 dark:text-green-300 dark:border-green-800',
    failed: 'bg-orange-100 text-orange-700 border-orange-300 dark:bg-orange-900/40 dark:text-orange-300 dark:border-orange-800',
    dead_lettered: 'bg-red-100 text-red-700 border-red-300 dark:bg-red-900/40 dark:text-red-300 dark:border-red-800',
};

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status }) => {
    const cls = STYLE[status] || 'bg-gray-100 text-gray-700 border-gray-300';
    const label = status.replace('_', ' ');
    return (
        <span className={`inline-flex items-center px-2 py-0.5 text-xs font-medium rounded border ${cls}`}>
            {label}
        </span>
    );
};

export default StatusBadge;
