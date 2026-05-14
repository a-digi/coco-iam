import React from 'react';

export const WidgetSkeleton: React.FC<{ className?: string }> = ({ className = 'h-[200px]' }) => (
  <div className={`animate-pulse rounded-xl bg-gray-100 dark:bg-surface-700 ${className}`} />
);

export const WidgetError: React.FC<{ message: string; onRetry?: () => void }> = ({ message, onRetry }) => (
  <div className="flex flex-col items-center justify-center py-8 text-[0.875rem] text-red-500 dark:text-red-400">
    <span className="font-medium mb-1">Failed to load</span>
    <span className="text-[0.75rem] text-gray-500 dark:text-gray-400 mb-2">{message}</span>
    {onRetry && (
      <button
        type="button"
        onClick={onRetry}
        className="text-[0.75rem] underline hover:no-underline"
      >
        Retry
      </button>
    )}
  </div>
);
