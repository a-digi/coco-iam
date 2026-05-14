export interface EmailAccount {
    id: string;
    name: string;
    host: string;
    port: number;
    username: string;
    /** Always empty in GET responses. Send a non-empty value to update. */
    password: string;
    from_name: string;
    from_email: string;
    use_tls: boolean;
    is_active: boolean;
    created_at: string;
    updated_at: string;
}

export interface EmailAccountPatch {
    host?: string;
    port?: number;
    username?: string;
    password?: string;
    from_name?: string;
    from_email?: string;
    use_tls?: boolean;
}

/** Shared with the backend. Lowercase letters, digits, _ or -, start with letter. */
export const ACCOUNT_NAME_PATTERN = /^[a-z][a-z0-9_-]*$/;

export const EMPTY_ACCOUNT: EmailAccount = {
    id: '',
    name: '',
    host: '',
    port: 587,
    username: '',
    password: '',
    from_name: '',
    from_email: '',
    use_tls: false,
    is_active: false,
    created_at: '',
    updated_at: '',
};
