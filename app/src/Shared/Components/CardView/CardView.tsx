import { type ReactNode, useState, useEffect, useRef } from 'react';
import type { TableColumn } from '../Table/Table';
import Pagination from '../Pagination/Pagination';
import Filter from '../Filter/Filter';
import Dropdown from '../Dropdown/Dropdown';
import FilterItem from '../Filter/FilterItem';
import { searchInputTimeout } from '../../../config/data/search/timeout.ts';
import type { TableViewDropdownFilter, FilteredValue } from '../TableView/TableView';

interface CardViewProps<T> {
  columns: TableColumn<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  filters?: ReactNode | TableViewDropdownFilter;
  onFilterChange?: (values: FilteredValue[]) => void;
  rowKey?: (row: T, index: number) => string | number;
  /** Column key whose render output appears in the card header's top-right corner instead of the body grid. */
  actionsKey?: keyof T;
  className?: string;
  emptyText?: string;
}

// Determine inner-card grid columns based on number of body fields.
function bodyGridClass(count: number): string {
  if (count <= 1) return 'grid-cols-1';
  if (count <= 4) return 'grid-cols-2';
  return 'grid-cols-3';
}

function CardView<T>({
  columns,
  data,
  total,
  page,
  pageSize,
  onPageChange,
  filters,
  onFilterChange,
  rowKey,
  actionsKey,
  className = '',
  emptyText = 'No data found',
}: CardViewProps<T>) {
  const totalPages = Math.ceil(total / pageSize);

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

  // Filter toolbar — identical logic to TableView
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

  // Split columns: first is the card title, actionsKey (if any) goes to the header corner, rest are body fields
  const [titleCol, ...rest] = columns;
  const actionsCol = actionsKey ? rest.find(c => c.key === actionsKey) : undefined;
  const bodyColumns = rest.filter(c => c.key !== actionsKey);
  const gridClass = bodyGridClass(bodyColumns.length);

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

      {/* Cards grid */}
      {data.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
          <svg className="w-12 h-12 mb-3 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M20 13V7a2 2 0 00-2-2H6a2 2 0 00-2 2v6m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0H4" />
          </svg>
          <span className="text-sm">{emptyText}</span>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {data.map((row, rowIndex) => {
            const key = rowKey ? rowKey(row, rowIndex) : rowIndex;
            const rowIdx = row as Record<string, unknown>;
            const titleValue = titleCol.render
              ? titleCol.render(rowIdx[titleCol.key], row, rowIndex)
              : String(rowIdx[titleCol.key] ?? '');

            return (
              <div
                key={key}
                className="group bg-white dark:bg-surface-800 rounded-2xl border border-gray-100 dark:border-surface-700 shadow-sm hover:shadow-md hover:border-indigo-200 dark:hover:border-indigo-800 transition-all duration-200 flex flex-col"
              >
                {/* Card header — title field + optional actions in top-right */}
                <div className="px-5 pt-5 pb-4 border-b border-gray-50 dark:border-surface-700 flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-[0.625rem] font-semibold uppercase tracking-widest text-indigo-500 dark:text-indigo-400 mb-1">
                      {titleCol.label}
                    </div>
                    <div className="text-base font-semibold text-gray-900 dark:text-gray-100 leading-snug">
                      {titleValue}
                    </div>
                  </div>
                  {actionsCol && (
                    <div className="shrink-0 -mt-1 -mr-1">
                      {actionsCol.render
                        ? actionsCol.render(rowIdx[actionsCol.key], row, rowIndex)
                        : null}
                    </div>
                  )}
                </div>

                {/* Card body — remaining fields in auto grid */}
                {bodyColumns.length > 0 && (
                  <div className={`grid ${gridClass} gap-x-4 gap-y-4 px-5 py-4 flex-1`}>
                    {bodyColumns.map(col => {
                      const value = col.render
                        ? col.render(rowIdx[col.key], row, rowIndex)
                        : String(rowIdx[col.key] ?? '');
                      return (
                        <div key={String(col.key)}>
                          <div className="text-[0.625rem] font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500 mb-0.5">
                            {col.label}
                          </div>
                          <div className="text-sm text-gray-800 dark:text-gray-200 leading-snug break-words">
                            {value}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Footer pagination */}
      <div className="mt-6 flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
        <span>
          {total === 0
            ? 'No results'
            : `${(page - 1) * pageSize + 1}–${Math.min(page * pageSize, total)} of ${total}`}
        </span>
        <Pagination currentPage={page} totalPages={totalPages} onPageChange={onPageChange} />
      </div>
    </div>
  );
}

export default CardView;
