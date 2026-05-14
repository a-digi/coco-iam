import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface OrganizationGroupAcl {
    id: string;
    groupId: string;
    roles: string[];
    createdAt: string;
    isActive: boolean;
}

export const OrganizationGroupAclSchema: Schema = {
    id: 'id',
    groupId: 'group_id',
    roles: 'roles',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const OrganizationGroupAclResource = 'res:organization_group_acl';
