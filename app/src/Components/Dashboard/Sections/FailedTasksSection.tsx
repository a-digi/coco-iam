import React from 'react';
import { DashboardCard } from '../Partials/DashboardCard';
import { RecentFailedTasksTable } from '../Partials/RecentFailedTasksTable';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { RecentTask } from '../model/dashboard';

export const FailedTasksSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<RecentTask[]>('admin/dashboard/failed-tasks');

  return (
    <DashboardCard title="Recent Failed Tasks">
      {loading && <WidgetSkeleton className="h-[220px]" />}
      {error && <WidgetError message={error} onRetry={reload} />}
      {!loading && !error && data && <RecentFailedTasksTable tasks={data} />}
    </DashboardCard>
  );
};

export default FailedTasksSection;
