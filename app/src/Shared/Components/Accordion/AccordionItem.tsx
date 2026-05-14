import React from 'react';

export interface AccordionItemProps {
    title: string;
    isOpen: boolean;
    onToggle: () => void;
    children: React.ReactNode;
    variant?: 'standalone' | 'grouped';
    forceMount?: boolean;
    accessMe?: boolean;
}

export const AccordionItem: React.FC<AccordionItemProps> = ({
    title,
    isOpen,
    onToggle,
    children,
    variant = 'standalone',
    forceMount = false,
    accessMe = false,
}) => {
    const isStandalone = variant === 'standalone';

    const containerClasses = isStandalone
        ? "border border-gray-200 dark:border-surface-900 rounded-lg mb-4 bg-white dark:bg-surface-800 shadow-sm"
        : "border-b border-gray-200 dark:border-surface-900 last:border-b-0 bg-white dark:bg-surface-800";

    const renderedChildren = React.isValidElement(children) && typeof children.type !== 'string'
        ? React.cloneElement(children as React.ReactElement<{ accessMe?: boolean }>, { accessMe })
        : children;

    return (
        <div className={containerClasses}>
            <button
                className={`w-full flex justify-between items-center p-4 bg-gray-50 dark:bg-surface-800 focus:outline-none transition-colors hover:bg-gray-100 dark:hover:bg-surface-500 first:rounded-t-lg ${!isOpen && !isStandalone ? 'last:rounded-b-lg' : ''} ${isStandalone ? 'rounded-t-lg' : ''} ${isStandalone && !isOpen ? 'rounded-b-lg' : ''}`}
                onClick={onToggle}
                type="button"
            >
                <span className="font-medium text-gray-900 dark:text-gray-100">{title}</span>
                <svg className={`w-5 h-5 transition-transform text-gray-500 ${isOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7"></path>
                </svg>
            </button>
            {(isOpen || forceMount) && (
                <div className={`p-5 border-t border-gray-200 dark:border-surface-900 bg-white dark:bg-surface-900 ${isStandalone ? 'rounded-b-lg' : ''} ${!isOpen ? 'hidden' : ''}`}>
                    {renderedChildren}
                </div>
            )}
        </div>
    );
};

export default AccordionItem;
