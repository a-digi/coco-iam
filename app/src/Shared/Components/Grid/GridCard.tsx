import React from 'react';

export interface GridCardProps {
  children: React.ReactNode;
  className?: string;
}

const GridCard: React.FC<GridCardProps> = ({ children, className = '' }) => (
  <div
    className={`bg-gray-50 dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 transition hover:shadow-md hover:bg-gray-100 dark:hover:bg-surface-900 ${className}`.trim()}
  >
    {children}
  </div>
);

export default GridCard;
