import type { ApexOptions } from 'apexcharts';
import type { ApplicationBreakdown } from '../model/workspace';

export const radialBarOptions: ApexOptions = {
    chart: { type: 'radialBar', toolbar: { show: false } },
    colors: ['#d946ef'],
    plotOptions: {
        radialBar: {
            hollow: { size: '62%', background: 'transparent' },
            track: { background: '#f3f4f6', strokeWidth: '100%' },
            dataLabels: {
                name: {
                    show: true,
                    fontSize: '13px',
                    color: '#9ca3af',
                    offsetY: 22,
                },
                value: {
                    show: true,
                    fontSize: '30px',
                    fontWeight: 700,
                    color: '#111827',
                    offsetY: -8,
                    formatter: (val: number) => `${Math.round(val)}%`,
                },
            },
        },
    },
    labels: ['Active'],
    title: {
        text: 'Applications — Active Rate',
        align: 'center',
        style: { fontSize: '13px', fontWeight: '600', color: '#6b7280' },
    },
};

export const buildAppBarOptions = (breakdown: ApplicationBreakdown[]): ApexOptions => ({
    chart: { type: 'bar', toolbar: { show: false } },
    colors: ['#06b6d4'],
    plotOptions: {
        bar: {
            horizontal: false,
            columnWidth: '45%',
            borderRadius: 6,
            borderRadiusApplication: 'end',
        },
    },
    dataLabels: { enabled: true, style: { fontSize: '12px', fontWeight: 700, colors: ['#fff'] } },
    xaxis: {
        categories: breakdown.map(a => a.title),
        labels: { style: { fontSize: '11px' }, trim: true },
    },
    yaxis: {
        title: { text: 'Users', style: { fontSize: '11px', color: '#9ca3af' } },
        labels: { formatter: (val: number) => String(Math.round(val)) },
    },
    tooltip: {
        custom: ({ dataPointIndex }: { dataPointIndex: number }) => {
            const app = breakdown[dataPointIndex];
            if (!app) return '';
            const scopeTags = app.top_scopes.length > 0
                ? app.top_scopes
                    .map(s => `<span style="display:inline-block;background:#e0f2fe;color:#0369a1;border-radius:4px;padding:1px 7px;font-size:11px;margin:2px 2px 0 0">${s}</span>`)
                    .join('')
                : '<span style="color:#9ca3af;font-size:11px">No scopes configured</span>';
            return `<div style="padding:10px 14px;min-width:180px">
                <div style="font-weight:700;font-size:13px;margin-bottom:4px">${app.title}</div>
                <div style="color:#6b7280;font-size:12px;margin-bottom:6px">${app.user_count} user${app.user_count !== 1 ? 's' : ''}</div>
                <div style="font-size:11px;font-weight:600;color:#9ca3af;text-transform:uppercase;letter-spacing:.05em;margin-bottom:4px">Top scopes</div>
                <div>${scopeTags}</div>
            </div>`;
        },
    },
    title: {
        text: 'Users per Application',
        align: 'center',
        style: { fontSize: '13px', fontWeight: '600', color: '#6b7280' },
    },
});
