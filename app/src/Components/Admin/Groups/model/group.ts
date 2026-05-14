import type { Schema } from '../../../../config/data/mapper/mapper.ts';

export interface Group {
    id: string;
    groupType: string;
    title: string;
    groupDescription: string;
    createdAt: string;
    isActive: boolean;
}

export const GroupSchema: Schema = {
    id: 'id',
    groupType: 'group_type',
    title: 'title',
    groupDescription: 'group_description',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export interface GroupRequestResponse {
    message?: Group[] | null,
    success?: boolean
}

export const AdminGroupResource = 'res:admin_groups';