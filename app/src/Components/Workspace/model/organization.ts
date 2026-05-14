import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface Organization {
    id: string;
    title: string;
    description: string;
    createdAt: string;
    isActive: boolean;
}

export const OrganizationSchema: Schema = {
    id: 'id',
    title: 'title',
    description: 'description',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const OrganizationResource = 'res:organizations';
