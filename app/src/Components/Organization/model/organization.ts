import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface Organization {
    id: string;
    // Globally-unique human-readable identifier (the organization's
    // analogue of Workspace.workspaceId and Application.clientId).
    organizationId: string;
    title: string;
    description: string;
    createdAt: string;
    isActive: boolean;
}

export const OrganizationSchema: Schema = {
    id: 'id',
    organizationId: 'organization_id',
    title: 'title',
    description: 'description',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export interface OrganizationRequestResponse {
    message?: Organization[] | null,
    success?: boolean
}

export const OrganizationResource = 'res:organizations';
