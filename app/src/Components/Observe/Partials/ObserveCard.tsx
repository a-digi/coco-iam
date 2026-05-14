import React from 'react';

interface ObserveCardProps {
  title: string;
  children: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}

export const ObserveCard: React.FC<ObserveCardProps> = ({ title, children, actions, className = '' }) => (
  <div
    className={`bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-2xl shadow-sm p-5 ${className}`}
  >
    <div className="flex items-center justify-between mb-4">
      <h3 className="text-[0.75rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
        {title}
      </h3>
      {actions && <div className="flex items-center gap-2">{actions}</div>}
    </div>
    {children}
  </div>
);
