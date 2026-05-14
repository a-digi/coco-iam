import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface OrganizationUserAcl {
    id: string;
    userId: string;
    roles: string[];
    createdAt: string;
    isActive: boolean;
}

export const OrganizationUserAclSchema: Schema = {
    id: 'id',
    userId: 'user_id',
    roles: 'roles',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const OrganizationUserAclResource = 'res:organization_user_acl';
