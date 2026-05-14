import React from 'react';
import { Link } from 'react-router-dom';
import { ProgressLine } from '../../../Shared/Components/Progress';
import type { QueueBreakdown } from '../model/dashboard';

interface TopQueuesListProps {
  queues: QueueBreakdown[];
}

export const TopQueuesList: React.FC<TopQueuesListProps> = ({ queues }) => {
  if (queues.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-6 text-gray-400 dark:text-gray-500 text-[0.875rem]">
        <span className="text-2xl mb-1">—</span>
        No queue activity
      </div>
    );
  }

  return (
    <div className="space-y-4 mt-4">
      {queues.map(q => (
        <div key={q.name} className="space-y-1.5">
          <div className="flex items-center justify-between text-[0.8125rem]">
            <Link
              to={`/admin/queue/${encodeURIComponent(q.name)}`}
              className="font-semibold text-gray-800 dark:text-gray-200 hover:text-indigo-600 dark:hover:text-indigo-400 truncate"
            >
              {q.name}
            </Link>
            <span className="text-[0.75rem] text-gray-500 dark:text-gray-400 ml-2 shrink-0">
              {q.total} task{q.total !== 1 ? 's' : ''}
            </span>
          </div>

          <ProgressLine
            segments={[
              { color: 'success', value: q.success, label: `Success: ${q.success}` },
              { color: 'pending', value: q.pending, label: `Pending: ${q.pending}` },
              { color: 'error',   value: q.failed,  label: `Failed: ${q.failed}` },
            ]}
          />

          <div className="flex items-center gap-3 text-[0.6875rem] text-gray-500 dark:text-gray-400">
            <span className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-emerald-500" />
              {q.success} success
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-amber-400" />
              {q.pending} pending
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block w-2 h-2 rounded-full bg-red-500" />
              {q.failed} failed
            </span>
          </div>
        </div>
      ))}
    </div>
  );
};

export default TopQueuesList;
