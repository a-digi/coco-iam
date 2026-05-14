import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface Workspace {
    id: string;
    // Human-readable, admin-chosen identifier (the workspace's
    // analogue of Application.clientId). Unique per organization.
    workspaceId: string;
    title: string;
    description: string;
    organizationId: string;
    createdAt: string;
    isActive: boolean;
}

export const WorkspaceSchema: Schema = {
    id: 'id',
    workspaceId: 'workspace_id',
    title: 'title',
    description: 'description',
    organizationId: 'organization_id',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export interface WorkspaceRequestResponse {
    message?: Workspace[] | null,
    success?: boolean
}

export const WorkspaceResource = 'res:workspaces';

export interface WorkspaceStatsCounts {
    total: number;
    active: number;
    inactive: number;
}

export interface ApplicationBreakdown {
    id: string;
    title: string;
    user_count: number;
    top_scopes: string[];
}

export interface WorkspaceStats {
    organization_title: string;
    created_at: string;
    is_active: boolean;
    applications: WorkspaceStatsCounts;
    users: WorkspaceStatsCounts;
    applications_breakdown: ApplicationBreakdown[];
}
