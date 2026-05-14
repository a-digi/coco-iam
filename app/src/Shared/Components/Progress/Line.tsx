import React from 'react';

export type ProgressSegmentColor = 'success' | 'pending' | 'error' | 'info' | 'neutral';

export interface ProgressSegment {
  value: number;
  color: ProgressSegmentColor;
  label?: string;
}

export interface ProgressLineProps {
  segments: ProgressSegment[];
  height?: string;
  rounded?: boolean;
  className?: string;
  showTooltip?: boolean;
}

const SEGMENT_CLASSES: Record<ProgressSegmentColor, string> = {
  success: 'bg-emerald-500',
  pending: 'bg-amber-400',
  error:   'bg-red-500',
  info:    'bg-indigo-500',
  neutral: 'bg-gray-300 dark:bg-surface-600',
};

export const ProgressLine: React.FC<ProgressLineProps> = ({
  segments,
  height = 'h-2',
  rounded = true,
  className = '',
  showTooltip = true,
}) => {
  const total = segments.reduce((sum, s) => sum + Math.max(0, s.value), 0);
  const roundingCls = rounded ? 'rounded-full' : '';

  if (total === 0) {
    return (
      <div className={`w-full ${height} ${roundingCls} bg-gray-200 dark:bg-surface-700 ${className}`} />
    );
  }

  return (
    <div
      className={`w-full ${height} ${roundingCls} flex overflow-hidden bg-gray-200 dark:bg-surface-700 ${className}`}
    >
      {segments.map((seg, i) => {
        const pct = (Math.max(0, seg.value) / total) * 100;
        if (pct === 0) return null;
        return (
          <div
            key={i}
            className={SEGMENT_CLASSES[seg.color]}
            style={{ width: `${pct}%` }}
            title={showTooltip ? (seg.label ?? `${seg.color}: ${seg.value}`) : undefined}
          />
        );
      })}
    </div>
  );
};

export default ProgressLine;
