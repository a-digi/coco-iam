import React, { type ButtonHTMLAttributes } from 'react';

export interface SubmitSmallProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    label?: React.ReactNode;
    // Swallowed to keep it off the underlying DOM. See Submit.tsx.
    accessMe?: boolean;
}

export const SubmitSmall: React.FC<SubmitSmallProps> = ({
    label,
    className = '',
    children,
    type = 'button',
    accessMe: _accessMe,
    ...props
}) => {
    return (
        <button
            type={type}
            className={`cursor-pointer text-surface-500 hover:text-surface-900 dark:text-gray-200 dark:hover:text-gray-100 bg-gray-50 hover:bg-gray-100 dark:bg-surface-900/30 dark:hover:bg-surface-900/50 px-3 py-1 rounded-md transition-colors ${className}`}
            {...props}
        >
            {label || children}
        </button>
    );
};

export default SubmitSmall;
