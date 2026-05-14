import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../../Shared/Components/Font/Title';
import { Submit } from '../../../../Shared/Components/Button';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import MasonryView from '../../../../Shared/Components/MasonryView';
import type { FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import NoEntriesFound from '../../../../Shared/Components/NoEntries/NoEntriesFound';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { type OrganizationUser, OrganizationUserSchema, OrganizationUserResource } from '../../model/organizationUser';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { type ResourceFilter, buildFilterQueryString } from '../../../../config/data/resource/filters';
import { OrganizationPageHead } from '../../Shared/OrganizationPageHead';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import { OrganizationUserCard } from './Partials/OrganizationUserCard';

const OrganizationUsersDashboard: React.FC = () => {
    useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Users' }]);
    const { orgId } = useParams<{ orgId: string }>();
    const navigate = useNavigate();
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [users, setUsers] = useState<OrganizationUser[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);
    const [page, setPage] = useState<number>(1);
    const [pageSize, setPageSize] = useState<number>(24);

    const [isConfirmOpen, setIsConfirmOpen] = useState(false);
    const [userToDelete, setUserToDelete] = useState<{ id: string, username: string } | null>(null);
    const [isDeleting, setIsDeleting] = useState(false);

    const fetchUsers = React.useCallback(async () => {
        if (!orgId) return;
        setLoading(true);
        try {
            const qs = buildFilterQueryString([{ field: 'organization_id', operator: 'exact', value: orgId }]);
            const response = await get<{ message?: unknown }>(`organizations/{${OrganizationUserResource}}?${qs}`);
            const data = response?.message || response || [];
            if (Array.isArray(data)) {
                const mapped = mapObjects(OrganizationUserSchema, data) as unknown as OrganizationUser[];
                setUsers(mapped);
            } else {
                setUsers([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to fetch users';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [orgId, get, errorMessage]);

    useEffect(() => {
        void fetchUsers();
    }, [fetchUsers]);

    const promptDelete = React.useCallback((id: string, username: string) => {
        setUserToDelete({ id, username });
        setIsConfirmOpen(true);
    }, []);

    const confirmDelete = React.useCallback(async () => {
        if (!userToDelete) return;
        setIsDeleting(true);
        try {
            await del(`organizations/{${OrganizationUserResource}}/{id:${userToDelete.id}}`);
            successMessage(`User ${userToDelete.username} deleted successfully!`);
            void fetchUsers();
            setIsConfirmOpen(false);
        } catch {
            errorMessage('Failed to delete user');
        } finally {
            setIsDeleting(false);
        }
    }, [del, fetchUsers, successMessage, errorMessage, userToDelete]);

    const filteredUsers = useMemo(() => {
        if (currentFilters.length === 0) return users;
        return users.filter(u => {
            return currentFilters.every(filter => {
                const value = u[filter.field as keyof OrganizationUser];
                if (value === undefined || value === null) return false;
                const strValue = String(value).toLowerCase();
                const searchStr = String(filter.value).toLowerCase();
                return filter.operator === 'like' ? strValue.includes(searchStr) : strValue === searchStr;
            });
        });
    }, [users, currentFilters]);

    // Current page's slice — MasonryView expects the already-sliced
    // window. Client-side paging is fine here because an org's user
    // list is bounded by the existing list-resource cap; swap for a
    // backend-paged variant if that changes.
    const paginatedUsers = useMemo(
        () => filteredUsers.slice((page - 1) * pageSize, page * pageSize),
        [filteredUsers, page, pageSize],
    );

    // Reset to the first page whenever the filter set narrows/widens.
    useEffect(() => {
        setPage(1);
    }, [currentFilters]);

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

    if (!orgId) return <div>Missing organization id.</div>;

    return (
        <div>
            <OrganizationPageHead
                organizationId={orgId}
                backTo={`/organizations/edit/${orgId}`}
                backLabel="Back to organization"
            />

            <div className="flex justify-between items-center mb-6">
                <Title>Organization Users</Title>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsUsersWrite, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                    <Submit
                        type="button"
                        onClick={() => navigate(`/organizations/${orgId}/users/create`)}
                        label="Create User"
                    />
                </ScopeBasedComponentAccess>
            </div>

            {loading && users.length === 0 ? (
                <div>Loading users...</div>
            ) : users.length === 0 && currentFilters.length === 0 ? (
                <div className="mt-8">
                    <NoEntriesFound
                        title="No Users Found"
                        message="There are no users in this organization yet. Create the first one."
                    />
                </div>
            ) : (
                <MasonryView<OrganizationUser>
                    data={paginatedUsers}
                    renderItem={(user) => (
                        <OrganizationUserCard
                            user={user}
                            organizationId={orgId}
                            onDelete={promptDelete}
                        />
                    )}
                    itemKey={(user) => user.id}
                    total={filteredUsers.length}
                    page={page}
                    pageSize={pageSize}
                    onPageChange={setPage}
                    onPageSizeChange={setPageSize}
                    pageSizeOptions={[12, 24, 48, 96]}
                    filters={{
                        username: { type: 'text', label: 'Username', placeholder: 'Search username' },
                        email: { type: 'text', label: 'Email', placeholder: 'Search email' },
                    }}
                    onFilterChange={filterData}
                    columns={1}
                    gap={16}
                    breakpointCols={{ 640: 2, 1024: 3, 1440: 4 }}
                    emptyText="No users match your filters."
                />
            )}

            <ConfirmModal
                isOpen={isConfirmOpen}
                onClose={() => setIsConfirmOpen(false)}
                onConfirm={confirmDelete}
                title="Delete User"
                message={userToDelete ? `Are you sure you want to delete "${userToDelete.username}"?` : ''}
                confirmLabel="Delete"
                isLoading={isDeleting}
                variant="danger"
            />
        </div>
    );
};

export default OrganizationUsersDashboard;
