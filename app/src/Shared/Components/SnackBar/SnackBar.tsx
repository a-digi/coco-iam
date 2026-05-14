import React, { useEffect } from 'react';
import type { SnackBarMessage, SnackBarType } from './SnackBarContext';

interface SnackBarProps {
    snack: SnackBarMessage;
    onRemove: (id: string) => void;
}

const SnackBarStyles: Record<SnackBarType, string> = {
    success: 'bg-green-600 text-white border-green-700',
    error: 'bg-red-600 text-white border-red-700',
    danger: 'bg-orange-600 text-white border-orange-700',
    info: 'bg-blue-600 text-white border-blue-700',
};

const SnackBarIcons: Record<SnackBarType, React.ReactNode> = {
    success: (
        <svg className="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7"></path>
        </svg>
    ),
    error: (
        <svg className="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
        </svg>
    ),
    danger: (
        <svg className="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
        </svg>
    ),
    info: (
        <svg className="w-5 h-5 mr-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
        </svg>
    ),
};

export const SnackBar: React.FC<SnackBarProps> = ({ snack, onRemove }) => {
    const duration = snack.duration ?? 5000;

    useEffect(() => {
        if (duration > 0) {
            const timer = setTimeout(() => {
                onRemove(snack.id);
            }, duration);
            return () => clearTimeout(timer);
        }
    }, [duration, snack.id, onRemove]);

    return (
        <div
            className={`flex items-center p-4 mb-3 rounded-lg shadow-lg border transform transition-all duration-300 ease-in-out ${SnackBarStyles[
                snack.type
            ]}`}
            role="alert"
        >
            {SnackBarIcons[snack.type]}
            <span className="flex-1 font-medium text-sm">{snack.message}</span>
            <button
                type="button"
                className="ml-4 text-white hover:text-gray-200 focus:outline-none transition-colors"
                onClick={() => onRemove(snack.id)}
                aria-label="Close"
            >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
            </button>
        </div>
    );
};
