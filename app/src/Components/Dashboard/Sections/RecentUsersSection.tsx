import React from 'react';
import { DashboardCard } from '../Partials/DashboardCard';
import { RecentUsersTable } from '../Partials/RecentUsersTable';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { RecentUser } from '../model/dashboard';

export const RecentUsersSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<RecentUser[]>('admin/dashboard/recent-users');

  return (
    <DashboardCard title="Recent Users">
      {loading && <WidgetSkeleton className="h-[220px]" />}
      {error && <WidgetError message={error} onRetry={reload} />}
      {!loading && !error && data && <RecentUsersTable users={data} />}
    </DashboardCard>
  );
};

export default RecentUsersSection;
