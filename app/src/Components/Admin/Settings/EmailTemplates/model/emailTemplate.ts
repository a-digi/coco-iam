import type { Schema } from '../../../../../config/data/mapper/mapper.ts';

export interface EmailTemplate {
    id: string;
    name: string;
    description: string;
    subject: string;
    textBody: string;
    htmlBody: string;
    isActive: boolean;
    createdAt: string;
    updatedAt: string;
}

export const EmailTemplateSchema: Schema = {
    id: 'id',
    name: 'name',
    description: 'description',
    subject: 'subject',
    textBody: 'text_body',
    htmlBody: 'html_body',
    isActive: 'is_active',
    createdAt: 'created_at',
    updatedAt: 'updated_at',
};

// Must mirror the backend regex in api/src/mail/template/model.go.
// Lowercase letters, digits, underscores and hyphens; must start with a letter.
export const TEMPLATE_NAME_PATTERN = /^[a-z][a-z0-9_-]*$/;
