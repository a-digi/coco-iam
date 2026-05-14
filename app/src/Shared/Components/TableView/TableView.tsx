import type { ReactNode } from 'react';
import Table from '../Table/Table';
import type { TableColumn } from '../Table/Table';
import Pagination from '../Pagination/Pagination';
import Filter from '../Filter/Filter';
import Dropdown from '../Dropdown/Dropdown';
import { useState, useEffect, useRef } from 'react';
import FilterItem from '../Filter/FilterItem';
import { searchInputTimeout } from '../../../config/data/search/timeout.ts';

export type TableViewFilterType = 'text' | 'select' | 'number' | 'greater' | 'lower' | 'between' | 'date' | 'sort' | 'date-greater' | 'date-lower' | 'date-between';

export interface TableViewFilterConfig {
  type: TableViewFilterType;
  label?: string;
  options?: { label: string; value: string }[];
  placeholder?: string;
  min?: number;
  max?: number;
}

export type FilteredValueValue = string | number | null | [number | null, number | null] | [string | null, string | null];
export type FilteredValue = Record<string, FilteredValueValue>;

export interface TableViewDropdownFilter {
  [key: string]: TableViewFilterConfig;
}

interface TableViewProps<T> {
  columns: TableColumn<T>[];
  data: T[];
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  filters?: ReactNode | TableViewDropdownFilter;
  onFilterChange?: (values: FilteredValue[]) => void;
  rowKey?: (row: T, index: number) => string | number;
  className?: string;
  tableClassName?: string;
  emptyText?: string;
}

function TableView<T>({
  columns,
  data,
  total,
  page,
  pageSize,
  onPageChange,
  filters,
  onFilterChange,
  rowKey,
  className = '',
  tableClassName = '',
  emptyText = 'No data found',
}: TableViewProps<T>) {
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
    if (filterChangeTimeout.current) {
      clearTimeout(filterChangeTimeout.current);
    }

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


  let filterSelector: ReactNode = null;
  let filterElements: ReactNode = null;
  let filterSettingsIcon: ReactNode = null;
  if (filters && typeof filters === 'object' && !Array.isArray(filters)) {
    const filterKeys = Object.keys(filters);
    const availableFilterKeys = filterKeys.filter(key => !visibleFilters.includes(key));
    filterSettingsIcon = (
      <button
        type="button"
        aria-label="Filter Einstellungen"
        className={`ml-2 p-2 rounded hover:bg-gray-200 dark:hover:bg-surface-900 transition-transform ${showFilterDropdown ? 'text-blue-600 rotate-90' : 'text-gray-600'}`}
        onClick={() => setShowFilterDropdown(v => !v)}
      >
        <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M3 4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v1.382a2 2 0 0 1-.553 1.38l-4.447 4.447V16a1 1 0 0 1-1.447.894l-2-1A1 1 0 0 1 7 15V11.209l-4.447-4.447A2 2 0 0 1 2 5.382V4zm2.618 1L8 7.382V15l2 1V7.382l2.382-2.382H3.618z" fill="currentColor" />
        </svg>
      </button>
    );
    if (showFilterDropdown && availableFilterKeys.length > 0) {
      filterSelector = (
        <Dropdown
          name="filterSelector"
          options={availableFilterKeys
            .map(key => ({ label: (filters as TableViewDropdownFilter)[key].label || key.charAt(0).toUpperCase() + key.slice(1), value: key }))}
          placeholder="Filter auswählen"
          value={''}
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
          config={(filters as TableViewDropdownFilter)[key]
          }
          onClose={closedKey => {
            setVisibleFilters(prev => prev.filter(f => f !== closedKey));
            setFilteredValues(prev => {
              const newValues = { ...prev };
              delete newValues[closedKey];
              return newValues;
            });
          }}
          onValueChange={(filterKey, value) => {
            setFilteredValues(prev => ({ ...prev, [filterKey]: value }));
          }}
        />
      ));
  } else if (filters) {
    filterElements = filters;
  }

  return (
    <div className={`w-full ${className}`}>
      {filterSettingsIcon && (
        <div className="mb-4 flex items-center">
          {filterSettingsIcon}
          {showFilterDropdown && (
            <div className="flex items-center">
              {filterSelector}
              {filterElements && (
                <Filter onChange={() => { }}>{filterElements}</Filter>
              )}
            </div>
          )}
        </div>
      )}
      <Table
        columns={columns}
        data={data}
        rowKey={rowKey}
        className={tableClassName}
        emptyText={emptyText}
      />
      <div className="mt-4 flex justify-center">
        <Pagination
          currentPage={page}
          totalPages={totalPages}
          onPageChange={onPageChange}
        />
      </div>
    </div>
  );
}

export default TableView;
