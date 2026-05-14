import React, { useState } from 'react';

interface TooltipProps {
    content: React.ReactNode;
    children: React.ReactNode;
    position?: 'top' | 'bottom' | 'left' | 'right';
    className?: string;
}

const posClasses: Record<string, string> = {
    top:    'bottom-full left-1/2 -translate-x-1/2 mb-2',
    bottom: 'top-full left-1/2 -translate-x-1/2 mt-2',
    left:   'right-full top-1/2 -translate-y-1/2 mr-2',
    right:  'left-full top-1/2 -translate-y-1/2 ml-2',
};

export const Tooltip: React.FC<TooltipProps> = ({
    content,
    children,
    position = 'top',
    className = '',
}) => {
    const [visible, setVisible] = useState(false);

    return (
        <span
            className={`relative inline-flex items-center ${className}`}
            onMouseEnter={() => setVisible(true)}
            onMouseLeave={() => setVisible(false)}
        >
            {children}
            {visible && (
                <span
                    role="tooltip"
                    className={`absolute z-50 ${posClasses[position]} w-max max-w-[260px] px-2.5 py-1.5 rounded-lg bg-gray-900 dark:bg-surface-700 text-white text-xs leading-relaxed shadow-lg pointer-events-none whitespace-normal`}
                >
                    {content}
                </span>
            )}
        </span>
    );
};

export default Tooltip;
