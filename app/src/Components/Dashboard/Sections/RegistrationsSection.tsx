import React from 'react';
import { DashboardCard } from '../Partials/DashboardCard';
import { RegistrationsChart } from '../Partials/RegistrationsChart';
import { useWidgetData } from '../Partials/useWidgetData';
import { WidgetSkeleton, WidgetError } from '../Partials/WidgetState';
import type { OrgRegistrations } from '../model/dashboard';

export const RegistrationsSection: React.FC = () => {
  const { data, loading, error, reload } = useWidgetData<OrgRegistrations>(
    'admin/dashboard/registrations'
  );

  return (
    <DashboardCard title="Organization Registrations" className="md:h-full flex flex-col">
      <div className="flex-1 flex flex-col justify-center">
        {loading && <WidgetSkeleton className="h-[280px]" />}
        {error && <WidgetError message={error} onRetry={reload} />}
        {!loading && !error && data && <RegistrationsChart registrations={data} />}
      </div>
    </DashboardCard>
  );
};

export default RegistrationsSection;
