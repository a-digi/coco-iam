import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { OrganizationScopesDropdown } from '../../../Shared/OrganizationScopesDropdown';
import { InfoBadge } from '../../../../../Shared/Components/Badge/InfoBadge';
import { Modal } from '../../../../../Shared/Components/Modal/Modal';
import { Add } from '../../../../../Shared/Components/Button/Add';
import { buildFilterQueryString } from '../../../../../config/data/resource/filters';
import { OrganizationUserAclResource } from '../../../model/organizationUserAcl';

interface OrgDirectGrant {
    id: string;
    roles: string[];
    is_active: boolean;
    created_at: string;
}

interface GroupScopeGrant {
    group_id: string;
    group_name: string;
    roles: string[];
    is_active: boolean;
}

interface AppScopeGrant {
    application_id: string;
    client_id: string;
    roles: string[];
    is_active: boolean;
}

interface UserScopeView {
    user_id: string;
    direct: OrgDirectGrant | null;
    from_groups: GroupScopeGrant[];
    from_apps: AppScopeGrant[];
    effective_roles: string[];
}

interface OrganizationUserScopesProps {
    userId: string;
}

export const OrganizationUserScopes: React.FC<OrganizationUserScopesProps> = ({ userId }) => {
    const { get, patch, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [view, setView] = useState<UserScopeView | null>(null);
    const [fetching, setFetching] = useState(true);
    const [loading, setLoading] = useState(false);
    const [modalOpen, setModalOpen] = useState(false);

    const fetchScopes = useCallback(async () => {
        if (!userId) return;
        setFetching(true);
        try {
            const qs = buildFilterQueryString([{ field: 'user_id', operator: 'exact', value: userId }]);
            const response = await get<{ message?: UserScopeView }>(`organizations/{${OrganizationUserAclResource}}?${qs}`);
            const msg = response?.message as UserScopeView | undefined;
            if (msg && typeof msg === 'object' && 'user_id' in msg) {
                setView({
                    ...msg,
                    from_groups: msg.from_groups ?? [],
                    from_apps: msg.from_apps ?? [],
                    effective_roles: msg.effective_roles ?? [],
                });
            } else {
                setView({ user_id: userId, direct: null, from_groups: [], from_apps: [], effective_roles: [] });
            }
        } catch {
            errorMessage('Failed to load scopes');
            setView({ user_id: userId, direct: null, from_groups: [], from_apps: [], effective_roles: [] });
        } finally {
            setFetching(false);
        }
    }, [userId, get, errorMessage]);

    useEffect(() => { void fetchScopes(); }, [fetchScopes]);

    const saveDirectScopes = async (newRoles: string[]) => {
        if (!view) return;
        setLoading(true);
        try {
            if (view.direct?.id) {
                await patch(`organizations/{${OrganizationUserAclResource}}/{id:${view.direct.id}}`, { roles: newRoles });
            } else {
                await post(`organizations/{${OrganizationUserAclResource}}`, {
                    user_id: userId,
                    roles: newRoles,
                    is_active: true,
                });
            }
            successMessage('Scopes updated successfully!');
            await fetchScopes();
        } catch {
            errorMessage('Failed to update scopes');
        } finally {
            setLoading(false);
        }
    };

    const handleAdd = async (scope: string) => {
        const current = view?.direct?.roles ?? [];
        if (current.includes(scope)) return;
        await saveDirectScopes([...current, scope]);
        setModalOpen(false);
    };

    const handleRemove = async (scope: string) => {
        const current = view?.direct?.roles ?? [];
        await saveDirectScopes(current.filter(s => s !== scope));
    };

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading scopes...</div>;
    }

    const directRoles = view?.direct?.roles ?? [];
    const hasInherited = (view?.from_groups?.length ?? 0) > 0 || (view?.from_apps?.length ?? 0) > 0;

    return (
        <div className="space-y-6">

            {/* Direct org-level grant */}
            <div>
                <div className="flex items-center justify-between mb-2">
                    <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Direct scopes</h4>
                    <Add
                        label="Add Scope"
                        onClick={() => setModalOpen(true)}
                        disabled={loading}
                    />
                </div>
                <p className="text-xs text-gray-500 mb-3">Roles assigned directly to this user at the organization level.</p>
                <div className="flex flex-col gap-2 mb-3 items-start">
                    {directRoles.map(scope => (
                        <InfoBadge
                            key={scope}
                            label={scope}
                            onRemove={() => void handleRemove(scope)}
                            disabled={loading}
                        />
                    ))}
                    {directRoles.length === 0 && (
                        <span className="text-sm text-gray-500 italic">No direct scopes assigned.</span>
                    )}
                </div>

                <Modal
                    isOpen={modalOpen}
                    onClose={() => setModalOpen(false)}
                    title="Add Scope"
                    maxWidth="3xl"
                    closeOnBackdropClick={!loading}
                >
                    <OrganizationScopesDropdown
                        selectedValues={directRoles}
                        onChange={(val) => void handleAdd(val)}
                    />
                </Modal>
            </div>

            {/* Group inheritance */}
            {(view?.from_groups?.length ?? 0) > 0 && (
                <div>
                    <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Inherited from groups</h4>
                    <div className="space-y-3">
                        {view!.from_groups.map(g => (
                            <div key={g.group_id} className="rounded border border-gray-200 dark:border-gray-700 px-3 py-2">
                                <p className="text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">{g.group_name}</p>
                                <div className="flex flex-wrap gap-1">
                                    {g.roles.map(r => (
                                        <InfoBadge key={r} label={r} />
                                    ))}
                                    {g.roles.length === 0 && (
                                        <span className="text-xs text-gray-400 italic">No roles</span>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* App grants */}
            {(view?.from_apps?.length ?? 0) > 0 && (
                <div>
                    <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Inherited from applications</h4>
                    <div className="space-y-3">
                        {view!.from_apps.map(a => (
                            <div key={a.application_id} className="rounded border border-gray-200 dark:border-gray-700 px-3 py-2">
                                <p className="text-xs font-medium text-gray-600 dark:text-gray-400 mb-1">{a.client_id}</p>
                                <div className="flex flex-wrap gap-1">
                                    {a.roles.map(r => (
                                        <InfoBadge key={r} label={r} />
                                    ))}
                                    {a.roles.length === 0 && (
                                        <span className="text-xs text-gray-400 italic">No roles</span>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Effective roles summary */}
            {hasInherited && (
                <div>
                    <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Effective roles</h4>
                    <p className="text-xs text-gray-500 mb-2">Deduplicated union of all sources above.</p>
                    <div className="flex flex-wrap gap-1">
                        {(view?.effective_roles ?? []).map(r => (
                            <InfoBadge key={r} label={r} className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300" />
                        ))}
                        {(view?.effective_roles?.length ?? 0) === 0 && (
                            <span className="text-sm text-gray-500 italic">No effective roles.</span>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
};

export default OrganizationUserScopes;
