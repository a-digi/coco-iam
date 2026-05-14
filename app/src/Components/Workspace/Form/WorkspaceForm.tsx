import React, { useEffect, useMemo, useState } from 'react';

import { Switch } from '../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../Shared/Components/Button';
import { FormInput, FormSelect, FormTextarea, type FormSelectOption } from '../../../Shared/Components/Form';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { mapObjects } from '../../../config/data/mapper/mapper';
import {
    type Organization,
    OrganizationResource,
    OrganizationSchema,
} from '../model/organization';

export interface WorkspaceFormData {
    workspace_id: string;
    title: string;
    description: string;
    organization_id: string;
    is_active: boolean;
}

// URL-friendly slug: lowercase, alphanumerics + hyphens, ≤ 60 chars.
const slugify = (s: string): string =>
    s.toLowerCase()
        .trim()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .slice(0, 60);

interface WorkspaceFormProps {
    initialData?: Partial<WorkspaceFormData>;
    onSubmit: (data: WorkspaceFormData) => Promise<void>;
    loading: boolean;
    submitLabel?: string;
    // When set, the organization picker is hidden and this id is used
    // as the immutable organization_id for the workspace.
    lockedOrganizationId?: string;
    cancelTo?: string;
    // Shown as a read-only field when the organization is immutable
    // (e.g., edit mode). Falls back to the id if no title is provided.
    organizationDisplay?: { id: string; title?: string };
    // Hides the organization picker entirely; use when parent renders
    // its own read-only organization block above the form.
    hideOrganizationField?: boolean;
}

export const WorkspaceForm: React.FC<WorkspaceFormProps> = ({
    initialData,
    onSubmit,
    loading,
    submitLabel = 'Save',
    lockedOrganizationId,
    cancelTo = '/workspaces',
    organizationDisplay,
    hideOrganizationField = false,
}) => {
    const [title, setTitle] = useState(initialData?.title || '');
    const [workspaceIdInput, setWorkspaceIdInput] = useState(initialData?.workspace_id || '');
    const [workspaceIdTouched, setWorkspaceIdTouched] = useState(!!initialData?.workspace_id);
    const [description, setDescription] = useState(initialData?.description || '');
    const [organizationId, setOrganizationId] = useState(
        lockedOrganizationId || initialData?.organization_id || '',
    );
    const [isActive, setIsActive] = useState(initialData?.is_active ?? true);

    // Auto-derive workspace_id from title until the admin edits it.
    const effectiveWorkspaceId = workspaceIdTouched ? workspaceIdInput : slugify(title);

    const [organizations, setOrganizations] = useState<Organization[]>([]);
    const [organizationsLoading, setOrganizationsLoading] = useState(false);

    const { get } = useHttpClient();
    const { errorMessage } = useSnackBar();

    const canPickOrganization = !lockedOrganizationId && !hideOrganizationField;

    useEffect(() => {
        if (!canPickOrganization) return;
        let cancelled = false;
        const fetchOrganizations = async () => {
            setOrganizationsLoading(true);
            try {
                const response = await get<{ message?: unknown }>(`organizations/{${OrganizationResource}}`);
                const data = response?.message || response || [];
                if (Array.isArray(data) && !cancelled) {
                    const mapped = mapObjects(
                        OrganizationSchema,
                        data as Record<string, unknown>[],
                    ) as unknown as Organization[];
                    setOrganizations(mapped);
                }
            } catch (err: unknown) {
                if (cancelled) return;
                let msg = 'Failed to load organizations';
                if (err instanceof Error) msg = err.message || msg;
                errorMessage(msg);
            } finally {
                if (!cancelled) setOrganizationsLoading(false);
            }
        };
        void fetchOrganizations();
        return () => {
            cancelled = true;
        };
    }, [canPickOrganization, get, errorMessage]);

    const organizationOptions = useMemo<FormSelectOption[]>(
        () => organizations.map(o => ({ value: o.id, label: o.title })),
        [organizations],
    );

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        await onSubmit({
            workspace_id: effectiveWorkspaceId,
            title,
            description,
            organization_id: lockedOrganizationId || organizationId,
            is_active: isActive,
        });
    };

    const showReadOnlyOrg = !!organizationDisplay && !canPickOrganization;

    return (
        <form onSubmit={handleSubmit} className="space-y-6 mt-6">
            {showReadOnlyOrg && (
                <div className="p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                    <div className="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400 mb-1">
                        Organization
                    </div>
                    <div className="text-sm text-gray-900 dark:text-gray-100">
                        {organizationDisplay?.title || organizationDisplay?.id}
                    </div>
                </div>
            )}

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <FormInput
                    id="title"
                    label="Title"
                    value={title}
                    onChange={setTitle}
                    required
                    autoComplete="off"
                />
                <FormInput
                    id="workspace_id"
                    label="Workspace ID"
                    value={effectiveWorkspaceId}
                    onChange={v => { setWorkspaceIdInput(v); setWorkspaceIdTouched(true); }}
                    required
                    placeholder="auto-generated from title"
                    description="Unique within the organization. Lowercase, hyphens, no spaces."
                    inputClassName="font-mono text-sm"
                />
                {canPickOrganization && (
                    <FormSelect
                        id="organization_id"
                        label="Organization"
                        value={organizationId}
                        onChange={setOrganizationId}
                        options={organizationOptions}
                        placeholder={organizationsLoading ? 'Loading organizations...' : 'Select an organization'}
                        required
                        disabled={organizationsLoading}
                    />
                )}
                <FormTextarea
                    id="description"
                    label="Description"
                    value={description}
                    onChange={setDescription}
                    className="md:col-span-2"
                />
            </div>

            <div className="flex items-center space-x-6 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                <Switch
                    id="is_active"
                    checked={isActive}
                    onChange={setIsActive}
                    label="Is Active"
                />
            </div>

            <div className="flex justify-start gap-4 items-center pt-4">
                <ScopeBasedComponentAccess
                    requiredScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}
                >
                    <Submit
                        loading={loading}
                        label={submitLabel}
                    />
                </ScopeBasedComponentAccess>
                <Cancel to={cancelTo} />
            </div>
        </form>
    );
};
