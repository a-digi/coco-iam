import React, { type InputHTMLAttributes } from 'react';

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'onChange'> {
    checked?: boolean;
    onChange?: (checked: boolean) => void;
    label?: React.ReactNode;
    description?: React.ReactNode;
}

/**
 * Checkbox — project-standard checkbox styled to match Switch (indigo accent,
 * dark-mode variant, accessible focus ring). Mirrors Switch's API so call
 * sites can swap between the two without hooking into the underlying DOM
 * event shape.
 */
export const Checkbox: React.FC<CheckboxProps> = ({
    checked = false,
    onChange,
    label,
    description,
    disabled = false,
    id,
    className = '',
    ...props
}) => {
    const defaultId = React.useId();
    const checkboxId = id || defaultId;

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (onChange) onChange(e.target.checked);
    };

    return (
        <div className={`flex flex-col gap-1 ${className}`}>
            <label
                className={`flex items-center gap-3 w-fit ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
                htmlFor={checkboxId}
            >
                <span className="relative inline-flex items-center justify-center w-5 h-5">
                    <input
                        type="checkbox"
                        id={checkboxId}
                        className="peer sr-only"
                        checked={checked}
                        onChange={handleChange}
                        disabled={disabled}
                        {...props}
                    />
                    <span
                        aria-hidden="true"
                        className="block w-5 h-5 rounded-md border border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-900 transition-colors peer-checked:bg-indigo-600 peer-checked:border-indigo-600 dark:peer-checked:bg-indigo-500 dark:peer-checked:border-indigo-500 peer-focus-visible:ring-2 peer-focus-visible:ring-indigo-400 peer-focus-visible:ring-offset-2 dark:peer-focus-visible:ring-offset-surface-900"
                    />
                    <svg
                        aria-hidden="true"
                        viewBox="0 0 20 20"
                        className="pointer-events-none absolute w-3.5 h-3.5 text-white opacity-0 peer-checked:opacity-100 transition-opacity"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth={3}
                    >
                        <path strokeLinecap="round" strokeLinejoin="round" d="M4 10l4 4 8-8" />
                    </svg>
                </span>
                {label && (
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        {label}
                    </span>
                )}
            </label>
            {description && (
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 ml-[2rem]">
                    {description}
                </p>
            )}
        </div>
    );
};

export default Checkbox;
