import { createContext } from 'react';
import type { AdminProfile } from './types';

export interface AdminProfileContextType {
    profile: AdminProfile | null;
    // loading is true while the initial fetch is in flight.
    loading: boolean;
    // refresh forces a re-fetch from /api/v1/admin/users/me. Called
    // from the profile page after a successful PATCH/upload/delete
    // so the top-bar and avatar stay in sync.
    refresh: () => Promise<void>;
}

export const AdminProfileContext = createContext<AdminProfileContextType | undefined>(undefined);
