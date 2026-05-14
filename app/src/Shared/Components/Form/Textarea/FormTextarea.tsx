import React from 'react';

interface FormTextareaProps {
    id?: string;
    label?: string;
    description?: string;
    value: string;
    onChange: (value: string) => void;
    placeholder?: string;
    required?: boolean;
    disabled?: boolean;
    rows?: number;
    error?: string;
    className?: string;
    textareaClassName?: string;
}

const BASE = 'w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:border-indigo-400 transition bg-white dark:bg-surface-900 text-gray-900 dark:text-white disabled:opacity-50 disabled:cursor-not-allowed';

export const FormTextarea: React.FC<FormTextareaProps> = ({
    id,
    label,
    description,
    value,
    onChange,
    placeholder,
    required,
    disabled,
    rows = 4,
    error,
    className,
    textareaClassName,
}) => {
    const reactId = React.useId();
    const textareaId = id ?? reactId;
    return (
    <div className={className}>
        {label && (
            <label htmlFor={textareaId} className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {label}
            </label>
        )}
        <textarea
            id={textareaId}
            value={value}
            onChange={e => onChange(e.target.value)}
            placeholder={placeholder}
            required={required}
            disabled={disabled}
            rows={rows}
            className={`${BASE} ${error ? 'border-red-400 dark:border-red-500' : 'border-gray-300 dark:border-gray-600'}${textareaClassName ? ` ${textareaClassName}` : ''}`}
        />
        {description && <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{description}</p>}
        {error && <p className="text-xs text-red-600 dark:text-red-400 mt-0.5">{error}</p>}
    </div>
    );
};
