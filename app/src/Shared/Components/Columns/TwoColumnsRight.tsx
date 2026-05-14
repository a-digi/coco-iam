import React from 'react';

interface TwoColumnsRightProps {
  left: React.ReactNode;
  right: React.ReactNode;
  className?: string;
}

export const TwoColumnsRight: React.FC<TwoColumnsRightProps> = ({ left, right, className = '' }) => (
  <div className={`flex gap-6 items-start ${className}`}>
    <div className="w-[30%] shrink-0">{left}</div>
    <div className="flex-1 min-w-0">{right}</div>
  </div>
);

export default TwoColumnsRight;
