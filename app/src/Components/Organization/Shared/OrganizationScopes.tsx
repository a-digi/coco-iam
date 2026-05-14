import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { OrganizationScopesDropdown } from './OrganizationScopesDropdown';
import { InfoBadge } from '../../../Shared/Components/Badge/InfoBadge';
import { buildFilterQueryString } from '../../../config/data/resource/filters';

interface OrganizationScopesProps {
    entityId: string;
    resourceName: string;
    resourceKey: string;
}

/**
 * Manages scope assignments for organization users or organization groups.
 * Mirrors the admin Scopes partial but uses the organizations/ URL prefix and
 * only offers scopes under the organizations: root.
 */
export const OrganizationScopes: React.FC<OrganizationScopesProps> = ({ entityId, resourceName, resourceKey }) => {
    const { get, patch, post } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [selectedScopes, setSelectedScopes] = useState<string[]>([]);
    const [aclId, setAclId] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);

    const fetchAcl = React.useCallback(async () => {
        if (!entityId) return;
        setFetching(true);
        try {
            const queryString = buildFilterQueryString([{ field: resourceKey, operator: 'exact', value: entityId }]);
            const response = await get<{ message: unknown }>(`organizations/{res:${resourceName}}?${queryString}`);
            const message = response?.message || response;
            const acls = Array.isArray(message) ? message : (message ? [message] : []);

            if (acls.length > 0) {
                const first = acls[0] as { id: string; roles: string[] };
                setAclId(first.id);
                setSelectedScopes(Array.isArray(first.roles) ? first.roles : []);
            } else {
                setAclId(null);
                setSelectedScopes([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to load scopes';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setFetching(false);
        }
    }, [entityId, get, resourceName, resourceKey, errorMessage]);

    useEffect(() => {
        void fetchAcl();
    }, [fetchAcl]);

    const saveScopes = async (newScopes: string[]) => {
        setLoading(true);
        try {
            if (aclId) {
                await patch(`organizations/{res:${resourceName}}/{id:${aclId}}`, { roles: newScopes });
            } else {
                const response = await post<{ message?: { id: string }; id?: string }>(
                    `organizations/{res:${resourceName}}`,
                    { [resourceKey]: entityId, roles: newScopes, is_active: true }
                );
                const newId = response?.message?.id || response?.id;
                if (newId) setAclId(newId);
            }
            setSelectedScopes(newScopes);
            successMessage('Scopes updated successfully!');
        } catch (err: unknown) {
            let errorMsg = 'Failed to update scopes';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };

    const handleAddScope = async (scope: string) => {
        if (selectedScopes.includes(scope)) return;
        await saveScopes([...selectedScopes, scope]);
    };

    const handleRemoveScope = async (scope: string) => {
        await saveScopes(selectedScopes.filter(s => s !== scope));
    };

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading scopes...</div>;
    }

    return (
        <div className="space-y-4">
            <p className="text-sm text-gray-500">Assign fine-grained organization-scoped permissions.</p>

            <div className="flex flex-col gap-2 mb-4 items-start">
                {selectedScopes.map(scope => (
                    <InfoBadge
                        key={`direct-${scope}`}
                        label={scope}
                        onRemove={() => void handleRemoveScope(scope)}
                        disabled={loading}
                    />
                ))}

                {selectedScopes.length === 0 && (
                    <span className="text-sm text-gray-500 italic">No scopes assigned.</span>
                )}
            </div>

            <div className="pt-2">
                <OrganizationScopesDropdown
                    selectedValues={selectedScopes}
                    onChange={(val) => void handleAddScope(val)}
                />
            </div>
        </div>
    );
};

export default OrganizationScopes;
