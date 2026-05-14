import React from 'react';
import ReactApexChart from 'react-apexcharts';
import type { ApexOptions } from 'apexcharts';
import { useTheme } from '../../../Layout/ThemeContextContext';
import type { QueueStatusCount } from '../model/dashboard';

interface QueueStatusChartProps {
  statuses: QueueStatusCount[];
}

const STATUS_COLORS: Record<string, string> = {
  completed:     '#10b981',
  pending:       '#f59e0b',
  failed:        '#ef4444',
  in_progress:   '#6366f1',
  dead_lettered: '#9ca3af',
};

const DEFAULT_COLOR = '#6b7280';

export const QueueStatusChart: React.FC<QueueStatusChartProps> = ({ statuses }) => {
  const { theme } = useTheme();

  if (statuses.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-[220px] text-gray-400 dark:text-gray-500 text-[0.875rem]">
        <span className="text-2xl mb-2">—</span>
        No queue data
      </div>
    );
  }

  const total = statuses.reduce((sum, s) => sum + s.count, 0);
  const colors = statuses.map(s => STATUS_COLORS[s.status] ?? DEFAULT_COLOR);

  const options: ApexOptions = {
    chart: {
      type: 'donut',
      background: 'transparent',
    },
    theme: { mode: theme },
    colors,
    labels: statuses.map(s => s.status),
    plotOptions: {
      pie: {
        donut: {
          size: '65%',
          labels: {
            show: true,
            total: {
              show: true,
              label: 'Total',
              formatter: () => String(total),
              color: theme === 'dark' ? '#e5e7eb' : '#111827',
              fontSize: '18px',
              fontWeight: 700,
            },
            value: {
              color: theme === 'dark' ? '#d1d5db' : '#374151',
              fontSize: '14px',
              fontWeight: 600,
            },
          },
        },
      },
    },
    dataLabels: { enabled: false },
    legend: {
      position: 'bottom',
      fontSize: '12px',
    },
    tooltip: {
      y: { formatter: (val: number) => `${val} task${val !== 1 ? 's' : ''}` },
    },
  };

  return (
    <ReactApexChart
      type="donut"
      series={statuses.map(s => s.count)}
      options={options}
      height={220}
    />
  );
};

export default QueueStatusChart;
