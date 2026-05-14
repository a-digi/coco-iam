import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../../Shared/Components/Font/Title';
import { Submit } from '../../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import NoEntriesFound from '../../../../Shared/Components/NoEntries/NoEntriesFound';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { type OrganizationGroup, OrganizationGroupSchema, OrganizationGroupResource } from '../../model/organizationGroup';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { type ResourceFilter, buildFilterQueryString } from '../../../../config/data/resource/filters';
import { formatDate } from '../../../../config/data/date/date';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

const OrganizationGroupsDashboard: React.FC = () => {
    useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Groups' }]);
    const { orgId } = useParams<{ orgId: string }>();
    const navigate = useNavigate();
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [groups, setGroups] = useState<OrganizationGroup[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

    const [isConfirmOpen, setIsConfirmOpen] = useState(false);
    const [groupToDelete, setGroupToDelete] = useState<{ id: string, title: string } | null>(null);
    const [isDeleting, setIsDeleting] = useState(false);

    const fetchGroups = React.useCallback(async () => {
        if (!orgId) return;
        setLoading(true);
        try {
            const qs = buildFilterQueryString([{ field: 'organization_id', operator: 'exact', value: orgId }]);
            const response = await get<{ message?: unknown }>(`organizations/{${OrganizationGroupResource}}?${qs}`);
            const data = response?.message || response || [];
            if (Array.isArray(data)) {
                const mapped = mapObjects(OrganizationGroupSchema, data) as unknown as OrganizationGroup[];
                setGroups(mapped);
            } else {
                setGroups([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to fetch groups';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [orgId, get, errorMessage]);

    useEffect(() => {
        void fetchGroups();
    }, [fetchGroups]);

    const promptDelete = React.useCallback((id: string, title: string) => {
        setGroupToDelete({ id, title });
        setIsConfirmOpen(true);
    }, []);

    const confirmDelete = React.useCallback(async () => {
        if (!groupToDelete) return;
        setIsDeleting(true);
        try {
            await del(`organizations/{${OrganizationGroupResource}}/{id:${groupToDelete.id}}`);
            successMessage(`Group ${groupToDelete.title} deleted successfully!`);
            void fetchGroups();
            setIsConfirmOpen(false);
        } catch {
            errorMessage('Failed to delete group');
        } finally {
            setIsDeleting(false);
        }
    }, [del, fetchGroups, successMessage, errorMessage, groupToDelete]);

    const filteredGroups = useMemo(() => {
        if (currentFilters.length === 0) return groups;
        return groups.filter(g => {
            return currentFilters.every(filter => {
                const value = g[filter.field as keyof OrganizationGroup];
                if (!value) return false;
                const strValue = String(value).toLowerCase();
                const searchStr = String(filter.value).toLowerCase();
                return filter.operator === 'like' ? strValue.includes(searchStr) : strValue === searchStr;
            });
        });
    }, [groups, currentFilters]);

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

    const columns = useMemo<TableColumn<OrganizationGroup>[]>(() => [
        { key: 'title', label: 'Title' },
        { key: 'groupDescription', label: 'Description' },
        {
            key: 'createdAt',
            label: 'Created at',
            render: (value) => formatDate(value as string)
        },
        {
            key: 'id',
            label: 'Actions',
            render: (_value, row) => (
                <div className="flex items-center space-x-2">
                    <EditAction to={`/organizations/${orgId}/groups/edit/${row.id}`} />
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsGroupsDelete, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                        <DeleteAction onClick={() => promptDelete(row.id, row.title)} />
                    </ScopeBasedComponentAccess>
                </div>
            )
        },
    ], [orgId, promptDelete]);

    if (!orgId) return <div>Missing organization id.</div>;

    return (
        <div>
            <OrganizationHeader organizationId={orgId} />

            <div className="flex justify-between items-center mb-6">
                <Title>Organization Groups</Title>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsGroupsWrite, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                    <Submit
                        type="button"
                        onClick={() => navigate(`/organizations/${orgId}/groups/create`)}
                        label="Create Group"
                    />
                </ScopeBasedComponentAccess>
            </div>

            {loading && groups.length === 0 ? (
                <div>Loading groups...</div>
            ) : groups.length === 0 && currentFilters.length === 0 ? (
                <div className="mt-8">
                    <NoEntriesFound
                        title="No Groups Found"
                        message="There are no groups in this organization yet. Create the first one."
                    />
                </div>
            ) : (
                <TableView
                    columns={columns}
                    data={filteredGroups}
                    total={filteredGroups.length}
                    page={1}
                    pageSize={filteredGroups.length || 1}
                    onPageChange={() => { }}
                    filters={{
                        title: { type: 'text', label: 'Title', placeholder: 'Search title' },
                    }}
                    onFilterChange={filterData}
                />
            )}

            <ConfirmModal
                isOpen={isConfirmOpen}
                onClose={() => setIsConfirmOpen(false)}
                onConfirm={confirmDelete}
                title="Delete Group"
                message={groupToDelete ? `Are you sure you want to delete "${groupToDelete.title}"?` : ''}
                confirmLabel="Delete"
                isLoading={isDeleting}
                variant="danger"
            />
        </div>
    );
};

export default OrganizationGroupsDashboard;
