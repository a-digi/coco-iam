import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface OrganizationGroup {
    id: string;
    groupType: string;
    title: string;
    groupDescription: string;
    organizationId: string;
    createdAt: string;
    isActive: boolean;
}

export const OrganizationGroupSchema: Schema = {
    id: 'id',
    groupType: 'group_type',
    title: 'title',
    groupDescription: 'group_description',
    organizationId: 'organization_id',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const OrganizationGroupResource = 'res:organization_groups';
