import type { Schema } from '../../../config/data/mapper/mapper.ts';

export interface ApplicationScope {
    id: string;
    applicationId: string;
    scopeId: string;
    description: string;
    // JSON array of opaque resource id strings, as delivered by the
    // API. UI code that edits this parses and serialises on the fly.
    resourceIds: string;
    createdAt: string;
    isActive: boolean;
}

export const ApplicationScopeSchema: Schema = {
    id: 'id',
    applicationId: 'application_id',
    scopeId: 'scope_id',
    description: 'description',
    resourceIds: 'resource_ids',
    createdAt: 'created_at',
    isActive: 'is_active',
};

export const ApplicationScopeResource = 'res:application_scopes';

// Matches the backend regex in ScopeIDFormat: letters or underscores per
// segment, separated by `:`.
export const SCOPE_ID_PATTERN = /^[a-zA-Z_]+(:[a-zA-Z_]+)*$/;

// parseResourceIds reads the string shape from the API into a usable
// array. Tolerates both a JSON-string payload and an already-parsed
// array — different API paths emit different shapes.
export function parseResourceIds(raw: unknown): string[] {
    if (Array.isArray(raw)) return raw.map(String);
    if (typeof raw !== 'string' || raw === '') return [];
    try {
        const parsed: unknown = JSON.parse(raw);
        return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch {
        return [];
    }
}
