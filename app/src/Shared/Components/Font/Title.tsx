import React from 'react';

interface TitleProps {
  children: React.ReactNode;
  className?: string;
}

const Title: React.FC<TitleProps> = ({ children, className = '' }) => {
  return (
    <h1 className={`text-xl font-bold mb-4 text-gray-900 dark:text-gray-100 ${className}`}>
      {children}
    </h1>
  );
};

export default Title;
