import React from 'react';
import { Link } from 'react-router-dom';
import { formatDateOnly } from '../../../config/data/date/date';
import type { RecentTask } from '../model/dashboard';

interface RecentFailedTasksTableProps {
  tasks: RecentTask[];
}

export const RecentFailedTasksTable: React.FC<RecentFailedTasksTableProps> = ({ tasks }) => {
  if (tasks.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-6 text-emerald-600 dark:text-emerald-400 text-[0.875rem]">
        <svg
          className="w-8 h-8 mb-2"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={1.75}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span className="font-medium">No failed tasks</span>
        <span className="text-[0.75rem] text-gray-400 dark:text-gray-500 mt-0.5">All clear</span>
      </div>
    );
  }

  return (
    <table className="w-full text-[0.8125rem] mt-3">
      <thead>
        <tr className="border-b border-gray-100 dark:border-surface-700">
          <th className="text-left py-2 pr-3 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Queue
          </th>
          <th className="text-left py-2 pr-3 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Error
          </th>
          <th className="text-left py-2 font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide text-[0.6875rem]">
            Created
          </th>
        </tr>
      </thead>
      <tbody>
        {tasks.map(task => (
          <tr
            key={task.id}
            className="border-b border-gray-50 dark:border-surface-700/50 hover:bg-gray-50 dark:hover:bg-surface-700/30 transition-colors"
          >
            <td className="py-2.5 pr-3">
              <Link
                to={`/admin/queue/tasks/${task.id}`}
                className="text-red-600 dark:text-red-400 hover:underline font-medium"
              >
                {task.queue_name || '—'}
              </Link>
            </td>
            <td className="py-2.5 pr-3 text-gray-500 dark:text-gray-400 max-w-[200px] truncate" title={task.last_error}>
              {task.last_error || '—'}
            </td>
            <td className="py-2.5 text-gray-500 dark:text-gray-400 whitespace-nowrap">
              {formatDateOnly(task.created_at)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
};

export default RecentFailedTasksTable;
