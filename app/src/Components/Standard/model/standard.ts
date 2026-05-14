import type {Schema} from '../../../config/data/mapper/mapper.ts';

export interface Standard {
    id?: string,
    filePath?: string,
    fileHash?: string,
    createdAt: string,
    title: string,
    version: string
}

export interface StandardRequestResponse {
    message?: Standard[]|null,
    success?: boolean
}

export const StandardSchema: Schema = {
    id: 'id',
    filePath: 'file_path',
    fileHash: 'file_hash',
    createdAt: 'created_at',
    isActive: 'is_active',
    title: 'title',
    version: 'version'
};
