import React from 'react';

export interface GridProps {
  children: React.ReactNode;
  columns?: number;
  gap?: string;
  className?: string;
}

const Grid: React.FC<GridProps> = ({ children, columns = 3, gap = 'gap-6', className = '' }) => {
  return (
    <div
      className={`grid ${gap} grid-cols-1 sm:grid-cols-2 md:grid-cols-${columns} ${className}`.trim()}
    >
      {children}
    </div>
  );
};

export default Grid;
