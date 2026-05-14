// AdminProfile mirrors the `GET /api/v1/admin/users/me` response.
// Kept separate from the auth-token shape so the profile fetch
// can fail (or be stale) without breaking the auth gate.
export interface AdminProfile {
    id: string;
    username: string;
    email: string;
    is_super_admin: boolean;
    is_active: boolean;
    created_at?: string;

    first_name: string;
    last_name: string;
    phone: string;
    locale: string;
    timezone: string;

    // avatar_url is either the public serve path or an empty
    // string. Empty → the UI renders its placeholder icon.
    avatar_url: string;
}
