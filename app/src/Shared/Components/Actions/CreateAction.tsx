import React from 'react';
import { ActionButton, type ActionButtonProps } from './ActionButton';

export const CreateAction: React.FC<Omit<ActionButtonProps, 'icon' | 'label'> & { label?: string }> = ({ label = 'Create', variant = 'primary', ...props }) => (
    <ActionButton
        icon={
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
        }
        label={label}
        variant={variant}
        {...props}
    />
);

export default CreateAction;
