import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { AdminProfileContext } from './AdminProfileContext';
import type { AdminProfile } from './types';
import { useAuth } from '../Guard/useAuth';
import { useHttpClient } from '../../../api/http/useHttpClient';

// AdminProfileProvider owns the cached /me response so every
// consumer (top-bar user menu, profile page, anywhere else that
// needs the name/avatar) reads from one source of truth. Sits
// below AuthProvider + HttpClientProvider in the tree.
//
// Fetches on mount when an auth token is already present (page
// reload) and whenever the token changes (login/logout).
export const AdminProfileProvider = ({ children }: { children: ReactNode }) => {
    const { authToken } = useAuth();
    const { get } = useHttpClient();
    const [profile, setProfile] = useState<AdminProfile | null>(null);
    const [loading, setLoading] = useState(false);

    const refresh = useCallback(async () => {
        if (!authToken) {
            setProfile(null);
            return;
        }
        setLoading(true);
        try {
            const resp = await get<{ message?: AdminProfile }>('admin/users/me');
            const body = resp?.message;
            setProfile(body ?? null);
        } catch {
            // Non-fatal — keep whatever we had cached, or null on
            // first load. Consumers fall back to `username` from
            // the JWT via profileDisplayName.
            setProfile(null);
        } finally {
            setLoading(false);
        }
    }, [authToken, get]);

    // Re-fetch whenever the auth token changes. Logging out nulls
    // the token, which nulls the profile via the early return in
    // `refresh`.
    useEffect(() => {
        void refresh();
    }, [refresh]);

    return (
        <AdminProfileContext.Provider value={{ profile, loading, refresh }}>
            {children}
        </AdminProfileContext.Provider>
    );
};

export default AdminProfileProvider;
