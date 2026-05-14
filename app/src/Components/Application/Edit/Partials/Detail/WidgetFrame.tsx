import React from 'react';

interface Props {
  title: string;
  loading: boolean;
  error: string | null;
  onRetry?: () => void;
  children: React.ReactNode;
}

/**
 * WidgetFrame wraps each analytics widget with a consistent shell:
 * title header, loading skeleton, error banner, or content.
 */
export const WidgetFrame: React.FC<Props> = ({ title, loading, error, onRetry, children }) => {
  return (
    <div className="bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-xl p-4 shadow-sm">
      <h3 className="text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-2">
        {title}
      </h3>
      {loading ? (
        <div className="h-[48px] animate-pulse bg-gray-100 dark:bg-surface-700 rounded-md" />
      ) : error ? (
        <div className="text-sm text-red-500">
          <span className="font-medium">Failed to load.</span>{' '}
          {onRetry && (
            <button type="button" onClick={onRetry} className="underline hover:no-underline">
              Retry
            </button>
          )}
        </div>
      ) : (
        children
      )}
    </div>
  );
};

export default WidgetFrame;
