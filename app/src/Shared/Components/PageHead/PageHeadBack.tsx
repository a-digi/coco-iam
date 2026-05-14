import React from 'react';
import { Link } from 'react-router-dom';

interface PageHeadBackProps {
    to: string;
    label?: string;
}

export const PageHeadBack: React.FC<PageHeadBackProps> = ({ to, label = 'Back' }) => (
    <Link
        to={to}
        aria-label={label}
        title={label}
        className="group inline-flex items-center gap-1.5 text-xs text-gray-500 hover:text-indigo-600 dark:text-gray-400 dark:hover:text-indigo-400 transition-colors mb-3"
    >
        <svg
            className="w-3.5 h-3.5 transition-transform group-hover:-translate-x-0.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.25}
            aria-hidden="true"
        >
            <path strokeLinecap="round" strokeLinejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
        </svg>
        {label}
    </Link>
);

export default PageHeadBack;
