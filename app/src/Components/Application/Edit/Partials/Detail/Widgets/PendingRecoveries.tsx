import React from 'react';
import { useWidget } from '../useWidget';
import { WidgetFrame } from '../WidgetFrame';

interface PendingCount {
  count: number;
}

export const PendingRecoveries: React.FC<{ applicationId: string }> = ({ applicationId }) => {
  const { data, loading, error, reload } = useWidget<PendingCount>(applicationId, 'pending-recoveries');
  const count = data?.count ?? 0;
  return (
    <WidgetFrame title="Pending Recoveries" loading={loading} error={error} onRetry={reload}>
      <div className="flex items-baseline gap-2">
        <span className={`text-3xl font-bold ${count > 0 ? 'text-amber-600' : 'text-gray-900 dark:text-gray-100'}`}>
          {count}
        </span>
        <span className="text-sm text-gray-500">active tokens</span>
      </div>
    </WidgetFrame>
  );
};

export default PendingRecoveries;
