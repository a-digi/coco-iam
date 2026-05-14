import { type ReactNode, useState, useEffect, useRef } from 'react';
import { Masonry } from '../Masonry';
import Pagination from '../Pagination/Pagination';
import Filter from '../Filter/Filter';
import Dropdown from '../Dropdown/Dropdown';
import FilterItem from '../Filter/FilterItem';
import { searchInputTimeout } from '../../../config/data/search/timeout.ts';
import type { TableViewDropdownFilter, FilteredValue } from '../TableView/TableView';

/**
 * MasonryView is the card-list sibling of TableView / CardView. It
 * takes an already-paginated `data` slice and a `renderItem` callback
 * that produces a custom card per item — unlike CardView which derives
 * its cards from `columns`. The outer chrome (filter toolbar,
 * pagination, page-size selector) is identical to TableView so users
 * get the same mental model across the three views.
 *
 * As with TableView, `page`, `pageSize`, `total`, and `onPageChange`
 * are owned by the caller — the caller decides whether filtering /
 * pagination happens client-side or via a backend round-trip.
 *
 * `pageSizeOptions` enables the "limit (amount)" selector. Pass
 * `false` (or an empty array) to hide it.
 */
export interface MasonryViewProps<T> {
  data: T[];
  renderItem: (item: T, index: number) => ReactNode;
  itemKey: (item: T, index: number) => string | number;

  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  /** When provided, a small "Show <N>" selector appears next to the
   *  pagination. Hiding is explicit: pass `false` or `[]`. */
  pageSizeOptions?: number[] | false;
  onPageSizeChange?: (pageSize: number) => void;

  filters?: ReactNode | TableViewDropdownFilter;
  onFilterChange?: (values: FilteredValue[]) => void;

  // Masonry layout knobs — forwarded to <Masonry>.
  columns?: number;
  gap?: number | string;
  breakpointCols?: Record<number, number>;

  className?: string;
  emptyText?: string;
}

const DEFAULT_PAGE_SIZE_OPTIONS = [12, 24, 48, 96];
const DEFAULT_BREAKPOINT_COLS: Record<number, number> = { 640: 2, 1024: 3, 1440: 4 };

function MasonryView<T>({
  data,
  renderItem,
  itemKey,
  total,
  page,
  pageSize,
  onPageChange,
  pageSizeOptions,
  onPageSizeChange,
  filters,
  onFilterChange,
  columns = 1,
  gap = 16,
  breakpointCols = DEFAULT_BREAKPOINT_COLS,
  className = '',
  emptyText = 'No data found',
}: MasonryViewProps<T>) {
  const totalPages = Math.max(1, Math.ceil(total / Math.max(1, pageSize)));

  // Filter toolbar state — copy of the TableView/CardView pattern so
  // a future refactor can hoist it. Kept in-file for now to avoid a
  // shared-hook churn that's out of scope.
  const [visibleFilters, setVisibleFilters] = useState<string[]>([]);
  const [filteredValues, setFilteredValues] = useState<FilteredValue>({});
  const [showFilterDropdown, setShowFilterDropdown] = useState(false);
  const filterChangeTimeout = useRef<number | null>(null);
  const prevShowFilterDropdown = useRef(showFilterDropdown);

  useEffect(() => {
    if (prevShowFilterDropdown.current && !showFilterDropdown) {
      setTimeout(() => {
        setFilteredValues({});
        setVisibleFilters([]);
      }, 0);
    }
    prevShowFilterDropdown.current = showFilterDropdown;
  }, [showFilterDropdown]);

  useEffect(() => {
    if (!onFilterChange) return;
    if (filterChangeTimeout.current) clearTimeout(filterChangeTimeout.current);
    filterChangeTimeout.current = window.setTimeout(() => {
      onFilterChange([filteredValues]);
      filterChangeTimeout.current = null;
    }, searchInputTimeout);
    return () => {
      if (filterChangeTimeout.current) {
        clearTimeout(filterChangeTimeout.current);
        filterChangeTimeout.current = null;
      }
    };
  }, [filteredValues, onFilterChange]);

  // Filter toolbar rendering — identical logic to TableView / CardView
  let filterSelector: ReactNode = null;
  let filterElements: ReactNode = null;
  let filterSettingsIcon: ReactNode = null;

  if (filters && typeof filters === 'object' && !Array.isArray(filters)) {
    const filterKeys = Object.keys(filters);
    const availableFilterKeys = filterKeys.filter(key => !visibleFilters.includes(key));
    filterSettingsIcon = (
      <button
        type="button"
        aria-label="Filter settings"
        className={`ml-2 p-2 rounded hover:bg-gray-200 dark:hover:bg-surface-900 transition-transform ${showFilterDropdown ? 'text-blue-600 rotate-90' : 'text-gray-600'}`}
        onClick={() => setShowFilterDropdown(v => !v)}
      >
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
          <path d="M3 4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v1.382a2 2 0 0 1-.553 1.38l-4.447 4.447V16a1 1 0 0 1-1.447.894l-2-1A1 1 0 0 1 7 15V11.209l-4.447-4.447A2 2 0 0 1 2 5.382V4zm2.618 1L8 7.382V15l2 1V7.382l2.382-2.382H3.618z" fill="currentColor" />
        </svg>
      </button>
    );
    if (showFilterDropdown && availableFilterKeys.length > 0) {
      filterSelector = (
        <Dropdown
          name="filterSelector"
          options={availableFilterKeys.map(key => ({
            label: (filters as TableViewDropdownFilter)[key].label || key.charAt(0).toUpperCase() + key.slice(1),
            value: key,
          }))}
          placeholder="Select filter"
          value=""
          onChange={opt => {
            const key = typeof opt.value === 'string' ? opt.value : String(opt.value);
            if (key && !visibleFilters.includes(key)) setVisibleFilters(prev => [...prev, key]);
          }}
          className="min-w-[180px] mb-2"
        />
      );
    }
    filterElements = visibleFilters
      .filter(key => filterKeys.includes(key))
      .map(key => (
        <FilterItem
          key={key}
          filterKey={key}
          config={(filters as TableViewDropdownFilter)[key]}
          onClose={closedKey => {
            setVisibleFilters(prev => prev.filter(f => f !== closedKey));
            setFilteredValues(prev => {
              const next = { ...prev };
              delete next[closedKey];
              return next;
            });
          }}
          onValueChange={(filterKey, value) => {
            setFilteredValues(prev => ({ ...prev, [filterKey]: value }));
          }}
        />
      ));
  } else if (filters) {
    filterElements = filters as ReactNode;
  }

  const sizeOptions: number[] =
    pageSizeOptions === false
      ? []
      : pageSizeOptions && pageSizeOptions.length > 0
        ? pageSizeOptions
        : onPageSizeChange
          ? DEFAULT_PAGE_SIZE_OPTIONS
          : [];

  return (
    <div className={`w-full ${className}`}>
      {/* Filter toolbar */}
      {filterSettingsIcon && (
        <div className="mb-4 flex items-center">
          {filterSettingsIcon}
          {showFilterDropdown && (
            <div className="flex items-center">
              {filterSelector}
              {filterElements && <Filter onChange={() => { }}>{filterElements}</Filter>}
            </div>
          )}
        </div>
      )}
      {!filterSettingsIcon && filterElements && (
        <div className="mb-4">
          <Filter onChange={() => { }}>{filterElements}</Filter>
        </div>
      )}

      {/* Cards */}
      {data.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
          <svg className="w-12 h-12 mb-3 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M20 13V7a2 2 0 00-2-2H6a2 2 0 00-2 2v6m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0H4" />
          </svg>
          <span className="text-sm">{emptyText}</span>
        </div>
      ) : (
        <Masonry columns={columns} gap={gap} breakpointCols={breakpointCols}>
          {data.map((item, i) => (
            <div key={itemKey(item, i)}>{renderItem(item, i)}</div>
          ))}
        </Masonry>
      )}

      {/* Footer — range summary, page-size selector, pagination */}
      <div className="mt-6 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500 dark:text-gray-400">
        <span>
          {total === 0
            ? 'No results'
            : `${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, total)} of ${total}`}
        </span>
        <div className="flex items-center gap-3">
          {sizeOptions.length > 0 && onPageSizeChange && (
            <label className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-widest text-gray-400">Show</span>
              <select
                value={pageSize}
                onChange={e => onPageSizeChange(Number(e.target.value))}
                className="px-2 py-1 rounded-md border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-800 text-sm text-gray-900 dark:text-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              >
                {sizeOptions.includes(pageSize)
                  ? null
                  : <option value={pageSize}>{pageSize}</option>}
                {sizeOptions.map(n => (
                  <option key={n} value={n}>{n}</option>
                ))}
              </select>
            </label>
          )}
          <Pagination currentPage={page} totalPages={totalPages} onPageChange={onPageChange} />
        </div>
      </div>
    </div>
  );
}

export default MasonryView;
