import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface OrganizationUser {
    id: string;
    username: string;
    email: string;
    organizationId: string;
    createdAt: string;
    isActive: boolean;
    activationPending: boolean;
}

export const OrganizationUserSchema: Schema = {
    id: 'id',
    username: 'username',
    email: 'email',
    organizationId: 'organization_id',
    createdAt: 'created_at',
    isActive: 'is_active',
    activationPending: 'activation_pending',
};

export const OrganizationUserResource = 'res:organization_users';
