import React, { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall } from '../../../../Shared/Components/Button/SubmitSmall';
import { type Workspace, WorkspaceSchema, WorkspaceResource } from '../../../Workspace/model/workspace';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { type ResourceFilter, buildFilterQueryString } from '../../../../config/data/resource/filters';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import NoEntriesFound from '../../../../Shared/Components/NoEntries/NoEntriesFound';

interface WorkspacesProps {
  organizationId: string;
}

export const Workspaces: React.FC<WorkspacesProps> = ({ organizationId }) => {
  const { get, del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [fetching, setFetching] = useState(true);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [toDelete, setToDelete] = useState<{ id: string; title: string } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const fetchWorkspaces = React.useCallback(async () => {
    if (!organizationId) return;
    setFetching(true);
    try {
      const qs = buildFilterQueryString([
        { field: 'organization_id', operator: 'exact', value: organizationId },
      ]);
      const response = await get<{ message?: unknown }>(`workspaces/{${WorkspaceResource}}?${qs}`);
      const data = response?.message || response || [];
      if (Array.isArray(data)) {
        const mapped = mapObjects(
          WorkspaceSchema,
          data as Record<string, unknown>[],
        ) as unknown as Workspace[];
        setWorkspaces(mapped);
      } else {
        setWorkspaces([]);
      }
    } catch (err: unknown) {
      let msg = 'Failed to load workspaces';
      if (err instanceof Error) msg = err.message || msg;
      errorMessage(msg);
    } finally {
      setFetching(false);
    }
  }, [organizationId, get, errorMessage]);

  useEffect(() => {
    void fetchWorkspaces();
  }, [fetchWorkspaces]);

  const promptDelete = React.useCallback((id: string, title: string) => {
    setToDelete({ id, title });
    setIsConfirmOpen(true);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!toDelete) return;
    setIsDeleting(true);
    try {
      await del(`workspaces/{${WorkspaceResource}}/{id:${toDelete.id}}`);
      successMessage(`Workspace ${toDelete.title} deleted successfully!`);
      setIsConfirmOpen(false);
      setToDelete(null);
      void fetchWorkspaces();
    } catch (err: unknown) {
      let msg = 'Failed to delete workspace';
      if (err instanceof Error) msg = err.message || msg;
      errorMessage(msg);
    } finally {
      setIsDeleting(false);
    }
  }, [del, fetchWorkspaces, successMessage, errorMessage, toDelete]);

  const visibleWorkspaces = useMemo(() => {
    if (currentFilters.length === 0) return workspaces;
    return workspaces.filter(w =>
      currentFilters.every(filter => {
        const value = w[filter.field as keyof Workspace];
        if (!value) return false;
        const strValue = String(value).toLowerCase();
        const searchStr = String(filter.value).toLowerCase();
        return filter.operator === 'like'
          ? strValue.includes(searchStr)
          : strValue === searchStr;
      }),
    );
  }, [workspaces, currentFilters]);

  const filterData = React.useCallback((values: FilteredValue[]) => {
    const newFilters: ResourceFilter[] = [];
    if (values.length > 0) {
      const activeFilters = values[0];
      Object.entries(activeFilters).forEach(([key, val]) => {
        if (val === undefined || val === null || val === '') return;
        newFilters.push({
          field: String(key),
          operator: 'like',
          value: String(val),
        });
      });
    }
    setCurrentFilters(newFilters);
  }, []);

  const columns = useMemo<TableColumn<Workspace>[]>(() => [
    { key: 'title', label: 'Title' },
    { key: 'description', label: 'Description' },
    {
      key: 'id',
      label: 'Action',
      render: (_value, row) => (
        <div className="flex justify-end gap-2">
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}
          >
            <EditAction to={`/workspaces/edit/${row.id}`} />
          </ScopeBasedComponentAccess>
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.WorkspacesDelete, AppScopes.Workspaces, AppScopes.SuperAdmin]}
          >
            <DeleteAction onClick={() => promptDelete(row.id, row.title)} />
          </ScopeBasedComponentAccess>
        </div>
      ),
    },
  ], [promptDelete]);

  if (fetching) {
    return <div className="text-sm text-gray-500 py-2">Loading workspaces...</div>;
  }

  if (workspaces.length === 0 && currentFilters.length === 0) {
    return (
      <div className="space-y-4">
        <div className="flex justify-between items-center">
          <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200">
            Organization Workspaces
          </h4>
          <ScopeBasedComponentAccess
            requiredScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}
          >
            <SubmitSmall
              onClick={() => navigate(`/organizations/${organizationId}/workspaces/create`)}
            >
              Create Workspace
            </SubmitSmall>
          </ScopeBasedComponentAccess>
        </div>
        <NoEntriesFound
          title="No Workspaces Found"
          message="There are no workspaces in this organization yet. Create the first one using the button above."
        />
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex justify-between items-start">
        <div>
          <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">
            Organization Workspaces
          </h4>
          <p className="text-sm text-gray-500 mb-4">
            Workspaces that belong to this organization.
          </p>
        </div>
        <ScopeBasedComponentAccess
          requiredScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}
        >
          <SubmitSmall
            onClick={() => navigate(`/organizations/${organizationId}/workspaces/create`)}
          >
            Create Workspace
          </SubmitSmall>
        </ScopeBasedComponentAccess>
      </div>

      <div>
        <TableView
          columns={columns}
          data={visibleWorkspaces}
          total={visibleWorkspaces.length}
          page={1}
          pageSize={visibleWorkspaces.length || 1}
          onPageChange={() => { }}
          filters={{
            title: { type: 'text', label: 'Title', placeholder: 'Search title' },
          }}
          onFilterChange={filterData}
          emptyText="No workspaces belong to this organization yet."
        />
      </div>

      <ConfirmModal
        isOpen={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete Workspace"
        message={toDelete ? `Are you sure you want to delete the workspace "${toDelete.title}"?` : ''}
        confirmLabel="Delete"
        isLoading={isDeleting}
        variant="danger"
      />
    </div>
  );
};

export default Workspaces;
