import React, { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { WorkspaceResource, type Workspace, WorkspaceSchema } from '../model/workspace';
import { OrganizationResource, type Organization, OrganizationSchema } from '../model/organization';
import { formatDateOnly } from '../../../config/data/date/date';
import { CardView } from '../../../Shared/Components/CardView';
import type { FilteredValue } from '../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../Shared/Components/Table/Table';
import Title from '../../../Shared/Components/Font/Title.tsx';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../Shared/Components/Button';
import { ConfirmModal } from '../../../Shared/Components/Modal';
import { DotsDropdown } from '../../../Shared/Components/DotsDropdown';
import { StatCard } from '../../../Shared/Components/Cards';
import { useResourceRepository } from '../../../config/data/resource/repository';
import type { ResourceFilter } from '../../../config/data/resource/filters';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const WorkspacesDashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Workspaces' }]);

  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const navigate = useNavigate();
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [workspaceToDelete, setWorkspaceToDelete] = useState<{ id: string; title: string } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const { del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const { fetchCollection } = useResourceRepository();

  const fetchWorkspaces = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mapped = await fetchCollection<Workspace>(
        `workspaces/{${WorkspaceResource}}`,
        WorkspaceSchema,
        { filters: currentFilters }
      );
      setWorkspaces(mapped);
    } catch {
      const msg = 'Failed to fetch workspaces';
      setError(msg);
      errorMessage(msg);
    } finally {
      setLoading(false);
    }
  }, [fetchCollection, errorMessage, currentFilters]);

  const fetchOrganizations = React.useCallback(async () => {
    try {
      const mapped = await fetchCollection<Organization>(
        `organizations/{${OrganizationResource}}`,
        OrganizationSchema,
      );
      setOrganizations(mapped);
    } catch {
      // non-fatal: workspaces will render with id-only fallback
    }
  }, [fetchCollection]);

  const previousFilters = React.useRef<string | null>(null);

  useEffect(() => {
    const filtersString = JSON.stringify(currentFilters);
    if (previousFilters.current !== filtersString) {
      previousFilters.current = filtersString;
      void fetchWorkspaces();
    }
  }, [fetchWorkspaces, currentFilters]);

  useEffect(() => {
    void fetchOrganizations();
  }, [fetchOrganizations]);

  const organizationIndex = useMemo(() => {
    const idx = new Map<string, Organization>();
    organizations.forEach(o => idx.set(o.id, o));
    return idx;
  }, [organizations]);

  const promptDelete = React.useCallback((id: string, title: string) => {
    setWorkspaceToDelete({ id, title });
    setIsConfirmOpen(true);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!workspaceToDelete) return;
    setIsDeleting(true);
    try {
      await del(`workspaces/{${WorkspaceResource}}/{id:${workspaceToDelete.id}}`);
      successMessage(`Workspace ${workspaceToDelete.title} deleted successfully!`);
      void fetchWorkspaces();
      setIsConfirmOpen(false);
    } catch {
      errorMessage('Failed to delete workspace');
    } finally {
      setIsDeleting(false);
    }
  }, [del, fetchWorkspaces, successMessage, errorMessage, workspaceToDelete]);

  const columns = React.useMemo<TableColumn<Workspace>[]>(() => [
    {
      key: 'title',
      label: '',
      render: (value, row) => (
        <div className="flex items-center gap-2">
          <span className="font-semibold text-gray-900 dark:text-gray-100">{String(value)}</span>
          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${row.isActive
            ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
            : 'bg-gray-100 text-gray-600 dark:bg-surface-700 dark:text-gray-400'
            }`}>
            {row.isActive ? 'Active' : 'Inactive'}
          </span>
        </div>
      ),
    },
    {
      key: 'description',
      label: 'Description',
      render: (value) => (
        <span className="text-sm text-gray-600 dark:text-gray-400">{String(value || '—')}</span>
      ),
    },
    {
      key: 'organizationId',
      label: 'Organization',
      render: (value) => {
        const orgId = value as string;
        if (!orgId) return '—';
        const org = organizationIndex.get(orgId);
        return (
          <Link
            to={`/organizations/edit/${orgId}`}
            className="text-indigo-600 hover:text-indigo-500 dark:text-indigo-400 text-sm"
            onClick={e => e.stopPropagation()}
          >
            {org?.title || orgId}
          </Link>
        );
      },
    },
    {
      key: 'createdAt',
      label: 'Created',
      render: (value) => (
        <span className="text-sm text-gray-700 dark:text-gray-300">{formatDateOnly(value as string)}</span>
      ),
    },
    {
      key: 'id',
      label: 'Actions',
      render: (_value, row) => (
        <DotsDropdown
          items={[
            {
              label: 'Edit',
              href: `/workspaces/edit/${row.id}`,
              icon: (
                <svg fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                </svg>
              ),
            },
            {
              label: 'Applications',
              href: `/workspaces/${row.id}/applications`,
              icon: (
                <svg fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6A2.25 2.25 0 0 1 6 3.75h2.25A2.25 2.25 0 0 1 10.5 6v2.25a2.25 2.25 0 0 1-2.25 2.25H6a2.25 2.25 0 0 1-2.25-2.25V6ZM3.75 15.75A2.25 2.25 0 0 1 6 13.5h2.25a2.25 2.25 0 0 1 2.25 2.25V18a2.25 2.25 0 0 1-2.25 2.25H6A2.25 2.25 0 0 1 3.75 18v-2.25ZM13.5 6a2.25 2.25 0 0 1 2.25-2.25H18A2.25 2.25 0 0 1 20.25 6v2.25A2.25 2.25 0 0 1 18 10.5h-2.25a2.25 2.25 0 0 1-2.25-2.25V6ZM13.5 15.75a2.25 2.25 0 0 1 2.25-2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25h-2.25A2.25 2.25 0 0 1 13.5 18v-2.25Z" />
                </svg>
              ),
            },
            {
              label: 'Delete',
              variant: 'danger',
              onClick: () => promptDelete(row.id, row.title),
              icon: (
                <svg fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
                </svg>
              ),
            },
          ]}
        />
      ),
    },
  ], [promptDelete, organizationIndex]);

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

  const totalWorkspaces = workspaces.length;
  const activeWorkspaces = workspaces.filter(w => w.isActive).length;
  const inactiveWorkspaces = totalWorkspaces - activeWorkspaces;
  const orgsWithWorkspaces = new Set(workspaces.map(w => w.organizationId).filter(Boolean)).size;

  if (loading && workspaces.length === 0) return <div>Loading workspaces...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <Title>Workspaces</Title>
        {/* The Create button is gated on an organization existing —
            workspaces belong to orgs, so the action is meaningless
            without one. When there are zero orgs we skip rendering
            the button entirely (not just disable) to keep the
            header uncluttered. */}
        {organizations.length > 0 && (
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}>
            <Submit
              type="button"
              onClick={() => navigate('/workspaces/create')}
              label="Create Workspace"
            />
          </ScopeBasedComponentAccess>
        )}
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Total Workspaces" value={totalWorkspaces} color="blue" />
        <StatCard label="Active" value={activeWorkspaces} color="teal" />
        <StatCard label="Inactive" value={inactiveWorkspaces} color="amber" />
        <StatCard label="Organizations" value={orgsWithWorkspaces} color="violet" />
      </div>

      <CardView
        columns={columns}
        data={workspaces}
        total={workspaces.length}
        page={1}
        pageSize={workspaces.length || 1}
        onPageChange={() => {}}
        filters={{
          title: { type: 'text', label: 'Title', placeholder: 'Search title' },
        }}
        onFilterChange={filterData}
        rowKey={(row) => row.id}
        actionsKey="id"
      />

      <ConfirmModal
        isOpen={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete Workspace"
        message={workspaceToDelete ? `Are you sure you want to delete the workspace "${workspaceToDelete.title}"?` : ''}
        confirmLabel="Delete"
        isLoading={isDeleting}
        variant="danger"
      />
    </div>
  );
};

export default WorkspacesDashboard;
