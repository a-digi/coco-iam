import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface Application {
    id: string;
    workspaceId: string;
    clientId: string;
    title: string;
    description: string;
    createdAt: string;
    isActive: boolean;
    allowRecovery: boolean;
    allowRegistration: boolean;
}

export const ApplicationSchema: Schema = {
    id: 'id',
    workspaceId: 'workspace_id',
    clientId: 'client_id',
    title: 'title',
    description: 'description',
    createdAt: 'created_at',
    isActive: 'is_active',
    allowRecovery: 'allow_recovery',
    allowRegistration: 'allow_registration',
};

export const ApplicationResource = 'res:applications';
