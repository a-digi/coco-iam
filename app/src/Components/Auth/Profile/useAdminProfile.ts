import { useContext } from 'react';
import { AdminProfileContext } from './AdminProfileContext';

export function useAdminProfile() {
    const ctx = useContext(AdminProfileContext);
    if (!ctx) {
        throw new Error('useAdminProfile must be used within AdminProfileProvider');
    }
    return ctx;
}
