import React, { useState } from 'react';
import ReactApexChart from 'react-apexcharts';
import type { ApexOptions } from 'apexcharts';
import { useTheme } from '../../../Layout/ThemeContextContext';
import type { OrgRegistrations, RegistrationPoint } from '../model/dashboard';

type Mode = 'weekday' | 'month' | 'year';

interface RegistrationsChartProps {
  registrations: OrgRegistrations;
}

const MODE_LABELS: Record<Mode, string> = {
  weekday: 'Weekday',
  month:   'Month',
  year:    'Year',
};

const pointsFor = (reg: OrgRegistrations, mode: Mode): RegistrationPoint[] => {
  if (mode === 'weekday') return reg.by_weekday;
  if (mode === 'month')   return reg.by_month;
  return reg.by_year;
};

export const RegistrationsChart: React.FC<RegistrationsChartProps> = ({ registrations }) => {
  const { theme } = useTheme();
  const [mode, setMode] = useState<Mode>('weekday');

  const points = pointsFor(registrations, mode);
  const isEmpty = points.length === 0 || points.every(p => p.count === 0);

  const options: ApexOptions = {
    chart: {
      type: 'bar',
      toolbar: { show: false },
      background: 'transparent',
    },
    theme: { mode: theme },
    colors: ['#6366f1'],
    plotOptions: {
      bar: {
        columnWidth: '55%',
        borderRadius: 4,
        borderRadiusApplication: 'end',
      },
    },
    dataLabels: { enabled: false },
    xaxis: {
      categories: points.map(p => p.label),
      labels: { style: { fontSize: '11px' } },
    },
    yaxis: {
      labels: {
        formatter: (val: number) => String(Math.round(val)),
        style: { fontSize: '11px' },
      },
      min: 0,
      forceNiceScale: true,
    },
    tooltip: {
      y: { formatter: (val: number) => `${val} org${val !== 1 ? 's' : ''}` },
    },
    grid: { strokeDashArray: 4, borderColor: theme === 'dark' ? '#374151' : '#e5e7eb' },
  };

  return (
    <div>
      <div className="flex items-center justify-end gap-1 mb-3">
        {(['weekday', 'month', 'year'] as Mode[]).map(m => (
          <button
            key={m}
            type="button"
            onClick={() => setMode(m)}
            className={`px-3 py-1 text-[0.75rem] font-medium rounded-md transition-colors ${
              mode === m
                ? 'bg-indigo-600 text-white'
                : 'bg-gray-100 dark:bg-surface-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-surface-600'
            }`}
          >
            {MODE_LABELS[m]}
          </button>
        ))}
      </div>

      {isEmpty ? (
        <div className="flex flex-col items-center justify-center h-[280px] text-gray-400 dark:text-gray-500 text-[0.875rem]">
          <span className="text-2xl mb-2">—</span>
          No registration data
        </div>
      ) : (
        <ReactApexChart
          type="bar"
          series={[{ name: 'Organizations', data: points.map(p => p.count) }]}
          options={options}
          height={280}
        />
      )}
    </div>
  );
};

export default RegistrationsChart;
