import React, { type ButtonHTMLAttributes } from 'react';
import { SubmitSmall } from './SubmitSmall';

export interface RemoveProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    label?: string;
}

export const Remove: React.FC<RemoveProps> = ({
    label = 'Remove',
    className = '',
    ...props
}) => {
    return (
        <SubmitSmall
            className={`flex items-center justify-center gap-1.5 font-medium bg-red-50 text-red-600 hover:bg-red-100 dark:bg-red-900/30 dark:text-red-400 dark:hover:bg-red-900/50 border-transparent shadow-none ${className}`}
            {...props}
        >
            <svg
                className="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
            >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            <span>{label}</span>
        </SubmitSmall>
    );
};

export default Remove;
