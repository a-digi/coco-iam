import React, { useEffect, useMemo, useRef, useState } from 'react';
import ReactApexChart from 'react-apexcharts';
import type { ApexOptions } from 'apexcharts';
import { ObserveCard } from '../Partials/ObserveCard';
import { useObserveMetrics, type MetricRange } from '../hooks/useObserveMetrics';
import { useObserveRawData, type RawPage } from '../hooks/useObserveRawData';
import { WidgetSkeleton, WidgetError } from '../../Dashboard/Partials/WidgetState';
import { useTheme } from '../../../Layout/ThemeContextContext';
import type { Batch } from '../model/observe';
import { Tabs } from '../../../Shared/Components/Tabs/Tabs';
import { Tooltip } from '../../../Shared/Components/Tooltip/Tooltip';
import { ProgressLine, type ProgressSegmentColor } from '../../../Shared/Components/Progress/Line';

// ─── constants ───────────────────────────────────────────────────────────────

const RANGES: MetricRange[] = ['1h', '6h', '24h', '7d'];
const OS_KEY = '__os__';

const TARGET_POINTS: Record<MetricRange, number> = {
  '1h': 60,
  '6h': 72,
  '24h': 96,
  '7d': 168,
};

const OS_COLOR = '#f59e0b';
const PROCESS_PALETTE = ['#6366f1', '#10b981', '#06b6d4', '#8b5cf6', '#ef4444', '#f97316', '#ec4899'];

const RSS_TOOLTIP_TEXT = 'Resident Set Size — the amount of RAM this process currently holds in physical memory. Does not include memory swapped to disk or shared library pages counted in other processes.';

// ─── helpers ─────────────────────────────────────────────────────────────────

function cpuColor(pct: number): ProgressSegmentColor {
  if (pct > 80) return 'error';
  if (pct > 50) return 'pending';
  return 'info';
}

function memColor(pct: number): ProgressSegmentColor {
  if (pct > 85) return 'error';
  if (pct > 65) return 'pending';
  return 'success';
}

function mbFromBytes(bytes: number): number {
  return Math.round((bytes / 1024 / 1024) * 10) / 10;
}

function gbFromBytes(bytes: number): number {
  return Math.round((bytes / 1024 / 1024 / 1024) * 100) / 100;
}

function avgNum(values: number[]): number {
  if (!values.length) return 0;
  return values.reduce((a, b) => a + b, 0) / values.length;
}

function entityColor(key: string, processNames: string[]): string {
  if (key === OS_KEY) return OS_COLOR;
  const idx = processNames.indexOf(key);
  return PROCESS_PALETTE[idx % PROCESS_PALETTE.length];
}

// ─── aggregation ─────────────────────────────────────────────────────────────

function aggregateBatches(sorted: Batch[], targetPoints: number): Batch[] {
  if (sorted.length <= targetPoints) return sorted;
  const bucketSize = sorted.length / targetPoints;
  const result: Batch[] = [];
  for (let i = 0; i < targetPoints; i++) {
    const start = Math.floor(i * bucketSize);
    const end = Math.min(Math.floor((i + 1) * bucketSize), sorted.length);
    const bucket = sorted.slice(start, end);
    if (!bucket.length) continue;
    const mid = bucket[Math.floor(bucket.length / 2)];

    const osItems = bucket.map(b => b.payload.os).filter(Boolean) as NonNullable<typeof mid.payload.os>[];
    const avgOS = osItems.length > 0 ? {
      ...osItems[0],
      mem_used_bytes:      avgNum(osItems.map(o => o.mem_used_bytes)),
      mem_available_bytes: avgNum(osItems.map(o => o.mem_available_bytes)),
      cpu_usage_pct:       avgNum(osItems.map(o => o.cpu_usage_pct)),
      cpu_load_1m:         avgNum(osItems.map(o => o.cpu_load_1m)),
      cpu_load_5m:         avgNum(osItems.map(o => o.cpu_load_5m)),
    } : null;

    const procs = (b: Batch) => b.payload.processes ?? [];
    const names = [...new Set(bucket.flatMap(b => procs(b).map(p => p.name)))];
    const avgProcesses = names.map(name => {
      const samples = bucket.flatMap(b => procs(b)).filter(p => p.name === name);
      const found = samples.find(p => p.found) ?? samples[0];
      if (!found) return { name, found: false, cpu_pct: 0, rss_bytes: 0, threads: 0 };
      const active = samples.filter(p => p.found);
      return {
        ...found,
        cpu_pct:   active.length ? avgNum(active.map(p => p.cpu_pct)) : 0,
        rss_bytes: active.length ? Math.round(avgNum(active.map(p => p.rss_bytes))) : 0,
        threads:   active.length ? Math.round(avgNum(active.map(p => p.threads))) : 0,
      };
    });
    result.push({ ...mid, payload: { ...mid.payload, os: avgOS, processes: avgProcesses } });
  }
  return result;
}

// ─── sub-components ───────────────────────────────────────────────────────────

const RssLabel: React.FC<{ light?: boolean }> = ({ light = false }) => (
  <Tooltip content={RSS_TOOLTIP_TEXT}>
    <span className={`underline decoration-dotted cursor-help ${light ? 'decoration-white/40' : 'decoration-gray-400'}`}>
      RSS
    </span>
  </Tooltip>
);

// A single labelled metric row with an optional progress bar.
interface MetricRowProps {
  label: React.ReactNode;
  value: string;
  pct?: number; // 0–100 — renders a progress bar when provided
  barColor?: ProgressSegmentColor;
}

const MetricRow: React.FC<MetricRowProps> = ({ label, value, pct, barColor = 'info' }) => (
  <div className="flex items-center gap-3 py-2.5 border-b border-gray-100 dark:border-surface-700 last:border-0">
    <span className="text-xs text-gray-500 dark:text-gray-400 w-32 flex-shrink-0">{label}</span>
    <div className="flex-1 flex items-center">
      {pct !== undefined && (
        <ProgressLine
          height="h-1.5"
          className="w-32 max-w-full"
          segments={[
            { value: Math.max(0, Math.min(100, pct)), color: barColor },
            { value: Math.max(0, 100 - pct), color: 'neutral' },
          ]}
        />
      )}
    </div>
    <span className="text-xs font-mono font-semibold text-gray-800 dark:text-gray-100 w-28 text-right flex-shrink-0 tabular-nums">
      {value}
    </span>
  </div>
);

// Titled section card wrapping a set of MetricRows.
interface SnapshotSectionProps {
  dotColor: string;
  title: React.ReactNode;
  children: React.ReactNode;
}

const SnapshotSection: React.FC<SnapshotSectionProps> = ({ dotColor, title, children }) => (
  <div className="rounded-xl border border-gray-200 dark:border-surface-700 overflow-hidden">
    <div className="px-4 py-2.5 bg-gray-50 dark:bg-surface-700/60 border-b border-gray-200 dark:border-surface-700 flex items-center gap-2">
      <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ backgroundColor: dotColor }} />
      <span className="text-xs font-semibold text-gray-600 dark:text-gray-300 uppercase tracking-wide">{title}</span>
    </div>
    <div className="px-4 divide-y divide-gray-100 dark:divide-surface-700">
      {children}
    </div>
  </div>
);


interface AreaChartProps {
  labels: string[];
  series: { name: string; data: (number | null)[] }[];
  colors: string[];
  yFormatter: (v: number) => string;
  height?: number;
  showLegend?: boolean;
}

const AreaChart: React.FC<AreaChartProps> = ({
  labels, series, colors, yFormatter, height = 220, showLegend = true,
}) => {
  const { theme } = useTheme();

  const options: ApexOptions = {
    chart: { type: 'area', toolbar: { show: false }, background: 'transparent', animations: { enabled: false } },
    theme: { mode: theme },
    colors,
    stroke: { curve: 'straight', width: 1.5 },
    fill: { type: 'gradient', gradient: { opacityFrom: 0.15, opacityTo: 0.0 } },
    dataLabels: { enabled: false },
    xaxis: {
      categories: labels,
      labels: { show: labels.length <= 60, style: { fontSize: '10px' }, rotate: 0 },
      tickAmount: 6,
    },
    yaxis: { labels: { formatter: yFormatter, style: { fontSize: '11px' } }, min: 0 },
    tooltip: { y: { formatter: yFormatter } },
    legend: { show: showLegend, position: 'top', fontSize: '12px' },
    grid: { strokeDashArray: 4, borderColor: theme === 'dark' ? '#374151' : '#e5e7eb' },
  };

  if (!labels.length || !series.some(s => s.data.some(v => v !== null && v !== 0))) {
    return (
      <div className="flex items-center justify-center h-[160px] text-sm text-gray-400 italic">
        No data in this time range
      </div>
    );
  }

  return <ReactApexChart type="area" series={series} options={options} height={height} />;
};

interface RawDataTableProps {
  data: RawPage | null;
  hasOS: boolean;
  processNames: string[];
  onPageChange: (p: number) => void;
}

const RawDataTable: React.FC<RawDataTableProps> = ({ data, hasOS, processNames, onPageChange }) => {
  const entities = useMemo(
    () => [...(hasOS ? [OS_KEY] : []), ...processNames],
    [hasOS, processNames],
  );
  const [entity, setEntity] = useState<string>(() => entities[0] ?? '');

  useEffect(() => {
    if (entities.length > 0 && !entities.includes(entity)) {
      setEntity(entities[0]);
    }
  }, [entities, entity]);

  if (!data) return null;
  const { items, total, page, limit } = data;
  const totalPages = Math.ceil(total / limit);
  const isOS = entity === OS_KEY;

  return (
    <div className="space-y-3">

      {entities.length > 1 && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500 dark:text-gray-400 font-medium">Source:</span>
          <select
            value={entity}
            onChange={e => setEntity(e.target.value)}
            className="text-xs px-2 py-1.5 rounded-md border border-gray-200 dark:border-surface-600 bg-white dark:bg-surface-700 text-gray-700 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-indigo-500 cursor-pointer"
          >
            {hasOS && <option value={OS_KEY}>Host OS</option>}
            {processNames.map(name => <option key={name} value={name}>{name}</option>)}
          </select>
        </div>
      )}

      <div className="overflow-x-auto rounded-lg border border-gray-200 dark:border-surface-600">
        <table className="w-full text-xs text-left">
          <thead className="bg-gray-50 dark:bg-surface-700 text-gray-500 dark:text-gray-400 uppercase tracking-wide">
            <tr>
              <th className="px-3 py-2 font-semibold whitespace-nowrap">Timestamp</th>
              {isOS ? (
                <>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">CPU %</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">Load 1m</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">Load 5m</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">Load 15m</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">RAM Used</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">RAM Total</th>
                </>
              ) : (
                <>
                  <th className="px-3 py-2 font-semibold text-center whitespace-nowrap">Status</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">PID</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">CPU %</th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap"><RssLabel /></th>
                  <th className="px-3 py-2 font-semibold text-right whitespace-nowrap">Threads</th>
                  <th className="px-3 py-2 font-semibold whitespace-nowrap">Error</th>
                </>
              )}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-surface-600">
            {items.map(b => {
              const ts = (
                <td className="px-3 py-1.5 text-gray-600 dark:text-gray-300 whitespace-nowrap font-mono">
                  {new Date(b.captured_at).toLocaleString()}
                </td>
              );

              if (isOS) {
                const os = b.payload.os;
                return (
                  <tr key={b.id} className="hover:bg-gray-50 dark:hover:bg-surface-700/50 transition-colors">
                    {ts}
                    {os ? (
                      <>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{os.cpu_usage_pct.toFixed(1)}%</td>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{os.cpu_load_1m.toFixed(2)}</td>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{os.cpu_load_5m.toFixed(2)}</td>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{os.cpu_load_15m.toFixed(2)}</td>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{mbFromBytes(os.mem_used_bytes)} MB</td>
                        <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{gbFromBytes(os.mem_total_bytes)} GB</td>
                      </>
                    ) : (
                      <td colSpan={6} className="px-3 py-1.5 text-center text-gray-400 dark:text-gray-500 italic">no OS data</td>
                    )}
                  </tr>
                );
              }

              const proc = (b.payload.processes ?? []).find(p => p.name === entity);
              return (
                <tr key={b.id} className="hover:bg-gray-50 dark:hover:bg-surface-700/50 transition-colors">
                  {ts}
                  {proc ? (
                    <>
                      <td className="px-3 py-1.5 text-center">
                        {proc.found
                          ? <span className="inline-flex px-1.5 py-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400 font-medium">running</span>
                          : <span className="inline-flex px-1.5 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 font-medium">stopped</span>
                        }
                      </td>
                      <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{proc.pid ?? '—'}</td>
                      <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{proc.found ? `${proc.cpu_pct.toFixed(1)}%` : '—'}</td>
                      <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{proc.found ? `${mbFromBytes(proc.rss_bytes)} MB` : '—'}</td>
                      <td className="px-3 py-1.5 text-right font-mono text-gray-700 dark:text-gray-200">{proc.found ? proc.threads : '—'}</td>
                      <td className="px-3 py-1.5 font-mono text-red-500 dark:text-red-400">{proc.error ?? ''}</td>
                    </>
                  ) : (
                    <td colSpan={6} className="px-3 py-1.5 text-center text-gray-400 dark:text-gray-500 italic">—</td>
                  )}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
        <span>{total.toLocaleString()} entries · page {page + 1} of {Math.max(1, totalPages)}</span>
        {totalPages > 1 && (
          <div className="flex gap-1">
            <button type="button" disabled={page === 0} onClick={() => onPageChange(page - 1)}
              className="px-2 py-1 rounded bg-gray-100 dark:bg-surface-700 disabled:opacity-40 hover:bg-gray-200 dark:hover:bg-surface-600 transition-colors">
              ← Prev
            </button>
            <button type="button" disabled={page >= totalPages - 1} onClick={() => onPageChange(page + 1)}
              className="px-2 py-1 rounded bg-gray-100 dark:bg-surface-700 disabled:opacity-40 hover:bg-gray-200 dark:hover:bg-surface-600 transition-colors">
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  );
};

// ─── main component ───────────────────────────────────────────────────────────

interface MetricsSectionProps {
  agentId: string;
}

export const MetricsSection: React.FC<MetricsSectionProps> = ({ agentId }) => {
  const [range, setRange] = useState<MetricRange>('1h');
  const [rawTabVisited, setRawTabVisited] = useState(false);
  const [rawPage, setRawPage] = useState(0);

  // Main fetch — used by Latest Snapshot and Memory Usage tabs.
  const { batches, loading, error, reload } = useObserveMetrics(agentId, range);

  // Raw data fetch — paginated, deferred until the Raw Data tab is first opened.
  const { data: rawData, loading: rawLoading, error: rawError } = useObserveRawData(
    agentId, range, rawPage, rawTabVisited,
  );

  // Reset raw pagination when range or agent changes.
  useEffect(() => { setRawPage(0); }, [range, agentId]);

  const latest = batches[0] ?? null;
  const hasOS = useMemo(() => batches.some(b => b.payload.os), [batches]);

  const processNames = useMemo(() => {
    const names = new Set<string>();
    for (const b of batches) {
      for (const p of b.payload.processes ?? []) names.add(p.name);
    }
    return [...names];
  }, [batches]);

  const allEntities = useMemo(() => [
    ...(hasOS ? [OS_KEY] : []),
    ...processNames,
  ], [hasOS, processNames]);

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const knownEntities = useRef<Set<string>>(new Set());
  useEffect(() => {
    const toAdd = allEntities.filter(e => !knownEntities.current.has(e));
    if (toAdd.length) {
      setSelected(prev => new Set([...prev, ...toAdd]));
      toAdd.forEach(e => knownEntities.current.add(e));
    }
  }, [allEntities]);

  const toggleEntity = (key: string) =>
    setSelected(prev => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });

  const chartBatches = useMemo(() => {
    const sorted = [...batches].reverse();
    return aggregateBatches(sorted, TARGET_POINTS[range]);
  }, [batches, range]);

  const chartLabels = useMemo(
    () => chartBatches.map(b => new Date(b.captured_at).toLocaleTimeString()),
    [chartBatches],
  );

  const batchProc = (b: Batch, name: string) =>
    (b.payload.processes ?? []).find(p => p.name === name);

  const memorySeries = useMemo(() => {
    const series: { name: string; data: (number | null)[]; color: string }[] = [];
    if (selected.has(OS_KEY) && hasOS) {
      series.push({
        name: 'Host RAM',
        color: OS_COLOR,
        data: chartBatches.map(b => b.payload.os ? mbFromBytes(b.payload.os.mem_used_bytes) : null),
      });
    }
    for (const name of processNames) {
      if (selected.has(name)) {
        series.push({
          name: `${name} RSS`,
          color: entityColor(name, processNames),
          data: chartBatches.map(b => {
            const p = batchProc(b, name);
            return p?.found ? mbFromBytes(p.rss_bytes) : null;
          }),
        });
      }
    }
    return series;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chartBatches, selected, hasOS, processNames]);

  const cpuSeries = useMemo(() => {
    const series: { name: string; data: (number | null)[]; color: string }[] = [];
    if (selected.has(OS_KEY) && hasOS) {
      series.push({
        name: 'Host CPU',
        color: OS_COLOR,
        data: chartBatches.map(b => b.payload.os ? +b.payload.os.cpu_usage_pct.toFixed(1) : null),
      });
    }
    for (const name of processNames) {
      if (selected.has(name)) {
        series.push({
          name: `${name} CPU`,
          color: entityColor(name, processNames),
          data: chartBatches.map(b => {
            const p = batchProc(b, name);
            return p?.found ? +p.cpu_pct.toFixed(1) : null;
          }),
        });
      }
    }
    return series;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chartBatches, selected, hasOS, processNames]);

  const threadsSeries = useMemo(() => {
    return processNames
      .filter(name => selected.has(name))
      .map(name => ({
        name,
        color: entityColor(name, processNames),
        data: chartBatches.map(b => {
          const p = batchProc(b, name);
          return p?.found ? p.threads : null;
        }),
      }));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chartBatches, selected, processNames]);

  const selectedProcessNames = processNames.filter(n => selected.has(n));

  if (!agentId) {
    return (
      <div className="flex items-center justify-center h-32 text-sm text-gray-400 dark:text-gray-500 italic">
        Select an agent above to view metrics
      </div>
    );
  }

  // ─── tab content ───────────────────────────────────────────────────────────

  const osData = latest?.payload.os ?? null;
  const totalMemBytes = osData?.mem_total_bytes ?? 0;

  const snapshotContent = (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
      {/* Host OS */}
      {selected.has(OS_KEY) && osData && (
        <SnapshotSection dotColor={OS_COLOR} title="Host OS">
          <MetricRow
            label="RAM Used"
            value={`${mbFromBytes(osData.mem_used_bytes)} MB of ${gbFromBytes(totalMemBytes)} GB`}
            pct={totalMemBytes > 0 ? (osData.mem_used_bytes / totalMemBytes) * 100 : undefined}
            barColor={memColor(totalMemBytes > 0 ? (osData.mem_used_bytes / totalMemBytes) * 100 : 0)}
          />
          <MetricRow
            label="CPU"
            value={`${osData.cpu_usage_pct.toFixed(1)}%`}
            pct={osData.cpu_usage_pct}
            barColor={cpuColor(osData.cpu_usage_pct)}
          />
          <MetricRow
            label="Load avg"
            value={`${osData.cpu_load_1m.toFixed(2)} / ${osData.cpu_load_5m.toFixed(2)} / ${osData.cpu_load_15m.toFixed(2)}`}
          />
          {osData.disks.slice(0, 3).map(d => {
            const diskPct = d.total_bytes > 0 ? (d.used_bytes / d.total_bytes) * 100 : 0;
            return (
              <MetricRow
                key={d.mount}
                label={`Disk ${d.mount}`}
                value={`${gbFromBytes(d.used_bytes)} GB of ${gbFromBytes(d.total_bytes)} GB`}
                pct={diskPct}
                barColor={memColor(diskPct)}
              />
            );
          })}
        </SnapshotSection>
      )}

      {/* Per-process */}
      {selectedProcessNames.map(name => {
        const proc = (latest?.payload.processes ?? []).find(p => p.name === name);
        if (!proc) return null;
        const color = PROCESS_PALETTE[processNames.indexOf(name) % PROCESS_PALETTE.length];
        const rssPct = totalMemBytes > 0 ? (proc.rss_bytes / totalMemBytes) * 100 : undefined;

        return (
          <SnapshotSection
            key={name}
            dotColor={color}
            title={
              <>
                {name}
                {proc.pid
                  ? <span className="ml-2 font-mono font-normal normal-case text-gray-400 dark:text-gray-500 tracking-normal"> PID {proc.pid}</span>
                  : null}
              </>
            }
          >
            {!proc.found ? (
              <div className="py-3 text-sm text-amber-600 dark:text-amber-400 italic">
                Process not running
              </div>
            ) : (
              <>
                <MetricRow
                  label={<RssLabel />}
                  value={`${mbFromBytes(proc.rss_bytes)} MB${rssPct !== undefined ? ` (${rssPct.toFixed(1)}% of RAM)` : ''}`}
                  pct={rssPct}
                  barColor={memColor(rssPct ?? 0)}
                />
                <MetricRow
                  label="CPU"
                  value={`${proc.cpu_pct.toFixed(1)}%`}
                  pct={proc.cpu_pct}
                  barColor={cpuColor(proc.cpu_pct)}
                />
                <MetricRow
                  label="Threads"
                  value={String(proc.threads)}
                />
                {proc.error && (
                  <div className="py-2 text-xs text-red-500 dark:text-red-400 italic">
                    {proc.error}
                  </div>
                )}
              </>
            )}
          </SnapshotSection>
        );
      })}

      {!selected.has(OS_KEY) && selectedProcessNames.length === 0 && (
        <p className="text-sm text-gray-400 dark:text-gray-500 italic py-4">
          Select at least one entity in the filter above to view the snapshot.
        </p>
      )}
    </div>
  );

  const memoryContent = (
    <div className="space-y-4">
      {memorySeries.length > 0 && (
        <ObserveCard title="Memory Usage">
          <AreaChart
            labels={chartLabels}
            series={memorySeries.map(s => ({ name: s.name, data: s.data }))}
            colors={memorySeries.map(s => s.color)}
            yFormatter={(v: number) => `${v} MB`}
            height={240}
            showLegend={memorySeries.length > 1}
          />
        </ObserveCard>
      )}

      {cpuSeries.length > 0 && (
        <ObserveCard title="CPU Usage">
          <AreaChart
            labels={chartLabels}
            series={cpuSeries.map(s => ({ name: s.name, data: s.data }))}
            colors={cpuSeries.map(s => s.color)}
            yFormatter={(v: number) => `${v}%`}
            height={200}
            showLegend={cpuSeries.length > 1}
          />
        </ObserveCard>
      )}

      {threadsSeries.length > 0 && (
        <ObserveCard title="Threads">
          <AreaChart
            labels={chartLabels}
            series={threadsSeries.map(s => ({ name: s.name, data: s.data }))}
            colors={threadsSeries.map(s => s.color)}
            yFormatter={(v: number) => String(Math.round(v))}
            height={200}
            showLegend={threadsSeries.length > 1}
          />
        </ObserveCard>
      )}

      {memorySeries.length === 0 && cpuSeries.length === 0 && threadsSeries.length === 0 && (
        <p className="text-sm text-gray-400 dark:text-gray-500 italic py-8 text-center">
          Select at least one entity in the filter above to view charts.
        </p>
      )}
    </div>
  );

  const rawContent = rawLoading ? (
    <WidgetSkeleton className="h-[200px]" />
  ) : rawError ? (
    <WidgetError message={rawError} onRetry={() => setRawPage(p => p)} />
  ) : (
    <RawDataTable data={rawData} hasOS={hasOS} processNames={processNames} onPageChange={setRawPage} />
  );

  const rawTotal = rawData?.total;
  const tabs = [
    { id: 'snapshot', title: 'Latest Snapshot', content: snapshotContent },
    { id: 'memory',   title: 'Memory Usage',    content: memoryContent },
    {
      id: 'raw',
      title: `Raw Data${rawTotal != null ? ` (${rawTotal.toLocaleString()})` : ''}`,
      content: rawContent,
    },
  ];

  return (
    <div className="space-y-4">

      {/* Controls row: range + entity filter */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500 dark:text-gray-400 font-medium">Range:</span>
          {RANGES.map(r => (
            <button
              key={r}
              type="button"
              onClick={() => setRange(r)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors ${
                range === r
                  ? 'bg-indigo-600 text-white'
                  : 'bg-gray-100 dark:bg-surface-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-surface-600'
              }`}
            >
              {r}
            </button>
          ))}
        </div>

        {allEntities.length > 0 && !loading && (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs text-gray-500 dark:text-gray-400 font-medium">Show:</span>
            {allEntities.map(key => {
              const color = entityColor(key, processNames);
              const active = selected.has(key);
              const label = key === OS_KEY ? 'Host OS' : key;
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => toggleEntity(key)}
                  style={active ? { backgroundColor: color, borderColor: color } : { borderColor: color, color }}
                  className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border transition-colors ${
                    active ? 'text-white' : 'bg-transparent'
                  }`}
                >
                  <span
                    className="w-2 h-2 rounded-full flex-shrink-0"
                    style={{ backgroundColor: active ? 'rgba(255,255,255,0.7)' : color }}
                  />
                  {label}
                </button>
              );
            })}
          </div>
        )}
      </div>

      {loading && <WidgetSkeleton className="h-[300px]" />}
      {error && <WidgetError message={error} onRetry={reload} />}

      {!loading && !error && latest && (
        <Tabs
          items={tabs}
          initialActiveId="snapshot"
          onChange={id => { if (id === 'raw') setRawTabVisited(true); }}
        />
      )}

      {!loading && !error && batches.length === 0 && (
        <div className="flex items-center justify-center h-32 text-sm text-gray-400 dark:text-gray-500 italic">
          No data received yet for this agent in the selected range
        </div>
      )}
    </div>
  );
};
