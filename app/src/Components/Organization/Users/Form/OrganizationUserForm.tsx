import React, { useEffect, useState } from 'react';

import { Switch } from '../../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../../Shared/Components/Button';
import { FormInput, FormSelect } from '../../../../Shared/Components/Form';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { buildFilterQueryString } from '../../../../config/data/resource/filters';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { WorkspaceResource, WorkspaceSchema, type Workspace } from '../../../Workspace/model/workspace';
import { ApplicationResource, ApplicationSchema, type Application } from '../../../Application/model/application';
import { flattenWorkspaceApps, type AppOption } from './flattenWorkspaceApps';

export interface OrganizationUserFormData {
    username: string;
    email: string;
    is_active: boolean;
    // redirect_application_id — only set on create. Optional: UUID of
    // the application the newly-activated user should land on after
    // they finish activation. Empty string → use the default /login.
    redirect_application_id: string;
}

interface OrganizationUserFormProps {
    initialData?: Partial<OrganizationUserFormData>;
    onSubmit: (data: OrganizationUserFormData) => Promise<void>;
    loading: boolean;
    submitLabel?: string;
    cancelTo: string;
    // When set, render the "Send to application after activation"
    // dropdown — used on the create path only. Omitted on edit because
    // activations are per-invite, not a persistent user attribute.
    organizationId?: string;
}

export const OrganizationUserForm: React.FC<OrganizationUserFormProps> = ({
    initialData,
    onSubmit,
    loading,
    submitLabel = 'Save',
    cancelTo,
    organizationId,
}) => {
    const [username, setUsername] = useState(initialData?.username || '');
    const [email, setEmail] = useState(initialData?.email || '');
    const [isActive, setIsActive] = useState(initialData?.is_active ?? true);
    const [redirectAppID, setRedirectAppID] = useState(initialData?.redirect_application_id || '');

    const [appOptions, setAppOptions] = useState<AppOption[]>([]);
    const [appsLoading, setAppsLoading] = useState(false);

    const { get } = useHttpClient();

    // Populate the application dropdown when the form is in create mode
    // (organizationId provided). Silently tolerates partial failures —
    // the field stays empty and the user simply can't pick a redirect.
    useEffect(() => {
        if (!organizationId) return;
        let cancelled = false;
        const load = async () => {
            setAppsLoading(true);
            try {
                const wsQs = buildFilterQueryString([
                    { field: 'organization_id', operator: 'exact', value: organizationId },
                ]);
                const wsResp = await get<{ message?: unknown }>(`workspaces/{${WorkspaceResource}}?${wsQs}`);
                const wsData = wsResp?.message || wsResp || [];
                if (!Array.isArray(wsData)) return;
                const workspaces = mapObjects(WorkspaceSchema, wsData as Record<string, unknown>[]) as unknown as Workspace[];
                if (workspaces.length === 0) {
                    if (!cancelled) setAppOptions([]);
                    return;
                }
                // Fetch apps for each workspace, then let the pure
                // flattener produce the final "ws › app" option list.
                // The I/O lives here; the mapping lives in a testable
                // helper.
                const appsByWs: Record<string, Application[]> = {};
                await Promise.all(
                    workspaces.map(async ws => {
                        const qs = buildFilterQueryString([
                            { field: 'workspace_id', operator: 'exact', value: ws.id },
                        ]);
                        try {
                            const resp = await get<{ message?: unknown }>(`applications/{${ApplicationResource}}?${qs}`);
                            const data = resp?.message || resp || [];
                            if (!Array.isArray(data)) {
                                appsByWs[ws.id] = [];
                                return;
                            }
                            const apps = mapObjects(ApplicationSchema, data as Record<string, unknown>[]) as unknown as Application[];
                            appsByWs[ws.id] = apps;
                        } catch {
                            appsByWs[ws.id] = [];
                        }
                    }),
                );
                if (cancelled) return;
                setAppOptions(flattenWorkspaceApps(workspaces, appsByWs));
            } catch {
                if (!cancelled) setAppOptions([]);
            } finally {
                if (!cancelled) setAppsLoading(false);
            }
        };
        void load();
        return () => {
            cancelled = true;
        };
    }, [organizationId, get]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        await onSubmit({
            username,
            email,
            is_active: isActive,
            redirect_application_id: redirectAppID,
        });
    };

    return (
        <form onSubmit={handleSubmit} className="space-y-6 mt-6">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <FormInput
                    id="username"
                    label="Username"
                    value={username}
                    onChange={setUsername}
                    required={!!organizationId}
                    disabled={!organizationId}
                    autoComplete="off"
                />
                <FormInput
                    id="email"
                    type="email"
                    label="Email"
                    value={email}
                    onChange={setEmail}
                    required
                    autoComplete="off"
                />
            </div>

            {organizationId && (
                <div>
                    <FormSelect
                        id="redirect_application_id"
                        label="Send to application after activation"
                        value={redirectAppID}
                        onChange={setRedirectAppID}
                        options={[
                            { label: appsLoading ? 'Loading applications…' : '— none (use default login) —', value: '' },
                            ...appOptions.map(o => ({ label: o.label, value: o.id })),
                        ]}
                    />
                    <p className="text-xs text-gray-500 mt-1">
                        Optional. When set, the activation email&rsquo;s &ldquo;Go to login&rdquo; button will point
                        at that application&rsquo;s login page instead of the admin login.
                    </p>
                </div>
            )}

            <div className="flex items-center space-x-6 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
                <Switch id="is_active" checked={isActive} onChange={setIsActive} label="Is Active" />
            </div>

            <div className="flex justify-start gap-4 items-center pt-4">
                <Submit loading={loading} label={submitLabel} />
                <Cancel to={cancelTo} />
            </div>
        </form>
    );
};
