import React, { useEffect, useMemo, useState } from 'react';

export interface MasonryProps {
  /** Default column count. */
  columns?: number;
  /** Gap between items and columns (number → px, or any CSS length). */
  gap?: number | string;
  /**
   * Responsive column counts keyed by min-width (px).
   * Example: `{ 640: 2, 1024: 3, 1440: 4 }`.
   * The highest matching breakpoint wins.
   */
  breakpointCols?: Record<number, number>;
  children: React.ReactNode;
  className?: string;
}

const resolveColumns = (
  defaultCols: number,
  breakpointCols: Record<number, number> | undefined,
  width: number
): number => {
  if (!breakpointCols) return defaultCols;
  const sorted = Object.entries(breakpointCols)
    .map(([minWidth, cols]) => [Number(minWidth), cols] as [number, number])
    .sort((a, b) => a[0] - b[0]);
  let matched = defaultCols;
  for (const [minWidth, cols] of sorted) {
    if (width >= minWidth) matched = cols;
  }
  return matched;
};

/**
 * Masonry layout via round-robin column distribution.
 * Renders N flex columns; each child is placed into the next column in order.
 * Row alignment is preserved per column (items stack top → bottom).
 */
export const Masonry: React.FC<MasonryProps> = ({
  columns = 3,
  gap = 16,
  breakpointCols,
  children,
  className = '',
}) => {
  const gapValue = typeof gap === 'number' ? `${gap}px` : gap;
  const [width, setWidth] = useState<number>(() =>
    typeof window !== 'undefined' ? window.innerWidth : 1024
  );

  useEffect(() => {
    const onResize = () => setWidth(window.innerWidth);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  const effectiveCols = resolveColumns(columns, breakpointCols, width);

  const columnBuckets = useMemo(() => {
    const arr = React.Children.toArray(children);
    const buckets: React.ReactNode[][] = Array.from({ length: effectiveCols }, () => []);
    arr.forEach((child, i) => {
      buckets[i % effectiveCols].push(child);
    });
    return buckets;
  }, [children, effectiveCols]);

  return (
    <div className={`flex w-full ${className}`} style={{ gap: gapValue, alignItems: 'flex-start' }}>
      {columnBuckets.map((bucket, colIdx) => (
        <div
          key={colIdx}
          className="flex flex-col min-w-0 flex-1"
          style={{ gap: gapValue }}
        >
          {bucket.map((child, itemIdx) => (
            <div key={itemIdx} className="min-w-0">
              {child}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
};

export default Masonry;
