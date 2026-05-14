import React, { useEffect, useState, useMemo } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../../Shared/Components/Button/SubmitSmall';
import { Add } from '../../../../../Shared/Components/Button/Add';
import { DeleteAction } from '../../../../../Shared/Components/Actions/DeleteAction';
import { Close } from '../../../../../Shared/Components/Button/Close';
import { type OrganizationUser, OrganizationUserSchema, OrganizationUserResource } from '../../../model/organizationUser';
import { OrganizationGroupMemberResource } from '../../../model/organizationGroupMember';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import TableView, { type FilteredValue } from '../../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import { FormInput } from '../../../../../Shared/Components/Form';
import { type ResourceFilter, buildFilterQueryString } from '../../../../../config/data/resource/filters';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';

interface GroupMembersProps {
    organizationId: string;
    groupId: string;
}

interface Membership {
    id: string;
    user_id: string;
}

export const GroupMembers: React.FC<GroupMembersProps> = ({ organizationId, groupId }) => {
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [assignedUsers, setAssignedUsers] = useState<OrganizationUser[]>([]);
    const [availableUsers, setAvailableUsers] = useState<OrganizationUser[]>([]);
    const [memberships, setMemberships] = useState<Membership[]>([]);

    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

    const [isSearchVisible, setIsSearchVisible] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');
    const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('');

    const fetchMemberships = React.useCallback(async () => {
        if (!groupId) return;
        setFetching(true);
        try {
            const qs = buildFilterQueryString([{ field: 'group_id', operator: 'exact', value: groupId }]);
            const response = await get<{ message?: unknown[] }>(`organizations/{${OrganizationGroupMemberResource}}?${qs}`);
            const data = response?.message || response || [];

            if (Array.isArray(data)) {
                const membersList: Membership[] = data.map((m) => {
                    const raw = m as { id: string; user_id: string };
                    return { id: String(raw.id), user_id: String(raw.user_id) };
                });
                setMemberships(membersList);

                const rawUsers = data
                    .map((m) => (m as { user?: unknown }).user)
                    .filter(Boolean);
                if (rawUsers.length > 0) {
                    const mapped = mapObjects(OrganizationUserSchema, rawUsers as Record<string, unknown>[]) as unknown as OrganizationUser[];
                    setAssignedUsers(mapped);
                } else {
                    setAssignedUsers([]);
                }
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to load group members';
            if (err instanceof Error) {
                errorMsg = err.message || errorMsg;
            }
            errorMessage(errorMsg);
        } finally {
            setFetching(false);
        }
    }, [groupId, get, errorMessage]);

    useEffect(() => {
        void fetchMemberships();
    }, [fetchMemberships]);

    useEffect(() => {
        const timer = setTimeout(() => setDebouncedSearchQuery(searchQuery), 500);
        return () => clearTimeout(timer);
    }, [searchQuery]);

    useEffect(() => {
        if (!debouncedSearchQuery) {
            setAvailableUsers([]);
            return;
        }

        const fetchAvailable = async () => {
            try {
                const qs = buildFilterQueryString([{ field: 'organization_id', operator: 'exact', value: organizationId }]);
                const response = await get<{ message?: unknown }>(`organizations/{${OrganizationUserResource}}?${qs}`);
                const data = response?.message || response || [];
                if (Array.isArray(data)) {
                    const mapped = mapObjects(OrganizationUserSchema, data) as unknown as OrganizationUser[];
                    const assignedIds = new Set(memberships.map(m => m.user_id));
                    const available = mapped.filter(u => !assignedIds.has(u.id));
                    const query = debouncedSearchQuery.toLowerCase();
                    const filtered = available.filter(u =>
                        u.username?.toLowerCase().includes(query) ||
                        u.email?.toLowerCase().includes(query)
                    );
                    setAvailableUsers(filtered);
                }
            } catch (err: unknown) {
                let errorMsg = 'Failed to fetch available users';
                if (err instanceof Error) {
                    errorMsg = err.message || errorMsg;
                }
                errorMessage(errorMsg);
            }
        };

        void fetchAvailable();
    }, [debouncedSearchQuery, get, memberships, organizationId, errorMessage]);

    const handleAdd = React.useCallback(async (user: OrganizationUser) => {
        setLoading(true);
        try {
            const response = await post<{ message?: { id: string }; id?: string }>(`organizations/{${OrganizationGroupMemberResource}}`, {
                group_id: groupId,
                user_id: user.id,
                is_active: true,
            });
            const newId = response?.message?.id || response?.id;
            if (newId) {
                setMemberships(prev => [...prev, { id: newId, user_id: user.id }]);
                setAssignedUsers(prev => [...prev, user]);
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
    }, [groupId, post, successMessage, errorMessage]);

    const handleRemove = React.useCallback(async (membershipId: string) => {
        setLoading(true);
        try {
            await del(`organizations/{${OrganizationGroupMemberResource}}/{id:${membershipId}}`);
            setMemberships(prev => prev.filter(m => m.id !== membershipId));
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
    }, [del, successMessage, errorMessage]);

    const assignedIds = useMemo(() => new Set(memberships.map(m => m.user_id)), [memberships]);

    const visibleUsers = useMemo(() => {
        let filtered = assignedUsers.filter(u => assignedIds.has(u.id));
        if (currentFilters.length > 0) {
            filtered = filtered.filter(u => {
                return currentFilters.every(filter => {
                    const value = u[filter.field as keyof OrganizationUser];
                    if (!value) return false;
                    const strValue = String(value).toLowerCase();
                    const searchStr = String(filter.value).toLowerCase();
                    return filter.operator === 'like' ? strValue.includes(searchStr) : strValue === searchStr;
                });
            });
        }
        return filtered;
    }, [assignedUsers, assignedIds, currentFilters]);

    const filterData = React.useCallback((values: FilteredValue[]) => {
        const newFilters: ResourceFilter[] = [];
        if (values.length > 0) {
            const activeFilters = values[0];
            Object.entries(activeFilters).forEach(([key, val]) => {
                if (val === undefined || val === null || val === '') return;
                newFilters.push({ field: String(key), operator: 'like', value: String(val) });
            });
        }
        setCurrentFilters(newFilters);
    }, []);

    const columns = useMemo<TableColumn<OrganizationUser>[]>(() => [
        { key: 'username', label: 'Username' },
        { key: 'email', label: 'Email' },
        {
            key: 'id',
            label: 'Action',
            render: (_value, row) => {
                const membership = memberships.find(m => m.user_id === row.id);
                return (
                    <div className="flex justify-end gap-2">
                        <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsGroupsDelete, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                            <DeleteAction
                                onClick={() => {
                                    if (membership) void handleRemove(membership.id);
                                }}
                                disabled={loading || !membership}
                            />
                        </ScopeBasedComponentAccess>
                    </div>
                );
            }
        },
    ], [memberships, loading, handleRemove]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading members...</div>;
    }

    return (
        <div className="space-y-8">
            <div className="flex justify-between items-start">
                <div>
                    <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">Group Members</h4>
                    <p className="text-sm text-gray-500 mb-4">Users currently assigned to this group.</p>
                </div>
                {!isSearchVisible && (
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsGroupsWrite, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                        <Add onClick={() => setIsSearchVisible(true)} />
                    </ScopeBasedComponentAccess>
                )}
            </div>

            {isSearchVisible && (
                <div className="space-y-4">
                    <div className="flex justify-between items-center mb-2">
                        <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300">Search for users to add</h5>
                        <Close
                            onClick={() => {
                                setIsSearchVisible(false);
                                setSearchQuery('');
                            }}
                            label="Close"
                        />
                    </div>
                    <FormInput
                        id="search"
                        value={searchQuery}
                        onChange={setSearchQuery}
                        placeholder="Search available users by username or email..."
                    />

                    {debouncedSearchQuery && availableUsers.length === 0 && (
                        <p className="text-sm text-gray-500 pt-2">No available users found.</p>
                    )}

                    {debouncedSearchQuery && availableUsers.length > 0 && (
                        <div className="border border-gray-200 dark:border-surface-900 rounded-md overflow-hidden bg-white dark:bg-surface-800">
                            <div className="max-h-64 overflow-y-auto">
                                <table className="min-w-full divide-y divide-gray-200 dark:divide-surface-900">
                                    <thead className="bg-gray-50 dark:bg-surface-900 sticky top-0 hidden md:table-header-group">
                                        <tr>
                                            <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Username</th>
                                            <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Email</th>
                                            <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Action</th>
                                        </tr>
                                    </thead>
                                    <tbody className="bg-white dark:bg-surface-800 divide-y divide-gray-200 dark:divide-surface-900">
                                        {availableUsers.map((user) => (
                                            <tr key={user.id} className="hover:bg-gray-50 dark:hover:bg-surface-500 transition-colors flex flex-col md:table-row">
                                                <td className="px-6 py-4 md:whitespace-nowrap text-sm font-medium text-gray-900 dark:text-gray-100 flex items-center justify-between md:table-cell">
                                                    <span className="md:hidden font-bold text-gray-500 uppercase text-xs mr-2">Username:</span>
                                                    {user.username}
                                                </td>
                                                <td className="px-6 py-4 md:whitespace-nowrap text-sm text-gray-500 dark:text-gray-400 flex items-center justify-between md:table-cell">
                                                    <span className="md:hidden font-bold text-gray-500 uppercase text-xs mr-2">Email:</span>
                                                    {user.email}
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium flex justify-end md:table-cell border-t md:border-t-0 border-gray-100 dark:border-surface-900 bg-gray-50 dark:bg-surface-900 md:bg-transparent">
                                                    <SubmitSmall
                                                        onClick={() => {
                                                            void handleAdd(user);
                                                            setIsSearchVisible(false);
                                                            setSearchQuery('');
                                                        }}
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
                    )}
                </div>
            )}

            <div>
                <TableView
                    columns={columns}
                    data={visibleUsers}
                    total={visibleUsers.length}
                    page={1}
                    pageSize={visibleUsers.length || 1}
                    onPageChange={() => { }}
                    filters={{
                        username: { type: 'text', label: 'Username', placeholder: 'Search username' },
                        email: { type: 'text', label: 'Email', placeholder: 'Search email' },
                    }}
                    onFilterChange={filterData}
                    emptyText="No users assigned to this group."
                />
            </div>
        </div>
    );
};

export default GroupMembers;
