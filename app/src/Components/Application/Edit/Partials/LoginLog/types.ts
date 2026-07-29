export interface ApplicationLoginAttempt {
    id: string;
    application_user_id?: string;
    username: string;
    success: boolean;
    failure_reason?: string;
    ip: string;
    user_agent?: string;
    created_at: string;
}

export interface ApplicationLoginAttemptListResponse {
    attempts: ApplicationLoginAttempt[];
    total: number;
    limit: number;
    offset: number;
}

export interface ArchiveSummary {
    id: string;
    started_at: string;
    archived_at: string;
    row_count: number;
    size_bytes: number;
}

export interface ArchiveListResponse {
    archives: ArchiveSummary[];
    total: number;
    limit: number;
    offset: number;
}
