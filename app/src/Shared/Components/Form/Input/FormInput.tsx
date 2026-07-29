import React from 'react';

interface FormInputProps {
    id?: string;
    label?: string;
    description?: string;
    type?: 'text' | 'email' | 'password' | 'number' | 'url' | 'date';
    value: string | number;
    onChange: (value: string) => void;
    placeholder?: string;
    required?: boolean;
    disabled?: boolean;
    autoComplete?: string;
    min?: number;
    max?: number;
    minLength?: number;
    readOnly?: boolean;
    error?: string;
    className?: string;
    inputClassName?: string;
    autoFocus?: boolean;
    // Right-aligned slot inside the input (e.g. a show/hide password
    // toggle) — absolutely positioned over the input, which gets
    // extra right padding to make room for it.
    trailing?: React.ReactNode;
}

const BASE = 'w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-400 focus:border-indigo-400 transition bg-white dark:bg-surface-900 text-gray-900 dark:text-white disabled:opacity-50 disabled:cursor-not-allowed';

export const FormInput: React.FC<FormInputProps> = ({
    id,
    label,
    description,
    type = 'text',
    value,
    onChange,
    placeholder,
    required,
    disabled,
    autoComplete,
    min,
    max,
    minLength,
    readOnly,
    error,
    className,
    inputClassName,
    autoFocus,
    trailing,
}) => {
    const reactId = React.useId();
    const inputId = id ?? reactId;
    return (
    <div className={className}>
        {label && (
            <label htmlFor={inputId} className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {label}
            </label>
        )}
        <div className="relative">
            <input
                id={inputId}
                type={type}
                value={value}
                onChange={e => onChange(e.target.value)}
                placeholder={placeholder}
                required={required}
                disabled={disabled}
                autoComplete={autoComplete}
                autoFocus={autoFocus}
                min={min}
                max={max}
                minLength={minLength}
                readOnly={readOnly}
                className={`${BASE} ${trailing ? 'pr-10' : ''} ${error ? 'border-red-400 dark:border-red-500' : 'border-gray-300 dark:border-gray-600'}${inputClassName ? ` ${inputClassName}` : ''}`}
            />
            {trailing && (
                <span className="absolute right-3 top-1/2 -translate-y-1/2">{trailing}</span>
            )}
        </div>
        {description && <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">{description}</p>}
        {error && <p className="text-xs text-red-600 dark:text-red-400 mt-0.5">{error}</p>}
    </div>
    );
};
