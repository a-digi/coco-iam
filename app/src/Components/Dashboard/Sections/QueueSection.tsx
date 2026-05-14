import React from 'react';
import { DashboardCard } from '../Partials/DashboardCard';
import { QueueStatusChart } from '../Partials/QueueStatusChart';
import { TopQueuesList } from '../Partials/TopQueuesList';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { QueueResponse } from '../model/dashboard';

export const QueueSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<QueueResponse>('admin/dashboard/queue');

  return (
    <DashboardCard title="Queue Status" className="md:h-full flex flex-col">
      <div className="flex-1 overflow-y-auto">
        {loading && <WidgetSkeleton className="h-[320px]" />}
        {error && <WidgetError message={error} onRetry={reload} />}
        {!loading && !error && data && (
          <>
            <QueueStatusChart statuses={data.by_status} />
            <div className="mt-4 pt-4 border-t border-gray-100 dark:border-surface-700">
              <h4 className="text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1">
                Top Queues
              </h4>
              <TopQueuesList queues={data.top_queues} />
            </div>
          </>
        )}
      </div>
    </DashboardCard>
  );
};

export default QueueSection;
