import React from 'react';
import { Link } from 'react-router-dom';

export interface ActionButtonProps {
    icon: React.ReactNode;
    label: string;
    onClick?: () => void;
    to?: string;
    variant?: 'primary' | 'danger' | 'default';
    className?: string;
    disabled?: boolean;
}

export const ActionButton: React.FC<ActionButtonProps> = ({
    icon,
    label,
    onClick,
    to,
    variant = 'default',
    className = '',
    disabled = false
}) => {
    let colors = 'text-gray-500 hover:text-gray-900 bg-gray-50 hover:bg-gray-100 dark:bg-surface-800 dark:hover:bg-surface-700 dark:text-gray-400 dark:hover:text-gray-100 border border-gray-200 dark:border-surface-600';
    if (variant === 'danger') {
        colors = 'text-red-600 hover:text-red-900 bg-red-50 hover:bg-red-100 dark:bg-red-900/10 dark:hover:bg-red-900/30 dark:text-red-400 dark:hover:text-red-300 border border-red-200 dark:border-red-900/50';
    } else if (variant === 'primary') {
        colors = 'text-indigo-600 hover:text-indigo-900 bg-indigo-50 hover:bg-indigo-100 dark:bg-indigo-900/10 dark:hover:bg-indigo-900/30 dark:text-indigo-400 dark:hover:text-indigo-300 border border-indigo-200 dark:border-indigo-900/50';
    }

    const triggerClassName = `group relative inline-flex items-center justify-center p-2 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-indigo-500 shadow-sm ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'} ${colors} ${className}`;

    const inner = (
        <>
            <span className="sr-only">{label}</span>
            {icon}

            <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:block z-[90] whitespace-nowrap">
                <div className="bg-gray-900 text-white text-xs rounded py-1 px-2.5 shadow-lg">
                    {label}
                </div>
                <div className="w-0 h-0 border-l-[4px] border-r-[4px] border-t-[4px] border-l-transparent border-r-transparent border-t-gray-900 absolute left-1/2 -translate-x-1/2 top-full" />
            </div>
        </>
    );

    if (to && !disabled) {
        return (
            <Link to={to} className={triggerClassName}>
                {inner}
            </Link>
        );
    }

    return (
        <button
            type="button"
            onClick={onClick}
            disabled={disabled}
            className={triggerClassName}
        >
            {inner}
        </button>
    );
};

export default ActionButton;
