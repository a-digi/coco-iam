import type {Schema} from '../../../../config/data/mapper/mapper.ts';
import type {Standard} from '../../../Standard/model/standard.ts';

export interface User {
  id: string;
  username: string;
  email: string;
  createdAt: string;
  isSuperAdmin: boolean;
  isActive: boolean;
}

export const StandardSchema: Schema = {
    id: 'id',
    username: 'username',
    email: 'email',
    createdAt: 'created_at',
    isActive: 'is_active',
    isSuperAdmin: 'is_super_admin',
};

export interface StandardRequestResponse {
    message?: Standard[]|null,
    success?: boolean
}