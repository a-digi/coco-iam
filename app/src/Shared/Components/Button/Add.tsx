import React, { type ButtonHTMLAttributes } from 'react';
import { SubmitSmall } from './SubmitSmall';

export interface AddProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    label?: string;
}

export const Add: React.FC<AddProps> = ({
    label = 'Add',
    className = '',
    ...props
}) => {
    return (
        <SubmitSmall
            className={`flex items-center justify-center gap-1.5 font-medium ${className}`}
            {...props}
        >
            <svg
                className="w-4 h-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
            >
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4v16m8-8H4" />
            </svg>
            <span>{label}</span>
        </SubmitSmall>
    );
};

export default Add;
