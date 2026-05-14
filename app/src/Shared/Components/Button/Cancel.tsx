import React from 'react';
import { Link, type LinkProps } from 'react-router-dom';

type CancelLinkProps = Omit<LinkProps, 'to'> & { to: string };
type CancelButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & { to?: never };

export type CancelProps = (CancelLinkProps | CancelButtonProps) & {
    label?: React.ReactNode;
    // Swallowed to keep it off the underlying DOM. See Submit.tsx.
    accessMe?: boolean;
};

export const Cancel: React.FC<CancelProps> = ({
    label = 'Cancel',
    className = '',
    children,
    accessMe: _accessMe,
    ...props
}) => {
    const baseClassName = `px-6 py-2.5 bg-orange-500 dark:bg-orange-500 text-white font-medium rounded-lg shadow-sm hover:bg-orange-600 dark:hover:bg-orange-600 transition flex items-center justify-center ${className} ${'disabled' in props && props.disabled ? 'opacity-50 cursor-not-allowed' : ''}`;

    if ('to' in props && props.to) {
        return (
            <Link
                className={baseClassName}
                {...(props as CancelLinkProps)}
            >
                {label || children}
            </Link>
        );
    }

    return (
        <button
            type="button"
            className={baseClassName}
            {...(props as CancelButtonProps)}
        >
            {label || children}
        </button>
    );
};

export default Cancel;
