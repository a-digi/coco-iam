import React, { useState, useCallback } from 'react';
import type { ReactNode } from 'react';
import { SnackBarContext, type SnackBarMessage, type SnackBarType, type SnackBarPosition } from './SnackBarContext';
import { SnackBar } from './SnackBar';

const getPositionClass = (pos: SnackBarPosition) => {
    switch (pos) {
        case 'top-left': return 'top-4 left-4 items-start flex-col';
        case 'top-center': return 'top-4 left-1/2 -translate-x-1/2 items-center flex-col';
        case 'top-right': return 'top-4 right-4 items-end flex-col';
        case 'bottom-left': return 'bottom-4 left-4 items-start flex-col-reverse justify-end';
        case 'bottom-center': return 'bottom-4 left-1/2 -translate-x-1/2 items-center flex-col-reverse justify-end';
        case 'bottom-right': return 'bottom-4 right-4 items-end flex-col-reverse justify-end';
        default: return 'top-4 left-1/2 -translate-x-1/2 items-center flex-col';
    }
};

export const SnackBarProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
    const [snacks, setSnacks] = useState<SnackBarMessage[]>([]);

    const showSnackBar = useCallback((message: string, type: SnackBarType = 'info', duration: number = 5000, position: SnackBarPosition = 'top-center') => {
        const id = Math.random().toString(36).substring(2, 9) + Date.now().toString(36);
        setSnacks(prev => [...prev, { id, message, type, duration, position }]);
    }, []);

    const infoMessage = useCallback((message: string, duration?: number, position?: SnackBarPosition) => showSnackBar(message, 'info', duration, position), [showSnackBar]);
    const dangerMessage = useCallback((message: string, duration?: number, position?: SnackBarPosition) => showSnackBar(message, 'danger', duration, position), [showSnackBar]);
    const successMessage = useCallback((message: string, duration?: number, position?: SnackBarPosition) => showSnackBar(message, 'success', duration, position), [showSnackBar]);
    const errorMessage = useCallback((message: string, duration?: number, position?: SnackBarPosition) => showSnackBar(message, 'error', duration, position), [showSnackBar]);

    const removeMessage = useCallback((id: string) => {
        setSnacks(prev => prev.filter(snack => snack.id !== id));
    }, []);

    // Group snacks by position
    const groupedSnacks = snacks.reduce((acc, snack) => {
        const pos = snack.position || 'top-center';
        if (!acc[pos]) acc[pos] = [];
        acc[pos].push(snack);
        return acc;
    }, {} as Record<string, SnackBarMessage[]>);

    return (
        <SnackBarContext.Provider value={{ infoMessage, dangerMessage, successMessage, errorMessage, removeMessage }}>
            {children}
            {/* SnackBar Containers */}
            {Object.entries(groupedSnacks).map(([pos, posSnacks]) => (
                <div key={pos} className={`fixed z-50 flex pointer-events-none gap-2 ${getPositionClass(pos as SnackBarPosition)}`}>
                    {posSnacks.map(snack => (
                        <div key={snack.id} className="pointer-events-auto w-auto max-w-sm shrink-0">
                            <SnackBar snack={snack} onRemove={removeMessage} />
                        </div>
                    ))}
                </div>
            ))}
        </SnackBarContext.Provider>
    );
};
