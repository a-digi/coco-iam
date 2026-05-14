import React from 'react';
import { StatCard } from '../../../Shared/Components/Cards';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { DashboardStats } from '../model/dashboard';

export const StatsSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<DashboardStats>('admin/dashboard/stats');

  const carouselCls = 'flex overflow-x-auto snap-x snap-mandatory gap-4 pb-2 md:grid md:grid-cols-3 lg:grid-cols-5 md:overflow-x-visible md:snap-none';
  const itemCls = 'snap-start shrink-0 w-[44vw] sm:w-[32vw] md:w-auto md:shrink flex flex-col';

  if (loading) {
    return (
      <div className={carouselCls}>
        {[...Array(5)].map((_, i) => (
          <div key={i} className={itemCls}>
            <WidgetSkeleton className="h-[110px]" />
          </div>
        ))}
      </div>
    );
  }

  if (error || !data) return <WidgetError message={error ?? 'No data'} onRetry={reload} />;

  return (
    <div className={carouselCls}>
      <div className={itemCls}>
        <StatCard label="Admin Users" value={data.total_admin_users} color="blue" />
      </div>
      <div className={itemCls}>
        <StatCard label="Org Users" value={data.total_org_users} color="teal" />
      </div>
      <div className={itemCls}>
        <StatCard label="Organizations" value={data.total_organizations} color="violet" />
      </div>
      <div className={itemCls}>
        <StatCard label="Workspaces" value={data.total_workspaces} color="amber" />
      </div>
      <div className={itemCls}>
        <StatCard label="Applications" value={data.total_applications} color="indigo" />
      </div>
    </div>
  );
};

export default StatsSection;
