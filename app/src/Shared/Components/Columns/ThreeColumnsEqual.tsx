import React from 'react';

interface ThreeColumnsEqualProps {
  left: React.ReactNode;
  center: React.ReactNode;
  right: React.ReactNode;
  className?: string;
}

export const ThreeColumnsEqual: React.FC<ThreeColumnsEqualProps> = ({ left, center, right, className = '' }) => (
  <div className={`grid grid-cols-1 md:grid-cols-3 gap-6 items-start ${className}`}>
    <div className="min-w-0">{left}</div>
    <div className="min-w-0">{center}</div>
    <div className="min-w-0">{right}</div>
  </div>
);

export default ThreeColumnsEqual;
