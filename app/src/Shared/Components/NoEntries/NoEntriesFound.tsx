import React from 'react';

interface NoEntriesFoundProps {
    title?: string;
    message?: string;
    icon?: React.ReactNode;
    className?: string;
}

export const NoEntriesFound: React.FC<NoEntriesFoundProps> = ({
    title = 'No entries found',
    message = 'We could not find any records matching your criteria.',
    icon,
    className = ''
}) => {
    return (
        <div className={`flex flex-col items-center justify-center p-12 text-center rounded-xl border border-dashed border-gray-300 dark:border-surface-700 bg-gray-50/50 dark:bg-surface-800/30 ${className}`}>
            <div className="flex items-center justify-center w-16 h-16 rounded-full bg-gray-100 dark:bg-surface-800 mb-4 shadow-sm">
                {icon ? (
                    icon
                ) : (
                    <svg className="w-8 h-8 text-gray-400 dark:text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                )}
            </div>
            <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-1">
                {title}
            </h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 max-w-sm">
                {message}
            </p>
        </div>
    );
};

export default NoEntriesFound;
