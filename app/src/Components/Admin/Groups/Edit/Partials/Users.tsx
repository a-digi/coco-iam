import React, { useEffect, useState, useMemo } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../../Shared/Components/Button/SubmitSmall';
import { Add } from '../../../../../Shared/Components/Button/Add';
import { DeleteAction } from '../../../../../Shared/Components/Actions/DeleteAction';
import { LinkAction } from '../../../../../Shared/Components/Actions/Link';
import { Close } from '../../../../../Shared/Components/Button/Close';
import { type User, StandardSchema } from '../../../Users/model/user';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import TableView, { type FilteredValue } from '../../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import { type ResourceFilter, buildFilterQueryString } from '../../../../../config/data/resource/filters';
import { FormInput } from '../../../../../Shared/Components/Form';

interface UsersProps {
    entityId: string;
}

export const Users: React.FC<UsersProps> = ({ entityId }) => {
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [groupUsers, setGroupUsers] = useState<User[]>([]);
    const [availableUsers, setAvailableUsers] = useState<User[]>([]);
    const [memberships, setMemberships] = useState<{ id: string; user_id: string }[]>([]);

    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

    const [isSearchVisible, setIsSearchVisible] = useState(false);

    const [searchQuery, setSearchQuery] = useState('');
    const [debouncedSearchQuery, setDebouncedSearchQuery] = useState('');

    const fetchData = React.useCallback(async () => {
        if (!entityId) return;
        setFetching(true);
        try {
            try {
                const qs = buildFilterQueryString([{ field: 'group_id', operator: 'exact', value: entityId }]);
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                const membersResponse = await get<{ message?: any[] }>(`admin/{res:admin_group_members}?${qs}`);
                const membersData = membersResponse?.message || membersResponse || [];

                if (Array.isArray(membersData)) {
                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    const membersList = membersData.map((m: any) => ({ id: String(m.id), user_id: String(m.user_id) }));
                    setMemberships(membersList);

                    // eslint-disable-next-line @typescript-eslint/no-explicit-any
                    const rawUsers = membersData.map((m: any) => m.user).filter(Boolean);
                    if (rawUsers.length > 0) {
                        const mappedUsers = mapObjects(StandardSchema, rawUsers) as unknown as User[];
                        setGroupUsers(mappedUsers);
                    } else {
                        setGroupUsers([]);
                    }
                }
            } catch (err) {
                console.warn("No members found or error fetching members", err);
            }

        } catch (err) {
            console.error("Failed to fetch group users", err);
            errorMessage("Failed to load users");
        } finally {
            setFetching(false);
        }
    }, [entityId, get, errorMessage]);

    useEffect(() => {
        void fetchData();
    }, [fetchData]);

    useEffect(() => {
        const timer = setTimeout(() => {
            setDebouncedSearchQuery(searchQuery);
        }, 500);
        return () => clearTimeout(timer);
    }, [searchQuery]);

    useEffect(() => {
        if (!debouncedSearchQuery) {
            setAvailableUsers([]);
            return;
        }

        const fetchAvailableUsers = async () => {
            try {
                const usersResponse = await get<{ message?: unknown }>(`admin/{res:users}`);
                const usersData = usersResponse?.message || usersResponse || [];
                if (Array.isArray(usersData)) {
                    const mappedUsers = mapObjects(StandardSchema, usersData) as unknown as User[];
                    const assignedUserIds = new Set(memberships.map(m => m.user_id));

                    const available = mappedUsers.filter(u => !assignedUserIds.has(u.id));
                    const finalAvailable = available.filter(u => {
                        const query = debouncedSearchQuery.toLowerCase();
                        return u.username?.toLowerCase().includes(query) || u.email?.toLowerCase().includes(query);
                    });

                    setAvailableUsers(finalAvailable);
                }
            } catch (err) {
                console.error("Failed to fetch available users", err);
            }
        };

        void fetchAvailableUsers();
    }, [debouncedSearchQuery, get, memberships]);

    const handleAddUser = React.useCallback(async (user: User) => {
        setLoading(true);
        try {
            const response = await post<{ message?: { id: string }; id?: string }>(`admin/{res:admin_group_members}`, {
                group_id: entityId,
                user_id: user.id,
                is_active: true
            });
            const newMemberId = response?.message?.id || response?.id;

            if (newMemberId) {
                setMemberships(prev => [...prev, { id: newMemberId, user_id: user.id }]);
                setGroupUsers(prev => [...prev, user]);
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
    }, [entityId, post, successMessage, errorMessage]);

    const handleRemoveUser = React.useCallback(async (membershipId: string) => {
        setLoading(true);
        try {
            await del(`admin/{res:admin_group_members}/{id:${membershipId}}`);
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

    const assignedUserIds = useMemo(() => new Set(memberships.map(m => m.user_id)), [memberships]);

    const assignedUsers = useMemo(() => {
        let filtered = groupUsers.filter(u => assignedUserIds.has(u.id));
        if (currentFilters.length > 0) {
            filtered = filtered.filter(u => {
                return currentFilters.every(filter => {
                    const value = u[filter.field as keyof User];
                    if (!value) return false;
                    const strValue = String(value).toLowerCase();
                    const searchStr = String(filter.value).toLowerCase();
                    return filter.operator === 'like' ? strValue.includes(searchStr) : strValue === searchStr;
                });
            });
        }
        return filtered;
    }, [groupUsers, assignedUserIds, currentFilters]);

    const filterData = React.useCallback((values: FilteredValue[]) => {
        const newFilters: ResourceFilter[] = [];
        if (values.length > 0) {
            const activeFilters = values[0];
            Object.entries(activeFilters).forEach(([key, val]) => {
                if (val === undefined || val === null || val === '') return;
                newFilters.push({
                    field: String(key),
                    operator: 'like',
                    value: String(val)
                });
            });
        }
        setCurrentFilters(newFilters);
    }, []);

    const columns = useMemo<TableColumn<User>[]>(() => [
        { key: 'username', label: 'Username' },
        { key: 'email', label: 'Email' },
        {
            key: 'id',
            label: 'Action',
            render: (_value, row) => {
                const membership = memberships.find(m => m.user_id === row.id);
                return (
                    <div className="flex justify-end gap-2">
                        <LinkAction
                            to={`/admin/users/edit/${row.id}`}
                            label="View User"
                        />
                        <DeleteAction
                            onClick={() => {
                                if (membership) {
                                    void handleRemoveUser(membership.id);
                                }
                            }}
                            disabled={loading || !membership}
                        />
                    </div>
                );
            }
        },
    ], [memberships, loading, handleRemoveUser]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading users...</div>;
    }

    return (
        <div className="space-y-8">
            <div className="pt-6 border-t border-gray-200 dark:border-surface-800 flex justify-between items-start">
                <div>
                    <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">Manage Group Members</h4>
                    <p className="text-sm text-gray-500 mb-4">Users that are currently part of this group.</p>
                </div>
                {!isSearchVisible && (
                    <Add onClick={() => setIsSearchVisible(true)} />
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
                            placeholder="Search available users by username or email..."
                            inputClassName="pl-10"
                        />
                    </div>

                    {debouncedSearchQuery && availableUsers.length === 0 && (
                        <p className="text-sm text-gray-500 pt-2">No available users found matching your search.</p>
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
                                                            void handleAddUser(user);
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
                    data={assignedUsers}
                    total={assignedUsers.length}
                    page={1}
                    pageSize={assignedUsers.length || 1}
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

export default Users;
