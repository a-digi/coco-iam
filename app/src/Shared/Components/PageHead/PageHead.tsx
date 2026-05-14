import React from 'react';

interface PageHeadProps {
  /** Small uppercase text above the title (e.g. "Organization"). */
  kicker?: string;
  /** Primary heading — typically the entity name. */
  title: string;
  /** Optional secondary line below the title. */
  description?: string;
  /** Right-aligned slot for action buttons / links. */
  actions?: React.ReactNode;
  /** Optional slot rendered to the side of the title block, separated by a vertical rule. */
  meta?: React.ReactNode;
  /** Extra classes merged onto the outer wrapper. */
  className?: string;
}

export const PageHead: React.FC<PageHeadProps> = ({
  kicker,
  title,
  description,
  actions,
  meta,
  className = '',
}) => {
  return (
    <div
      className={`mb-6 p-4 rounded-lg bg-gradient-to-r from-indigo-50 to-white dark:from-surface-800 dark:to-surface-900 border border-indigo-100 dark:border-surface-800 ${className}`}
    >
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="flex items-start gap-0 min-w-0 flex-1">
          {/* Primary identity block */}
          <div className="min-w-0">
            {kicker && (
              <div className="text-xs uppercase tracking-wide text-indigo-600 dark:text-indigo-400 mb-1">
                {kicker}
              </div>
            )}
            <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 truncate">
              {title}
            </h2>
            {description && (
              <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">
                {description}
              </p>
            )}
          </div>

          {/* Meta slot — separated by a thin vertical rule */}
          {meta && (
            <>
              <div className="self-stretch w-px bg-indigo-200 dark:bg-surface-600 mx-5 shrink-0" />
              <div className="shrink-0">{meta}</div>
            </>
          )}
        </div>
        {actions && (
          <div className="flex flex-wrap gap-2">{actions}</div>
        )}
      </div>
    </div>
  );
};

export default PageHead;
