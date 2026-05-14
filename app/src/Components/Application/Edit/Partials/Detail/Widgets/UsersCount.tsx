import React from 'react';
import { useWidget } from '../useWidget';
import { WidgetFrame } from '../WidgetFrame';

interface CountPair {
  total: number;
  active: number;
}

export const UsersCount: React.FC<{ applicationId: string }> = ({ applicationId }) => {
  const { data, loading, error, reload } = useWidget<CountPair>(applicationId, 'users-count');
  return (
    <WidgetFrame title="Users on ACL" loading={loading} error={error} onRetry={reload}>
      <div className="flex items-baseline gap-2">
        <span className="text-3xl font-bold text-gray-900 dark:text-gray-100">{data?.total ?? 0}</span>
        <span className="text-sm text-gray-500">{data?.active ?? 0} active</span>
      </div>
    </WidgetFrame>
  );
};

export default UsersCount;
