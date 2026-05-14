import React from 'react';
import { Link } from 'react-router-dom';

interface PageHeadMetaProps {
    label: string;
    value: string;
    to?: string;
}

export const PageHeadMeta: React.FC<PageHeadMetaProps> = ({ label, value, to }) => (
    <div>
        <div className="text-xs font-medium uppercase tracking-wide text-indigo-600 dark:text-indigo-400 mb-0.5">
            {label}
        </div>
        {to ? (
            <Link to={to} className="text-xl font-semibold text-gray-900 dark:text-gray-100 truncate hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors">
                {value}
            </Link>
        ) : (
            <span className="text-xl font-semibold text-gray-900 dark:text-gray-100 truncate">
                {value}
            </span>
        )}
    </div>
);

export default PageHeadMeta;
