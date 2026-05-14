import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface ApplicationUserAcl {
    id: string;
    applicationId: string;
    userId: string;
    roles: string[];
    createdAt: string;
    isActive: boolean;
}

export const ApplicationUserAclSchema: Schema = {
    id: 'id',
    applicationId: 'application_id',
    userId: 'user_id',
    roles: 'roles',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const ApplicationUserAclResource = 'res:application_user_acl';
