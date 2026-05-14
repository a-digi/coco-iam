import React from 'react';
import { ActionButton, type ActionButtonProps } from './ActionButton';

export const LinkAction: React.FC<Omit<ActionButtonProps, 'icon' | 'label'> & { label?: string }> = ({ label = 'Link', ...props }) => (
    <ActionButton
        icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
            </svg>
        }
        label={label}
        {...props}
    />
);

export default LinkAction;
