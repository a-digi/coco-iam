import React, { useCallback, useEffect, useState } from 'react';
import ReactApexChart from 'react-apexcharts';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { WorkspaceResource, type WorkspaceStats } from '../model/workspace';
import { StatCard } from '../../../Shared/Components/Cards';
import { radialBarOptions, buildAppBarOptions } from './chartOptions';

interface WorkspaceDetailsPanelProps {
  workspaceId: string;
}

const STATS_ENDPOINT = (id: string) =>
  `workspaces/{${WorkspaceResource}}/{id:${id}}/stats`;

export const WorkspaceDetailsPanel: React.FC<WorkspaceDetailsPanelProps> = ({ workspaceId }) => {
  const [stats, setStats] = useState<WorkspaceStats | null>(null);
  const [loading, setLoading] = useState(true);

  const { get } = useHttpClient();
  const { errorMessage } = useSnackBar();

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const response = await get<{ message: WorkspaceStats }>(STATS_ENDPOINT(workspaceId));
      const data = response?.message ?? (response as unknown as WorkspaceStats);
      setStats(data);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load workspace statistics';
      errorMessage(msg);
    } finally {
      setLoading(false);
    }
  }, [workspaceId, get, errorMessage]);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <svg
          className="animate-spin h-6 w-6 text-indigo-600"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z" />
        </svg>
        <span className="ml-3 text-gray-500">Loading statistics...</span>
      </div>
    );
  }

  if (!stats) return null;

  const appTotal = stats.applications.total;
  const userTotal = stats.users.total;
  const activeAppPercent = appTotal > 0
    ? Math.round((stats.applications.active / appTotal) * 100)
    : 0;
  const hasAppData = appTotal > 0;
  const breakdown = stats.applications_breakdown ?? [];
  const hasBreakdown = breakdown.length > 0;

  return (
    <div className="space-y-6">
      {/* Stat cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatCard label="Total Applications" value={appTotal} color="blue" />
        <StatCard label="Active Applications" value={stats.applications.active} color="teal" />
        <StatCard label="Total Users with Access" value={userTotal} color="violet" />
        <StatCard label="Active Users" value={stats.users.active} color="amber" />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
        <div className="bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-xl p-4 shadow-sm">
          {hasAppData ? (
            <ReactApexChart
              type="radialBar"
              series={[activeAppPercent]}
              options={radialBarOptions}
              height={300}
            />
          ) : (
            <div className="flex flex-col items-center justify-center h-[300px] text-gray-400 text-sm">
              <span className="text-2xl mb-2">—</span>
              No applications yet
            </div>
          )}
        </div>
        <div className="bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-xl p-4 shadow-sm">
          {hasBreakdown ? (
            <ReactApexChart
              type="bar"
              series={[{ name: 'Users', data: breakdown.map(a => a.user_count) }]}
              options={buildAppBarOptions(breakdown)}
              height={300}
            />
          ) : (
            <div className="flex flex-col items-center justify-center h-[300px] text-gray-400 text-sm">
              <span className="text-2xl mb-2">—</span>
              No applications yet
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default WorkspaceDetailsPanel;
