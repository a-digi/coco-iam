import React, { type HTMLAttributes } from 'react';

export interface WarningBadgeProps extends HTMLAttributes<HTMLSpanElement> {
    label?: React.ReactNode;
    onRemove?: () => void;
    disabled?: boolean;
}

export const WarningBadge: React.FC<WarningBadgeProps> = ({
    label,
    className = '',
    onRemove,
    disabled = false,
    children,
    ...props
}) => {
    return (
        <span
            className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-surface-900 dark:text-yellow-400 ${className}`}
            {...props}
        >
            {label || children}
            {onRemove && (
                <button
                    type="button"
                    onClick={(e) => {
                        e.stopPropagation();
                        onRemove();
                    }}
                    className="ml-1.5 flex-shrink-0 focus:outline-none opacity-70 hover:opacity-100 transition-opacity"
                    disabled={disabled}
                    aria-label="Remove"
                >
                    <svg className="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                    </svg>
                </button>
            )}
        </span>
    );
};

export default WarningBadge;
