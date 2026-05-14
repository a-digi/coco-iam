import React, { type InputHTMLAttributes } from 'react';

export interface SwitchProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'onChange'> {
    checked?: boolean;
    onChange?: (checked: boolean) => void;
    label?: string;
    description?: string;
}

export const Switch: React.FC<SwitchProps> = ({
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
    const switchId = id || defaultId;

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        if (onChange) {
            onChange(e.target.checked);
        }
    };

    return (
        <div className={`flex flex-col gap-1 ${className}`}>
            <label
                className={`flex items-center cursor-pointer gap-3 w-fit ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
                htmlFor={switchId}
            >
                <div className="relative flex items-center">
                    <input
                        type="checkbox"
                        id={switchId}
                        className="sr-only peer"
                        checked={checked}
                        onChange={handleChange}
                        disabled={disabled}
                        {...props}
                    />
                    <div className="block bg-gray-300 dark:bg-surface-700 w-11 h-6 rounded-full transition-colors peer-checked:bg-indigo-600 dark:peer-checked:bg-indigo-500 peer-focus:ring-2 peer-focus:ring-indigo-400 peer-focus:ring-offset-2 dark:peer-focus:ring-offset-surface-900 border border-transparent dark:border-surface-600"></div>
                    <div className="absolute left-[2px] top-[2px] bg-white w-5 h-5 rounded-full transition-transform peer-checked:translate-x-full shadow-sm"></div>
                </div>
                {label && (
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                        {label}
                    </span>
                )}
            </label>
            {description && (
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 ml-[3.25rem]">
                    {description}
                </p>
            )}
        </div>
    );
};

export default Switch;
