import React, { useEffect, useState } from 'react';
import { OrganizationResource, type Organization, OrganizationSchema } from '../model/organization';
import { formatDateOnly } from '../../../config/data/date/date';
import TableView, { type FilteredValue } from '../../../Shared/Components/TableView/TableView';
import { TwoColumnsLeft } from '../../../Shared/Components/Columns';
import type { TableColumn } from '../../../Shared/Components/Table/Table';
import Title from '../../../Shared/Components/Font/Title.tsx';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useNavigate } from 'react-router-dom';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../Shared/Components/Modal';
import { StatCard, type StatCardColor } from '../../../Shared/Components/Cards';
import { useResourceRepository } from '../../../config/data/resource/repository';
import type { ResourceFilter } from '../../../config/data/resource/filters';
import NoEntriesFound from '../../../Shared/Components/NoEntries/NoEntriesFound';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { Alert } from '../../../Shared/Components/Alert';
import { AppScopes } from '../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

const OrganizationsDashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Organizations' }]);

  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const navigate = useNavigate();
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [orgToDelete, setOrgToDelete] = useState<{ id: string; title: string } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const { del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const { fetchCollection } = useResourceRepository();

  const fetchOrganizations = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mapped = await fetchCollection<Organization>(
        `organizations/{${OrganizationResource}}`,
        OrganizationSchema,
        { filters: currentFilters }
      );
      setOrganizations(mapped);
    } catch {
      const msg = 'Failed to fetch organizations';
      setError(msg);
      errorMessage(msg);
    } finally {
      setLoading(false);
    }
  }, [fetchCollection, errorMessage, currentFilters]);

  const previousFilters = React.useRef<string | null>(null);

  useEffect(() => {
    const filtersString = JSON.stringify(currentFilters);
    if (previousFilters.current !== filtersString) {
      previousFilters.current = filtersString;
      void fetchOrganizations();
    }
  }, [fetchOrganizations, currentFilters]);

  const promptDelete = React.useCallback((id: string, title: string) => {
    setOrgToDelete({ id, title });
    setIsConfirmOpen(true);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!orgToDelete) return;
    setIsDeleting(true);
    try {
      await del(`organizations/{${OrganizationResource}}/{id:${orgToDelete.id}}`);
      successMessage(`Organization ${orgToDelete.title} deleted successfully!`);
      void fetchOrganizations();
      setIsConfirmOpen(false);
    } catch {
      errorMessage('Failed to delete organization');
    } finally {
      setIsDeleting(false);
    }
  }, [del, fetchOrganizations, successMessage, errorMessage, orgToDelete]);

  const columns = React.useMemo<TableColumn<Organization>[]>(() => [
    {
      key: 'title',
      label: 'Organization',
      render: (value) => (
        <span className="font-semibold text-gray-900 dark:text-gray-100">{String(value)}</span>
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
        <div className="flex items-center gap-2">
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
            <EditAction to={`/organizations/edit/${row.id}`} />
          </ScopeBasedComponentAccess>
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsDelete, AppScopes.Organizations, AppScopes.SuperAdmin]}>
            <DeleteAction onClick={() => promptDelete(row.id, row.title)} />
          </ScopeBasedComponentAccess>
        </div>
      ),
    },
  ], [promptDelete]);

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

  const totalOrgs = organizations.length;
  const activeOrgs = organizations.filter(o => o.isActive).length;
  const inactiveOrgs = totalOrgs - activeOrgs;
  const recentOrgs = organizations.filter(o => {
    const created = new Date(o.createdAt);
    const thirtyDaysAgo = new Date();
    thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30);
    return created >= thirtyDaysAgo;
  }).length;

  const statCards: { label: string; value: number; color: StatCardColor }[] = [
    { label: 'Total',     value: totalOrgs,   color: 'blue' },
    { label: 'Active',    value: activeOrgs,  color: 'teal' },
    { label: 'Inactive',  value: inactiveOrgs, color: 'amber' },
    { label: 'New (30d)', value: recentOrgs,  color: 'violet' },
  ];

  const aboutAlert = (
    <Alert variant="info" title="About Organizations">
      An organization is the top-level tenant in coco-iam. It owns workspaces, applications, and its own pool of users.
      Each organization keeps its profile-field schema and user profile data in its own isolated database.
    </Alert>
  );

  const sidebar = (
    <div className="grid">
      <div className="grid grid-cols-2 gap-4">
        {statCards.map(c => (
          <StatCard key={c.label} label={c.label} value={c.value} color={c.color} />
        ))}
      </div>
      <div className="mt-5">{aboutAlert}</div>
    </div>
  );

  return (
    <div className="p-6">
      {/* ── Mobile layout ─────────────────────────────────── */}
      <div className="md:hidden space-y-6">
        <div className="flex justify-between items-center">
          <Title>Organizations</Title>
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
            <Submit
              type="button"
              onClick={() => navigate('/organizations/create')}
              label="Create Organization"
            />
          </ScopeBasedComponentAccess>
        </div>

        {/* Stats carousel */}
        <div className="flex overflow-x-auto snap-x snap-mandatory gap-4 pb-2">
          {statCards.map(c => (
            <div key={c.label} className="snap-start shrink-0 w-[44vw] sm:w-[32vw]">
              <StatCard label={c.label} value={c.value} color={c.color} />
            </div>
          ))}
        </div>

        {/* Organization cards */}
        {loading ? (
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-[88px] rounded-xl bg-gray-100 dark:bg-surface-800 animate-pulse" />
            ))}
          </div>
        ) : error ? (
          <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
        ) : organizations.length === 0 && currentFilters.length === 0 ? (
          <NoEntriesFound
            title="No Organizations Found"
            message="There are currently no organizations to display. Get started by creating your first organization using the button above."
          />
        ) : (
          <div className="space-y-3">
            {organizations.map(org => (
              <div
                key={org.id}
                className="bg-white dark:bg-surface-800 rounded-xl border border-gray-200 dark:border-surface-700 p-4 flex items-start justify-between gap-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-semibold text-gray-900 dark:text-gray-100 truncate">
                      {org.title}
                    </span>
                    <span
                      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold shrink-0 ${
                        org.isActive
                          ? 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-surface-700 dark:text-gray-400'
                      }`}
                    >
                      {org.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                  {org.organizationId && (
                    <p className="text-xs text-indigo-500 dark:text-indigo-400 font-mono mt-1 truncate">
                      {org.organizationId}
                    </p>
                  )}
                  {org.description && (
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">
                      {org.description}
                    </p>
                  )}
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1.5">
                    {formatDateOnly(org.createdAt)}
                  </p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                    <EditAction to={`/organizations/edit/${org.id}`} />
                  </ScopeBasedComponentAccess>
                  <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsDelete, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                    <DeleteAction onClick={() => promptDelete(org.id, org.title)} />
                  </ScopeBasedComponentAccess>
                </div>
              </div>
            ))}
          </div>
        )}

        {aboutAlert}
      </div>

      {/* ── Desktop layout ────────────────────────────────── */}
      <div className="hidden md:block">
        {organizations.length === 0 && currentFilters.length === 0 && !loading ? (
          <div className="space-y-4">
            <div className="flex justify-between items-center mb-2">
              <Title className="mb-0">Organizations</Title>
              <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                <Submit
                  type="button"
                  onClick={() => navigate('/organizations/create')}
                  label="Create Organization"
                />
              </ScopeBasedComponentAccess>
            </div>
            <NoEntriesFound
              title="No Organizations Found"
              message="There are currently no organizations to display. Get started by creating your first organization using the button above."
            />
          </div>
        ) : (
          <TwoColumnsLeft
            left={
              <div className="space-y-4">
                <div className="flex justify-between items-center mb-6">
                  <Title>Organizations</Title>
                </div>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
                  <div className="flex justify-start">
                    <Submit
                      type="button"
                      onClick={() => navigate('/organizations/create')}
                      label="Create Organization"
                    />
                  </div>
                </ScopeBasedComponentAccess>
                <TableView
                  columns={columns}
                  data={organizations}
                  total={organizations.length}
                  page={1}
                  pageSize={organizations.length || 1}
                  onPageChange={() => {}}
                  filters={{
                    title: { type: 'text', label: 'Title', placeholder: 'Search title' },
                  }}
                  onFilterChange={filterData}
                />
              </div>
            }
            right={sidebar}
          />
        )}
      </div>

      <ConfirmModal
        isOpen={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete Organization"
        message={orgToDelete ? `Are you sure you want to delete the organization "${orgToDelete.title}"?` : ''}
        confirmLabel="Delete"
        isLoading={isDeleting}
        variant="danger"
      />
    </div>
  );
};

export default OrganizationsDashboard;
