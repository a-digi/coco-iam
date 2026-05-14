import React, { useContext } from 'react';
import { Link } from 'react-router-dom';
import { BreadcrumbContext } from './BreadcrumbContext';

export const BreadcrumbBar: React.FC = () => {
  const ctx = useContext(BreadcrumbContext);
  if (!ctx || ctx.items.length === 0) return null;

  return (
    <nav
      aria-label="Breadcrumb"
      className="sticky top-0 z-20 flex items-center gap-1.5 px-6 h-9 bg-gray-50/90 dark:bg-surface-900/90 backdrop-blur border-b border-gray-100 dark:border-surface-800 text-sm"
    >
      {/* Home icon */}
      <Link
        to="/"
        className="text-gray-400 hover:text-indigo-600 dark:text-gray-500 dark:hover:text-indigo-400 transition-colors shrink-0"
        aria-label="Home"
      >
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12l8.954-8.955a1.126 1.126 0 011.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
        </svg>
      </Link>

      {ctx.items.map((item, i) => {
        const isLast = i === ctx.items.length - 1;
        return (
          <React.Fragment key={i}>
            {/* Separator */}
            <svg className="w-3 h-3 text-gray-300 dark:text-gray-600 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
            </svg>

            {isLast || !item.href ? (
              <span className={`truncate ${isLast ? 'text-gray-800 dark:text-gray-200 font-medium' : 'text-gray-500 dark:text-gray-400'}`}>
                {item.label}
              </span>
            ) : (
              <Link
                to={item.href}
                className="text-gray-500 dark:text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors truncate"
              >
                {item.label}
              </Link>
            )}
          </React.Fragment>
        );
      })}
    </nav>
  );
};
