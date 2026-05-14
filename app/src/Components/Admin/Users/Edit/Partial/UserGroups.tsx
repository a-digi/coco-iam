import React, { useEffect, useState } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { DefaultBadge } from '../../../../../Shared/Components/Badge/DefaultBadge';
import { SubmitSmall } from '../../../../../Shared/Components/Button/SubmitSmall';
import { type Group, GroupSchema, AdminGroupResource } from '../../../Groups/model/group';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import { buildFilterQueryString } from '../../../../../config/data/resource/filters';
import { FormInput } from '../../../../../Shared/Components/Form';

import { type InheritedScopes } from '../../../../Auth/Scopes/Partials/Scopes';
import { type ScopeAccessAware } from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { useAuth } from '../../../../Auth/Guard/useAuth';

interface UserGroupsProps extends Partial<ScopeAccessAware> {
    entityId: string;
    onInheritedScopesChange?: (scopes: InheritedScopes[]) => void;
}

export const UserGroups: React.FC<UserGroupsProps> = ({ entityId, onInheritedScopesChange, accessMe }) => {
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const { authToken } = useAuth();

    const [assignedGroups, setAssignedGroups] = useState<Group[]>([]);
    const [availableGroups, setAvailableGroups] = useState<Group[]>([]);
    const [userMemberships, setUserMemberships] = useState<{ id: string; group_id: string }[]>([]);

    // UI state
    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('');

    const fetchData = React.useCallback(async () => {
        if (!entityId) return;

        if (accessMe && authToken?.user?.id !== entityId) {
            setFetching(false);

            return;
        }

        setFetching(true);
        try {
            if (accessMe) {
                // Fetch from the custom me endpoint which bypasses general admin ACL constraints
                const meRes = await get<unknown>(`admin/me/admin_groups`);
                const meData = (meRes as { message?: unknown })?.message || meRes;
                if ((meData as { groups?: unknown })?.groups) {
                    setAssignedGroups((meData as { groups: unknown }).groups as unknown as Group[]);
                }

                return;
            }

            // 1. Fetch user's memberships first
            let membersList: { id: string; group_id: string }[] = [];
            try {
                const qs = buildFilterQueryString([{ field: 'user_id', operator: 'exact', value: entityId }]);
                const membersResponse = await get<{ message?: unknown }>(`admin/{res:admin_group_members}?${qs}`);
                const members = membersResponse?.message || membersResponse || [];
                if (Array.isArray(members)) {
                    membersList = members as { id: string; group_id: string }[];
                    setUserMemberships(membersList);
                }
            } catch (err) {
                console.warn("Failed to fetch user group memberships", err);
            }

            // 2. Fetch each group detail
            const groupsList: Group[] = [];
            for (const member of membersList) {
                try {
                    const gRes = await get<{ message?: unknown }>(`admin/{${AdminGroupResource}}/{id:${member.group_id}}`);
                    const rawGroup = gRes?.message || gRes;
                    if (rawGroup) {
                        const mappedGroups = mapObjects(GroupSchema, [rawGroup]) as unknown as Group[];
                        if (mappedGroups.length > 0) {
                            groupsList.push(mappedGroups[0]);
                        }
                    }
                } catch (err) {
                    console.error("Failed to fetch group", member.group_id, err);
                }
            }
            setAssignedGroups(groupsList);
        } catch (err) {
            console.error("Failed to fetch user groups", err);
        } finally {
            setFetching(false);
        }
    }, [entityId, get, accessMe, authToken?.user?.id]);

    useEffect(() => {
        void fetchData();
    }, [fetchData]);

    useEffect(() => {
        const timer = setTimeout(() => {
            setDebouncedSearchQuery(searchQuery);
        }, 500);

        return () => clearTimeout(timer);
    }, [searchQuery]);

    // Fetch available groups based on search
    useEffect(() => {
        if (!debouncedSearchQuery || accessMe || (accessMe && authToken?.user?.id !== entityId)) {
            setAvailableGroups([]);
            return;
        }

        const fetchAvailable = async () => {
            try {
                const qs = buildFilterQueryString([{ field: 'title', operator: 'like', value: debouncedSearchQuery }]);
                const groupsResponse = await get<{ message?: unknown }>(`admin/{${AdminGroupResource}}?${qs}`);
                const groupsData = groupsResponse?.message || groupsResponse || [];

                if (Array.isArray(groupsData)) {
                    const mappedGroups = mapObjects(GroupSchema, groupsData) as unknown as Group[];

                    // Filter out already assigned groups
                    const assignedGroupIds = new Set(userMemberships.map(m => m.group_id));
                    const unassigned = mappedGroups.filter(g => !assignedGroupIds.has(g.id));
                    setAvailableGroups(unassigned);
                }
            } catch (err) {
                console.error("Failed to fetch available groups", err);
            }
        };

        void fetchAvailable();
    }, [debouncedSearchQuery, get, userMemberships, accessMe, authToken?.user?.id, entityId]);

    const handleAddGroup = async (groupId: string) => {
        setLoading(true);
        try {
            const response = await post<{ message?: { id: string }; id?: string }>(`admin/{res:admin_group_members}`, {
                user_id: entityId,
                group_id: groupId,
                is_active: true
            });
            const newMemberId = response?.message?.id || response?.id;

            if (newMemberId) {
                setUserMemberships(prev => [...prev, { id: newMemberId, group_id: groupId }]);

                // Add group locally from available to assigned
                const addedGroup = availableGroups.find(g => g.id === groupId);
                if (addedGroup) {
                    setAssignedGroups(prev => [...prev, addedGroup]);
                    // remove from available
                    setAvailableGroups(prev => prev.filter(g => g.id !== groupId));
                }

                successMessage('User added to group successfully!');
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to add user to group';
            if (err instanceof Error) {
                errorMsg = err.message || errorMsg;
            }
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };

    const handleRemoveGroup = async (membershipId: string) => {
        setLoading(true);
        try {
            await del(`admin/{res:admin_group_members}/{id:${membershipId}}`);
            setUserMemberships(prev => prev.filter(m => m.id !== membershipId));

            // Remove from local assigned groups state based on matching membership
            const memberToRemove = userMemberships.find(m => m.id === membershipId);
            if (memberToRemove) {
                setAssignedGroups(prev => prev.filter(g => g.id !== memberToRemove.group_id));
            }

            successMessage('User removed from group successfully!');
        } catch (err: unknown) {
            let errorMsg = 'Failed to remove user from group';
            if (err instanceof Error) {
                errorMsg = err.message || errorMsg;
            }
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    };



    // Fetch ACLs for assigned groups to populate inherited scopes
    useEffect(() => {
        if (!onInheritedScopesChange) return;

        const fetchInheritedScopes = async () => {
            if (accessMe && authToken?.user?.id !== entityId) {
                return;
            }

            if (accessMe) {
                // In context of UserMe, we can piggy-back off the initial me API call if we wanted
                // But for now, since we already bypassed it, we can just do a standalone call again
                // or just skip inherited scopes fetch if the backend endpoints aren't available to UserMe.
                try {
                    const meRes = await get<unknown>(`admin/me/admin_groups`);
                    const meData = (meRes as { message?: unknown })?.message || meRes;
                    if ((meData as { inherited_acl?: string[] })?.inherited_acl) {
                        onInheritedScopesChange([{
                            title: 'Inherited from Groups',
                            scopes: (meData as { inherited_acl: string[] }).inherited_acl
                        }]);
                    }
                } catch (e) {
                    console.debug("Failed finding inherited ACLs natively", e);
                }
                return;
            }

            const inherited: InheritedScopes[] = [];
            for (const group of assignedGroups) {
                try {
                    const response = await get<{ message?: unknown }>(`admin/{res:admin_group_acl}?group_id=${group.id}`);
                    const acls = response?.message;
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    const aclsArray = Array.isArray(acls) ? acls : (acls ? [acls as any] : []);

                    if (aclsArray.length > 0 && aclsArray[0].roles) {
                        inherited.push({
                            title: group.title,
                            description: group.groupDescription,
                            scopes: aclsArray[0].roles as string[]
                        });
                    }
                } catch (err) {
                    console.error("Failed to fetch group ACL for", group.id, err);
                }
            }
            onInheritedScopesChange(inherited);
        };

        void fetchInheritedScopes();
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [userMemberships, assignedGroups]); // Re-run when memberships or groups change

    if (accessMe && authToken?.user?.id !== entityId) {
        return <div className="text-sm text-red-500 italic py-2">You do not have permission to view or manage these groups for other users.</div>;
    }

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading groups...</div>;
    }

    return (
        <div className="space-y-4">
            <p className="text-sm text-gray-500">Manage the groups where the user is a part of.</p>

            <div className="flex flex-wrap gap-2 mb-4">
                {assignedGroups.map(group => {
                    const membership = userMemberships.find(m => m.group_id === group.id);
                    return (
                        <DefaultBadge
                            key={group.id}
                            label={group.title}
                            onRemove={accessMe ? undefined : (membership ? () => void handleRemoveGroup(membership.id) : undefined)}
                            disabled={loading || accessMe}
                        />
                    );
                })}
                {assignedGroups.length === 0 && (
                    <span className="text-sm text-gray-500 italic">No groups assigned.</span>
                )}
            </div>

            {!accessMe && debouncedSearchQuery && availableGroups.length === 0 && (
                <p className="text-sm text-gray-500 pt-2">No available groups found matching your search.</p>
            )}

            {!accessMe && (
                <div className="space-y-4">
                    <div className="relative">
                        <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                            <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                            </svg>
                        </div>
                        <FormInput
                            id="search"
                            value={searchQuery}
                            onChange={setSearchQuery}
                            placeholder="Search available groups by name..."
                            inputClassName="pl-10"
                        />
                    </div>

                    <div className="border border-gray-200 dark:border-surface-900 rounded-md overflow-hidden bg-white dark:bg-surface-800">
                        <div className="max-h-64 overflow-y-auto">
                            <table className="min-w-full divide-y divide-gray-200 dark:divide-surface-900">
                                <thead className="bg-gray-50 dark:bg-surface-900 sticky top-0 md:table-header-group hidden">
                                    <tr>
                                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Group Title</th>
                                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Type</th>
                                        <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Description</th>
                                        <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Action</th>
                                    </tr>
                                </thead>
                                <tbody className="bg-white dark:bg-surface-800 divide-y divide-gray-200 dark:divide-surface-900">
                                    {availableGroups.map((group) => (
                                        <tr key={group.id} className="hover:bg-gray-50 dark:hover:bg-surface-500 transition-colors flex flex-col md:table-row">
                                            <td className="px-6 py-4 md:whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100 flex items-center justify-between md:table-cell">
                                                <span className="md:hidden font-bold text-gray-500 uppercase text-xs mr-2">Title:</span>
                                                {group.title}
                                            </td>
                                            <td className="px-6 py-4 md:whitespace-nowrap text-sm text-gray-500 dark:text-gray-400 flex items-center justify-between md:table-cell">
                                                <span className="md:hidden font-bold text-gray-500 uppercase text-xs mr-2">Type:</span>
                                                <DefaultBadge label={group.groupType || 'user'} />
                                            </td>
                                            <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400 flex items-center justify-between md:table-cell">
                                                <span className="md:hidden font-bold text-gray-500 uppercase text-xs mr-2">Description:</span>
                                                {group.groupDescription}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium flex justify-end md:table-cell border-t md:border-t-0 border-gray-100 dark:border-surface-900 bg-gray-50 dark:bg-surface-900 md:bg-transparent">
                                                <SubmitSmall
                                                    onClick={() => void handleAddGroup(group.id)}
                                                    disabled={loading}
                                                >
                                                    Add
                                                </SubmitSmall>
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default UserGroups;
