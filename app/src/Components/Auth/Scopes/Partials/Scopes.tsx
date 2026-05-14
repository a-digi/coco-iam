import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Scopes as ScopesDropdown } from '../../../Auth/Scopes/Scopes';
import { InfoBadge } from '../../../../Shared/Components/Badge/InfoBadge';
import { DefaultBadge } from '../../../../Shared/Components/Badge/DefaultBadge';
import { buildFilterQueryString } from '../../../../config/data/resource/filters';
import { type ScopeAccessAware } from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { useAuth } from '../../../../Components/Auth/Guard/useAuth';

export interface InheritedScopes {
    scopes: string[];
    title: string;
    description?: string;
}

interface ScopesProps extends Partial<ScopeAccessAware> {
    entityId: string;
    resourceName: string;
    resourceKey: string;
    inheritedScopes?: InheritedScopes[];
}

export const Scopes: React.FC<ScopesProps> = ({ entityId, resourceName, resourceKey, inheritedScopes, accessMe }) => {
    const { get, patch, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const { authToken } = useAuth();

    const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
    const [userAclId, setUserAclId] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);

    const fetchAcl = React.useCallback(async () => {
        if (!entityId) return;

        if (accessMe && authToken?.user?.id !== entityId) {
            setFetching(false);
            return;
        }

        setFetching(true);
        try {
            if (accessMe) {
                const aclResponse = await get<unknown>(`admin/me/acl`);
                const aclMessage = (aclResponse as { message?: unknown })?.message || aclResponse;
                const directAcl = (aclMessage as { direct_acl?: string[] })?.direct_acl || [];
                setSelectedScopes(directAcl);
                setUserAclId(null);

                return;
            }

            const queryString = buildFilterQueryString([{ field: resourceKey, operator: 'exact', value: entityId }]);
            const aclResponse = await get<{ message: unknown }>(`admin/{res:${resourceName}}?${queryString}`);
            const aclMessage = aclResponse?.message || aclResponse;
            const acls = Array.isArray(aclMessage) ? aclMessage : (aclMessage ? [aclMessage] : []);
            if (acls.length > 0) {
                const firstAcl = acls[0] as { id: string; roles: string[] };
                setUserAclId(firstAcl.id);
                setSelectedScopes(firstAcl.roles || []);
            } else {
                setUserAclId(null);
                setSelectedScopes([]);
            }
        } catch (err) {
            console.error("Failed to fetch user ACL", err);
        } finally {
            setFetching(false);
        }
    }, [entityId, get, resourceName, resourceKey, accessMe, authToken?.user?.id]);

    useEffect(() => {
        void fetchAcl();
    }, [fetchAcl]);

    const handleAddScope = async (scope: string) => {
        const newScopes = [...selectedScopes, scope];
        await saveScopes(newScopes);
    };

    const handleRemoveScope = async (scope: string) => {
        const newScopes = selectedScopes.filter(s => s !== scope);
        await saveScopes(newScopes);
    };

    const saveScopes = async (newScopes: string[]) => {
        setLoading(true);
        try {
            if (userAclId) {
                await patch(`admin/{res:${resourceName}}/{id:${userAclId}}`, { roles: newScopes });
            } else {
                const response = await post<{ message?: { id: string }; id?: string }>(`admin/{res:${resourceName}}`, { [resourceKey]: entityId, roles: newScopes, is_active: true });
                const newId = response?.message?.id || response?.id;
                if (newId) {
                    setUserAclId(newId);
                }
            }
            setSelectedScopes(newScopes);
            successMessage('Scopes updated successfully!');
        } catch (err: unknown) {
            let errorMsg = 'Failed to update scopes';
            if (err instanceof Error) {
                errorMsg = err.message || errorMsg;
            }
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };

    if (accessMe && authToken?.user?.id !== entityId) {
        return <div className="text-sm text-red-500 italic py-2">You do not have permission to view or manage these scopes for other users.</div>;
    }

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading scopes...</div>;
    }

    const uniqueInheritedScopesElements: React.ReactNode[] = [];
    if (inheritedScopes) {
        const seenInherited = new Set<string>();
        inheritedScopes.forEach(group => {
            group.scopes?.forEach((scope, i) => {
                if (!selectedScopes.includes(scope) && !seenInherited.has(scope)) {
                    seenInherited.add(scope);
                    uniqueInheritedScopesElements.push(
                        <DefaultBadge
                            key={`inherited-${group.title}-${scope}-${i}`}
                            label={`${scope} (from ${group.title})`}
                            disabled={true}
                        />
                    );
                }
            });
        });
    }

    return (
        <div className="space-y-4">
            <p className="text-sm text-gray-500">Manage fine-grained access levels for this user.</p>

            <div className="flex flex-col gap-2 mb-4 items-start">
                {selectedScopes.map(scope => (
                    <InfoBadge
                        key={`direct-${scope}`}
                        label={scope}
                        onRemove={accessMe ? undefined : () => void handleRemoveScope(scope)}
                        disabled={loading}
                    />
                ))}

                {uniqueInheritedScopesElements}

                {selectedScopes.length === 0 && uniqueInheritedScopesElements.length === 0 && (
                    <span className="text-sm text-gray-500 italic">No scopes assigned or inherited.</span>
                )}
            </div>

            {!accessMe && (
                <div className="pt-2">
                    <ScopesDropdown
                        selectedValues={selectedScopes}
                        onChange={(val) => void handleAddScope(val)}
                    />
                </div>
            )}
        </div>
    );
};

export default Scopes;
