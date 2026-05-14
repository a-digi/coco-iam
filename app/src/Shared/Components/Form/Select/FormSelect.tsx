import React from 'react';

export interface FormSelectOption {
    value: string | number;
    label: string;
}

interface FormSelectProps {
    id?: string;
    label?: string;
    description?: string;
    value: string | number;
    onChange: (value: string) => void;
    options: FormSelectOption[];
    placeholder?: string;
    required?: boolean;
    disabled?: boolean;
    error?: string;
    className?: string;
    selectClassName?: string;
}

const BASE = 'w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:border-indigo-400 transition bg-white dark:bg-surface-900 text-gray-900 dark:text-white disabled:opacity-50 disabled:cursor-not-allowed';

export const FormSelect: React.FC<FormSelectProps> = ({
    id,
    label,
    description,
    value,
    onChange,
    options,
    placeholder,
    required,
    disabled,
    error,
    className,
    selectClassName,
}) => {
    const reactId = React.useId();
    const selectId = id ?? reactId;
    return (
    <div className={className}>
        {label && (
            <label htmlFor={selectId} className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {label}
            </label>
        )}
        <select
            id={selectId}
            value={value}
            onChange={e => onChange(e.target.value)}
            required={required}
            disabled={disabled}
            className={`${BASE} ${error ? 'border-red-400 dark:border-red-500' : 'border-gray-300 dark:border-gray-600'}${selectClassName ? ` ${selectClassName}` : ''}`}
        >
            {placeholder && (
                <option value="" disabled>
                    {placeholder}
                </option>
            )}
            {options.map(opt => (
                <option key={opt.value} value={opt.value}>
                    {opt.label}
                </option>
            ))}
        </select>
        {description && <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{description}</p>}
        {error && <p className="text-xs text-red-600 dark:text-red-400 mt-0.5">{error}</p>}
    </div>
    );
};
