import React, { type ButtonHTMLAttributes } from 'react';

export interface SubmitProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    loading?: boolean;
    loadingText?: string;
    label?: React.ReactNode;
    // Injected by `ScopeBasedComponentAccess` via React.cloneElement.
    // We don't use it here, but accept + discard it so it doesn't
    // leak onto the underlying <button> as a DOM attribute.
    accessMe?: boolean;
}

export const Submit: React.FC<SubmitProps> = ({
    loading = false,
    loadingText = 'Submitting...',
    label,
    className = '',
    children,
    disabled,
    accessMe: _accessMe,
    ...props
}) => {
    return (
        <button
            type="submit"
            disabled={loading || disabled}
            className={`cursor-pointer px-6 py-2.5 bg-surface-300 hover:bg-surface-700 text-white font-medium rounded-lg shadow-md transition disabled:opacity-60 disabled:cursor-not-allowed flex items-center justify-center ${className}`}
            {...props}
        >
            {loading && (
                <svg className="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path>
                </svg>
            )}
            {loading ? loadingText : (label || children)}
        </button>
    );
};

export default Submit;
