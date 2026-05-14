import React from 'react';

interface DashboardCardProps {
  title: string;
  children: React.ReactNode;
  className?: string;
}

export const DashboardCard: React.FC<DashboardCardProps> = ({ title, children, className = '' }) => {
  return (
    <div
      className={`bg-white dark:bg-surface-800 border border-gray-200 dark:border-surface-700 rounded-2xl shadow-sm p-5 ${className}`}
    >
      <h3 className="text-[0.75rem] font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-4">
        {title}
      </h3>
      {children}
    </div>
  );
};

export default DashboardCard;
