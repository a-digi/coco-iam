import React from 'react';
import { DashboardCard } from '../Partials/DashboardCard';
import { TopOrgsTable } from '../Partials/TopOrgsTable';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { OrgUserCount } from '../model/dashboard';

export const TopOrgsSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<OrgUserCount[]>('admin/dashboard/top-orgs');

  return (
    <DashboardCard title="Top Organizations by Users">
      {loading && <WidgetSkeleton className="h-[220px]" />}
      {error && <WidgetError message={error} onRetry={reload} />}
      {!loading && !error && data && <TopOrgsTable orgs={data} />}
    </DashboardCard>
  );
};

export default TopOrgsSection;
