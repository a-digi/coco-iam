import React, { useEffect, useState, useMemo, useCallback } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../Shared/Components/Button/SubmitSmall';
import { Add } from '../../../../Shared/Components/Button/Add';
import { DeleteAction } from '../../../../Shared/Components/Actions/DeleteAction';
import { Close } from '../../../../Shared/Components/Button/Close';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { type ResourceFilter, buildFilterQueryString } from '../../../../config/data/resource/filters';
import { type OrganizationUser, OrganizationUserSchema, OrganizationUserResource } from '../../../Organization/model/organizationUser';
import { WorkspaceResource, WorkspaceSchema, type Workspace } from '../../../Workspace/model/workspace';
import { type ApplicationScope, ApplicationScopeSchema, ApplicationScopeResource } from '../../model/applicationScope';
import { ApplicationUserAclResource } from '../../model/applicationUserAcl';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { FormInput } from '../../../../Shared/Components/Form';
import { EditAction } from '../../../../Shared/Components/Actions';
import { AclEditorModal, type AclEditorValue } from './AclEditorModal';

interface UsersProps {
    applicationId: string;
    workspaceId: string;
}

interface AclRow {
    id: string;
    user_id: string;
    roles: string[];
    grantable_roles: string[];
    resource_ids: Record<string, string[]>;
    user?: OrganizationUser;
}

interface AclRowWithUser extends AclRow {
    username: string;
    email: string;
}

export const Users: React.FC<UsersProps> = ({ applicationId, workspaceId }) => {
    const { get, post, patch, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [acls, setAcls] = useState<AclRow[]>([]);
    const [appScopes, setAppScopes] = useState<ApplicationScope[]>([]);
    const [eligibleUsers, setEligibleUsers] = useState<OrganizationUser[]>([]);
    const [fetching, setFetching] = useState(true);
    const [loading, setLoading] = useState(false);

    const [isPickerOpen, setIsPickerOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');

    const [editingAclId, setEditingAclId] = useState<string | null>(null);
    const [savingAcl, setSavingAcl] = useState(false);

    const [page, setPage] = useState(1);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);
    const PAGE_SIZE = 20;

    const fetchAcls = useCallback(async () => {
        const qs = buildFilterQueryString([{ field: 'application_id', operator: 'exact', value: applicationId }]);
        const response = await get<{ message?: unknown[] }>(`applications/{${ApplicationUserAclResource}}?${qs}`);
        const data = response?.message || response || [];
        if (!Array.isArray(data)) {
            setAcls([]);
            return;
        }
        const rows: AclRow[] = data.map(r => {
            const raw = r as {
                id: string;
                user_id: string;
                roles: string[];
                grantable_roles?: string[] | string;
                resource_ids?: Record<string, string[]> | string;
            };
            // grantable_roles and resource_ids come back either as a
            // parsed array/object or as a JSON string depending on
            // which repository path served the request. Tolerate both.
            const parsedGrantable = typeof raw.grantable_roles === 'string'
                ? safeJsonParseArray(raw.grantable_roles)
                : (Array.isArray(raw.grantable_roles) ? raw.grantable_roles : []);
            const parsedResource = typeof raw.resource_ids === 'string'
                ? safeJsonParseObject(raw.resource_ids)
                : (raw.resource_ids ?? {});
            return {
                id: String(raw.id),
                user_id: String(raw.user_id),
                roles: Array.isArray(raw.roles) ? raw.roles : [],
                grantable_roles: parsedGrantable,
                resource_ids: parsedResource,
            };
        });
        setAcls(rows);
    }, [applicationId, get]);

    const fetchAppScopes = useCallback(async () => {
        const qs = buildFilterQueryString([{ field: 'application_id', operator: 'exact', value: applicationId }]);
        const response = await get<{ message?: unknown }>(`applications/{${ApplicationScopeResource}}?${qs}`);
        const data = response?.message || response || [];
        if (Array.isArray(data)) {
            const mapped = mapObjects(ApplicationScopeSchema, data) as unknown as ApplicationScope[];
            setAppScopes(mapped);
        }
    }, [applicationId, get]);

    // Resolve users in the workspace's organization. A workspace belongs to
    // exactly one organization, so one lookup + one user query suffice.
    const fetchEligibleUsers = useCallback(async () => {
        const wsRes = await get<{ message?: unknown }>(`workspaces/{${WorkspaceResource}}/{id:${workspaceId}}`);
        const wsRaw = wsRes?.message || wsRes;
        if (!wsRaw) {
            setEligibleUsers([]);
            return;
        }
        const mappedWs = mapObjects(WorkspaceSchema, [wsRaw] as Record<string, unknown>[]) as unknown as Workspace[];
        const orgId = mappedWs[0]?.organizationId;
        if (!orgId) {
            setEligibleUsers([]);
            return;
        }

        const uQs = buildFilterQueryString([{ field: 'organization_id', operator: 'exact', value: orgId }]);
        const uRes = await get<{ message?: unknown }>(`organizations/{${OrganizationUserResource}}?${uQs}`);
        const uData = uRes?.message || uRes || [];
        if (Array.isArray(uData)) {
            const mapped = mapObjects(OrganizationUserSchema, uData) as unknown as OrganizationUser[];
            setEligibleUsers(mapped);
        } else {
            setEligibleUsers([]);
        }
    }, [workspaceId, get]);

    useEffect(() => {
        if (!applicationId || !workspaceId) return;
        let cancelled = false;
        (async () => {
            setFetching(true);
            try {
                await Promise.all([fetchAcls(), fetchAppScopes(), fetchEligibleUsers()]);
            } catch (err: unknown) {
                if (cancelled) return;
                let errorMsg = 'Failed to load application users';
                if (err instanceof Error) errorMsg = err.message || errorMsg;
                errorMessage(errorMsg);
            } finally {
                if (!cancelled) setFetching(false);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [applicationId, workspaceId, fetchAcls, fetchAppScopes, fetchEligibleUsers, errorMessage]);

    const usersById = useMemo(() => {
        const m = new Map<string, OrganizationUser>();
        eligibleUsers.forEach(u => m.set(u.id, u));
        return m;
    }, [eligibleUsers]);

    const assignedUserIds = useMemo(() => new Set(acls.map(a => a.user_id)), [acls]);

    const enrichedAcls = useMemo<AclRowWithUser[]>(() => acls.map(a => {
        const u = usersById.get(a.user_id);
        return {
            ...a,
            username: u?.username ?? a.user_id,
            email: u?.email ?? '',
        };
    }), [acls, usersById]);

    const filteredAcls = useMemo(() => {
        if (currentFilters.length === 0) return enrichedAcls;
        return enrichedAcls.filter(a => currentFilters.every(f => {
            const raw = a[f.field as keyof AclRowWithUser];
            if (raw == null) return false;
            const hay = String(raw).toLowerCase();
            const needle = String(f.value).toLowerCase();
            return f.operator === 'like' ? hay.includes(needle) : hay === needle;
        }));
    }, [enrichedAcls, currentFilters]);

    const pagedAcls = useMemo(
        () => filteredAcls.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
        [filteredAcls, page],
    );

    useEffect(() => {
        const totalPages = Math.max(1, Math.ceil(filteredAcls.length / PAGE_SIZE));
        if (page > totalPages) setPage(totalPages);
    }, [filteredAcls.length, page]);

    const filterData = useCallback((values: FilteredValue[]) => {
        const next: ResourceFilter[] = [];
        if (values.length > 0) {
            Object.entries(values[0]).forEach(([key, val]) => {
                if (val === undefined || val === null || val === '') return;
                next.push({ field: key, operator: 'like', value: String(val) });
            });
        }
        setCurrentFilters(next);
        setPage(1);
    }, []);

    const availableUsers = useMemo(() => {
        const filtered = eligibleUsers.filter(u => !assignedUserIds.has(u.id));
        if (!searchQuery) return filtered;
        const q = searchQuery.toLowerCase();
        return filtered.filter(u => u.username?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q));
    }, [eligibleUsers, assignedUserIds, searchQuery]);

    const handleAddUser = useCallback(async (user: OrganizationUser) => {
        setLoading(true);
        try {
            await post(`applications/{${ApplicationUserAclResource}}`, {
                application_id: applicationId,
                user_id: user.id,
                roles: [],
                is_active: true,
            });
            successMessage(`User ${user.username} added.`);
            setIsPickerOpen(false);
            setSearchQuery('');
            await fetchAcls();
        } catch (err: unknown) {
            let errorMsg = 'Failed to add user';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [post, applicationId, successMessage, errorMessage, fetchAcls]);

    const handleRemoveUser = useCallback(async (aclId: string, username: string) => {
        setLoading(true);
        try {
            await del(`applications/{${ApplicationUserAclResource}}/{id:${aclId}}`);
            successMessage(`User ${username} removed.`);
            await fetchAcls();
        } catch (err: unknown) {
            let errorMsg = 'Failed to remove user';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [del, successMessage, errorMessage, fetchAcls]);

    const handleSaveAcl = useCallback(async (aclId: string, value: AclEditorValue) => {
        setSavingAcl(true);
        try {
            await patch(
                `applications/{${ApplicationUserAclResource}}/{id:${aclId}}`,
                {
                    roles: value.roles,
                    grantable_roles: value.grantable_roles,
                    resource_ids: value.resource_ids,
                },
            );
            setAcls(prev => prev.map(r => r.id === aclId ? {
                ...r,
                roles: value.roles,
                grantable_roles: value.grantable_roles,
                resource_ids: value.resource_ids,
            } : r));
            successMessage('ACL updated.');
            setEditingAclId(null);
        } catch (err: unknown) {
            let msg = 'Failed to update ACL';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSavingAcl(false);
        }
    }, [patch, successMessage, errorMessage]);

    const columns = useMemo<TableColumn<AclRowWithUser>[]>(() => [
        {
            key: 'username',
            label: 'Username',
            render: (_v, row) => (
                <span className="font-medium text-gray-900 dark:text-gray-100">
                    {row.username || <span className="font-mono text-xs">{row.user_id}</span>}
                </span>
            ),
        },
        {
            key: 'email',
            label: 'Email',
            render: (_v, row) => <span className="text-gray-500 dark:text-gray-400">{row.email}</span>,
        },
        {
            key: 'roles',
            label: 'Scopes',
            render: (_v, row) => {
                if (appScopes.length === 0) {
                    return <span className="text-xs text-gray-500 italic">Define application scopes first.</span>;
                }
                const count = row.roles.length;
                return (
                    <span className="text-sm text-gray-700 dark:text-gray-300">
                        {count === 1 ? '1 scope' : `${count} scopes`}
                    </span>
                );
            },
        },
        {
            key: 'id',
            label: 'Actions',
            render: (_v, row) => (
                <div className="flex justify-end gap-2">
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsAclWrite, AppScopes.ApplicationsAcl, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <EditAction
                            onClick={() => setEditingAclId(row.id)}
                            disabled={loading}
                        />
                    </ScopeBasedComponentAccess>
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsAclDelete, AppScopes.ApplicationsAcl, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <DeleteAction
                            onClick={() => handleRemoveUser(row.id, row.username || row.user_id)}
                            disabled={loading}
                        />
                    </ScopeBasedComponentAccess>
                </div>
            ),
        },
    ], [appScopes, loading, handleRemoveUser]);

    const editingAcl = editingAclId ? acls.find(a => a.id === editingAclId) : null;
    const editingUsername = editingAcl
        ? (usersById.get(editingAcl.user_id)?.username ?? editingAcl.user_id)
        : '';

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading users...</div>;
    }

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-start">
                <div>
                    <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">Application Users</h4>
                    <p className="text-sm text-gray-500 mb-4">Users assigned to this application and their scope subsets. Only users from organizations linked to this workspace are eligible.</p>
                </div>
                {!isPickerOpen && (
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsAclWrite, AppScopes.ApplicationsAcl, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <Add onClick={() => setIsPickerOpen(true)} />
                    </ScopeBasedComponentAccess>
                )}
            </div>

            {isPickerOpen && (
                <div className="space-y-4 p-4 border border-gray-200 dark:border-surface-800 rounded-md bg-gray-50 dark:bg-surface-900">
                    <div className="flex justify-between items-center">
                        <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300">Add a user</h5>
                        <Close
                            onClick={() => {
                                setIsPickerOpen(false);
                                setSearchQuery('');
                            }}
                            label="Close"
                        />
                    </div>
                    <FormInput
                        id="search"
                        value={searchQuery}
                        onChange={setSearchQuery}
                        placeholder="Search by username or email..."
                    />
                    {availableUsers.length === 0 ? (
                        <p className="text-sm text-gray-500">No eligible users to add.</p>
                    ) : (
                        <div className="border border-gray-200 dark:border-surface-900 rounded-md overflow-hidden bg-white dark:bg-surface-800 max-h-64 overflow-y-auto">
                            <table className="min-w-full divide-y divide-gray-200 dark:divide-surface-900">
                                <tbody className="divide-y divide-gray-200 dark:divide-surface-900">
                                    {availableUsers.map((u) => (
                                        <tr key={u.id} className="hover:bg-gray-50 dark:hover:bg-surface-500">
                                            <td className="px-4 py-3 text-sm font-medium text-gray-900 dark:text-gray-100">{u.username}</td>
                                            <td className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">{u.email}</td>
                                            <td className="px-4 py-3 text-right">
                                                <SubmitSmall onClick={() => void handleAddUser(u)} disabled={loading}>Add</SubmitSmall>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}
                </div>
            )}

            {acls.length === 0 ? (
                <p className="text-sm text-gray-500 italic">No users assigned.</p>
            ) : (
                <TableView
                    columns={columns}
                    data={pagedAcls}
                    total={filteredAcls.length}
                    page={page}
                    pageSize={PAGE_SIZE}
                    onPageChange={setPage}
                    filters={{
                        username: { type: 'text', label: 'Username', placeholder: 'Search username' },
                        email: { type: 'text', label: 'Email', placeholder: 'Search email' },
                    }}
                    onFilterChange={filterData}
                    rowKey={(row) => row.id}
                    emptyText="No users match the current filter."
                />
            )}

            {editingAcl && (
                <AclEditorModal
                    isOpen={!!editingAclId}
                    onClose={() => setEditingAclId(null)}
                    scopes={appScopes.map(s => ({ id: s.id, scopeId: s.scopeId, description: s.description }))}
                    initial={{
                        roles: editingAcl.roles,
                        grantable_roles: editingAcl.grantable_roles,
                        resource_ids: editingAcl.resource_ids,
                    }}
                    title={`Edit ACL — ${editingUsername}`}
                    saving={savingAcl}
                    onSave={v => handleSaveAcl(editingAcl.id, v)}
                />
            )}
        </div>
    );
};

// -- helpers -----------------------------------------------------------

function safeJsonParseArray(raw: string): string[] {
    try {
        const v = JSON.parse(raw);
        return Array.isArray(v) ? v.map(String) : [];
    } catch { return []; }
}

function safeJsonParseObject(raw: string): Record<string, string[]> {
    try {
        const v = JSON.parse(raw);
        if (!v || typeof v !== 'object' || Array.isArray(v)) return {};
        const out: Record<string, string[]> = {};
        for (const [k, ids] of Object.entries(v as Record<string, unknown>)) {
            if (Array.isArray(ids)) out[k] = ids.map(String);
        }
        return out;
    } catch { return {}; }
}

export default Users;
