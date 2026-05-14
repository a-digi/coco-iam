import React from 'react';

interface TwoColumnsLeftProps {
  left: React.ReactNode;
  right: React.ReactNode;
  className?: string;
}

export const TwoColumnsLeft: React.FC<TwoColumnsLeftProps> = ({ left, right, className = '' }) => (
  <div className={`flex gap-6 items-start ${className}`}>
    <div className="flex-1 min-w-0">{left}</div>
    <div className="w-[30%] shrink-0">{right}</div>
  </div>
);

export default TwoColumnsLeft;
