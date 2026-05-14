import React, { type ButtonHTMLAttributes } from 'react';
import { SubmitSmall } from './SubmitSmall';

export interface CloseProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    label?: string;
}

export const Close: React.FC<CloseProps> = ({
    label = '',
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
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
            {label && <span>{label}</span>}
        </SubmitSmall>
    );
};

export default Close;
