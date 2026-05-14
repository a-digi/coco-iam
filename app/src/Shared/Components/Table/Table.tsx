import React from 'react';

export interface TableColumn<T> {
  // `key` doubles as both a property lookup on T (for the
  // default cell renderer) and as a React key for the column.
  // Callers with custom `render` functions that don't index
  // into T can pass synthetic keys like "actions" or nested
  // paths like "counts.pending" — the default renderer only
  // runs when the key actually matches a T property.
  key: (keyof T & string) | string;
  label: string;
  className?: string;
  render?: (value: unknown, row: T, rowIndex: number) => React.ReactNode;
}

interface TableProps<T> {
  columns: TableColumn<T>[];
  data: T[];
  rowKey?: (row: T, index: number) => string | number;
  className?: string;
  headerClassName?: string;
  rowClassName?: (row: T, index: number) => string;
  emptyText?: string;
}

function Table<T>({
  columns,
  data,
  rowKey,
  className = '',
  headerClassName = '',
  rowClassName,
  emptyText = 'No data found',
}: TableProps<T>) {
  return (
    <div
      className={`overflow-hidden rounded-xl bg-white dark:bg-surface-900 shadow-sm ring-1 ring-gray-200/70 dark:ring-surface-700/80 ${className}`}
    >
      <div className="overflow-x-auto">
        <table className="min-w-full">
          <thead
            className={`bg-gray-50/80 dark:bg-surface-800 border-b border-gray-200 dark:border-surface-700 ${headerClassName}`}
          >
            <tr>
              {columns.map(col => (
                <th
                  key={String(col.key)}
                  className={`px-4 py-3 text-left text-[0.6875rem] font-semibold text-gray-600 dark:text-gray-300 uppercase tracking-widest ${col.className || ''}`}
                >
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-surface-800">
            {data.length === 0 ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-10 text-center text-sm text-gray-400 dark:text-gray-500"
                >
                  {emptyText}
                </td>
              </tr>
            ) : (
              data.map((row, rowIndex) => (
                <tr
                  key={rowKey ? rowKey(row, rowIndex) : rowIndex}
                  className={`transition-colors hover:bg-gray-50 dark:hover:bg-surface-800/60 ${
                    rowClassName ? rowClassName(row, rowIndex) : ''
                  }`}
                >
                  {columns.map(col => (
                    <td
                      key={String(col.key)}
                      className="px-4 py-3 whitespace-nowrap text-sm text-gray-800 dark:text-gray-200"
                    >
                      {col.render
                        ? col.render((row as Record<string, unknown>)[col.key], row, rowIndex)
                        : String((row as Record<string, unknown>)[col.key] ?? '')}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export default Table;
