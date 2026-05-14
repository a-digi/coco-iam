import { createContext, useContext } from 'react';

export type SnackBarType = 'info' | 'error' | 'danger' | 'success';

export type SnackBarPosition = 'top-left' | 'top-center' | 'top-right' | 'bottom-left' | 'bottom-center' | 'bottom-right';

export interface SnackBarMessage {
    id: string;
    type: SnackBarType;
    message: string;
    duration?: number;
    position?: SnackBarPosition;
}

export interface SnackBarContextProps {
    infoMessage: (message: string, duration?: number, position?: SnackBarPosition) => void;
    dangerMessage: (message: string, duration?: number, position?: SnackBarPosition) => void;
    successMessage: (message: string, duration?: number, position?: SnackBarPosition) => void;
    errorMessage: (message: string, duration?: number, position?: SnackBarPosition) => void;
    removeMessage: (id: string) => void;
}

export const SnackBarContext = createContext<SnackBarContextProps | undefined>(undefined);

export const useSnackBar = (): SnackBarContextProps => {
    const context = useContext(SnackBarContext);
    if (!context) {
        throw new Error('useSnackBar must be used within a SnackBarProvider');
    }
    return context;
};
