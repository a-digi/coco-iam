import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface OrganizationGroupMember {
    id: string;
    groupId: string;
    userId: string;
    createdAt: string;
    isActive: boolean;
}

export const OrganizationGroupMemberSchema: Schema = {
    id: 'id',
    groupId: 'group_id',
    userId: 'user_id',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const OrganizationGroupMemberResource = 'res:organization_group_members';
