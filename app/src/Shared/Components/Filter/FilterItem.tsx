import React, { useState } from 'react';
import Dropdown from '../Dropdown/Dropdown';
import type { TableViewFilterConfig } from '../TableView/TableView.tsx';

interface FilterItemProps {
  filterKey: string;
  config: TableViewFilterConfig;
  onValueChange?: (key: string, value: string | number | null | [number | null, number | null] | [string | null, string | null]) => void;
  onClose: (key: string) => void;
}

const FilterItem: React.FC<FilterItemProps> = ({ filterKey, config, onValueChange, onClose }) => {
  const [dropdownValue, setDropdownValue] = useState<string | number | null>('');

  const handleChange = (val: string | number | null | [number | null, number | null] | [string | null, string | null]) => {
    if (config.type === 'select') {
      setDropdownValue(val as string | number | null);
    }
    if (onValueChange) onValueChange(filterKey, val);
  };

  return (
    <div className="relative inline-block mr-2 group">
      {/* Filter-Element */}
      {(() => {
        switch (config.type) {
          case 'select':
            return (
              <Dropdown
                name={filterKey}
                options={config.options || []}
                placeholder={config.placeholder || config.label || filterKey.charAt(0).toUpperCase() + filterKey.slice(1)}
                value={dropdownValue ?? ''}
                onChange={opt => handleChange(opt.value)}
              />
            );
          case 'text':
            return (
              <input
                name={filterKey}
                type="text"
                onChange={e => handleChange(e.target.value)}
                placeholder={config.placeholder || config.label || filterKey.charAt(0).toUpperCase() + filterKey.slice(1)}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'number':
            return (
              <input
                name={filterKey}
                type="number"
                onChange={e => handleChange(e.target.value === '' ? null : Number(e.target.value))}
                min={config.min}
                max={config.max}
                placeholder={config.placeholder || config.label || filterKey.charAt(0).toUpperCase() + filterKey.slice(1)}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'greater':
            return (
              <input
                name={filterKey}
                type="number"
                onChange={e => handleChange(e.target.value === '' ? null : Number(e.target.value))}
                min={config.min}
                placeholder={config.placeholder || `>= ${config.label || filterKey}`}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'lower':
            return (
              <input
                name={filterKey}
                type="number"
                onChange={e => handleChange(e.target.value === '' ? null : Number(e.target.value))}
                max={config.max}
                placeholder={config.placeholder || `<= ${config.label || filterKey}`}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'between':
            return (
              <span className="flex items-center mx-1">
                <input
                  name={`${filterKey}_min`}
                  type="number"
                  onChange={e => handleChange([e.target.value === '' ? null : Number(e.target.value), null])}
                  min={config.min}
                  placeholder={config.placeholder || `Min ${config.label || filterKey}`}
                  className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
                />
                <span className="mx-1">-</span>
                <input
                  name={`${filterKey}_max`}
                  type="number"
                  onChange={e => handleChange([null, e.target.value === '' ? null : Number(e.target.value)])}
                  max={config.max}
                  placeholder={config.placeholder || `Max ${config.label || filterKey}`}
                  className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
                />
              </span>
            );
          case 'date':
            return (
              <input
                name={filterKey}
                type="date"
                onChange={e => handleChange(e.target.value)}
                placeholder={config.placeholder || config.label || filterKey.charAt(0).toUpperCase() + filterKey.slice(1)}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'sort':
            return (
              <select
                name={filterKey}
                onChange={e => handleChange(e.target.value)}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
                defaultValue=""
              >
                <option value="">{config.placeholder || config.label || 'Sortierung wählen'}</option>
                <option value="asc">Aufsteigend</option>
                <option value="desc">Absteigend</option>
              </select>
            );
          case 'date-greater':
            return (
              <input
                name={filterKey}
                type="date"
                onChange={e => handleChange(e.target.value)}
                placeholder={config.placeholder || `>= ${config.label || filterKey}`}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'date-lower':
            return (
              <input
                name={filterKey}
                type="date"
                onChange={e => handleChange(e.target.value)}
                placeholder={config.placeholder || `<= ${config.label || filterKey}`}
                className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
              />
            );
          case 'date-between':
            return (
              <span className="flex items-center mx-1">
                <input
                  name={`${filterKey}_min`}
                  type="date"
                  onChange={e => handleChange([e.target.value || null, null] as [string | null, string | null])}
                  placeholder={config.placeholder || `Min ${config.label || filterKey}`}
                  className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
                />
                <span className="mx-1">-</span>
                <input
                  name={`${filterKey}_max`}
                  type="date"
                  onChange={e => handleChange([null, e.target.value || null] as [string | null, string | null])}
                  placeholder={config.placeholder || `Max ${config.label || filterKey}`}
                  className="border px-2 py-1 rounded bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 mx-1"
                />
              </span>
            );
          default:
            return null;
        }
      })()}
      <button
        type="button"
        aria-label="Filter schließen"
        className="absolute -top-2 -right-2 bg-gray-200 dark:bg-surface-900 rounded-full w-6 h-6 flex items-center justify-center text-gray-700 dark:text-gray-200 hover:bg-red-400 hover:text-white transition opacity-0 group-hover:opacity-100 cursor-pointer"
        onClick={() => onClose(filterKey)}
        tabIndex={-1}
      >
        ×
      </button>
    </div>
  );
};

export default FilterItem;
