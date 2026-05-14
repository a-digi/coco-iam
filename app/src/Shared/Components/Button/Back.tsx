import React from 'react';
import { Link } from 'react-router-dom';

interface BackProps {
    to: string;
    label?: string;
    className?: string;
}

/**
 * Back is a subtle, inline navigation link intended to sit next to a
 * page Title. It pairs with forms that also have a Cancel button near
 * the Submit — the two answer the same "take me out of here" question,
 * but Back is prominent at the top so users don't have to scroll down
 * to find an escape hatch.
 */
export const Back: React.FC<BackProps> = ({ to, label = 'Back', className = '' }) => {
    return (
        <Link
            to={to}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium text-gray-600 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 rounded-md hover:bg-gray-100 dark:hover:bg-surface-800 transition-colors ${className}`}
        >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            {label}
        </Link>
    );
};

export default Back;
