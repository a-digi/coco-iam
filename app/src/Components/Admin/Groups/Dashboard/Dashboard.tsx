import React, { useEffect, useState } from 'react';
import { AdminGroupResource, type Group, GroupSchema } from '../model/group';
import { formatDateOnly } from '../../../../config/data/date/date';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import { TwoColumnsLeft } from '../../../../Shared/Components/Columns';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Title from '../../../../Shared/Components/Font/Title.tsx';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useNavigate } from 'react-router-dom';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { StatCard, type StatCardColor } from '../../../../Shared/Components/Cards';
import { useResourceRepository } from '../../../../config/data/resource/repository';
import type { ResourceFilter } from '../../../../config/data/resource/filters';
import NoEntriesFound from '../../../../Shared/Components/NoEntries/NoEntriesFound';
import { Alert } from '../../../../Shared/Components/Alert';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

const AdminGroupsDashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Groups' }]);

  const [groups, setGroups] = useState<Group[]>([]);
  const navigate = useNavigate();
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [groupToDelete, setGroupToDelete] = useState<{ id: string; title: string } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const { del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const { fetchCollection } = useResourceRepository();

  const fetchGroups = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mappedGroups = await fetchCollection<Group>(`admin/{${AdminGroupResource}}`, GroupSchema, { filters: currentFilters });
      setGroups(mappedGroups);
    } catch {
      const msg = 'Failed to fetch groups';
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
      void fetchGroups();
    }
  }, [fetchGroups, currentFilters]);

  const promptDelete = React.useCallback((id: string, title: string) => {
    setGroupToDelete({ id, title });
    setIsConfirmOpen(true);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!groupToDelete) return;
    setIsDeleting(true);
    try {
      await del(`admin/{${AdminGroupResource}}/{id:${groupToDelete.id}}`);
      successMessage(`Group ${groupToDelete.title} deleted successfully!`);
      void fetchGroups();
      setIsConfirmOpen(false);
    } catch {
      errorMessage('Failed to delete group');
    } finally {
      setIsDeleting(false);
    }
  }, [del, fetchGroups, successMessage, errorMessage, groupToDelete]);

  const columns = React.useMemo<TableColumn<Group>[]>(() => [
    {
      key: 'title',
      label: 'Group',
      render: (value, row) => (
        <div className="flex items-center gap-2">
          <span className="font-semibold text-gray-900 dark:text-gray-100">{String(value)}</span>
          {row.groupType && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300">
              {row.groupType}
            </span>
          )}
        </div>
      ),
    },
    {
      key: 'groupDescription',
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
          <EditAction to={`/admin/groups/edit/${row.id}`} />
          <DeleteAction onClick={() => promptDelete(row.id, row.title)} />
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

  const totalGroups = groups.length;
  const activeGroups = groups.filter(g => g.isActive).length;
  const groupTypes = [...new Set(groups.map(g => g.groupType).filter(Boolean))];

  const statCards: { label: string; value: number; color: StatCardColor }[] = [
    { label: 'Total Groups', value: totalGroups,                color: 'blue' },
    { label: 'Active',       value: activeGroups,               color: 'teal' },
    { label: 'Inactive',     value: totalGroups - activeGroups, color: 'amber' },
    { label: 'Group Types',  value: groupTypes.length,          color: 'violet' },
  ];

  const aboutAlert = (
    <Alert variant="info" title="About Admin Groups">
      Groups let you bundle permissions and assign them to multiple users at once. Each group has a type that defines its scope — for example <strong>role</strong> groups control access levels, while <strong>department</strong> groups organise users by team. Assign a user to a group on their profile page.
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
          <Title>Admin Groups</Title>
          <Submit
            type="button"
            onClick={() => navigate('/admin/groups/create')}
            label="Create Group"
          />
        </div>

        {/* Stats carousel */}
        <div className="flex overflow-x-auto snap-x snap-mandatory gap-4 pb-2">
          {statCards.map(c => (
            <div key={c.label} className="snap-start shrink-0 w-[44vw] sm:w-[32vw]">
              <StatCard label={c.label} value={c.value} color={c.color} />
            </div>
          ))}
        </div>

        {/* Group cards */}
        {loading ? (
          <div className="space-y-3">
            {[...Array(3)].map((_, i) => (
              <div key={i} className="h-[88px] rounded-xl bg-gray-100 dark:bg-surface-800 animate-pulse" />
            ))}
          </div>
        ) : error ? (
          <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
        ) : groups.length === 0 && currentFilters.length === 0 ? (
          <NoEntriesFound
            title="No Groups Found"
            message="There are currently no groups to display. Get started by creating your first group!"
          />
        ) : (
          <div className="space-y-3">
            {groups.map(group => (
              <div
                key={group.id}
                className="bg-white dark:bg-surface-800 rounded-xl border border-gray-200 dark:border-surface-700 p-4 flex items-start justify-between gap-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-semibold text-gray-900 dark:text-gray-100 truncate">
                      {group.title}
                    </span>
                    {group.groupType && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300 shrink-0">
                        {group.groupType}
                      </span>
                    )}
                    <span
                      className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold shrink-0 ${
                        group.isActive
                          ? 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300'
                          : 'bg-gray-100 text-gray-500 dark:bg-surface-700 dark:text-gray-400'
                      }`}
                    >
                      {group.isActive ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                  {group.groupDescription && (
                    <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">
                      {group.groupDescription}
                    </p>
                  )}
                  <p className="text-xs text-gray-400 dark:text-gray-500 mt-1.5">
                    {formatDateOnly(group.createdAt)}
                  </p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <EditAction to={`/admin/groups/edit/${group.id}`} />
                  <DeleteAction onClick={() => promptDelete(group.id, group.title)} />
                </div>
              </div>
            ))}
          </div>
        )}

        {aboutAlert}
      </div>

      {/* ── Desktop layout ────────────────────────────────── */}
      <div className="hidden md:block">
        <TwoColumnsLeft
          left={
            <div className="space-y-4">
              <div className="flex justify-between items-center mb-6">
                <Title>Admin Groups</Title>
                <Submit
                  type="button"
                  onClick={() => navigate('/admin/groups/create')}
                  label="Create Group"
                />
              </div>
              {loading && groups.length === 0 ? (
                <div className="space-y-3">
                  {[...Array(4)].map((_, i) => (
                    <div key={i} className="h-12 rounded-lg bg-gray-100 dark:bg-surface-800 animate-pulse" />
                  ))}
                </div>
              ) : groups.length === 0 && currentFilters.length === 0 && !loading ? (
                <NoEntriesFound
                  title="No Groups Found"
                  message="There are currently no groups to display. Get started by creating your first group!"
                />
              ) : (
                <TableView
                  columns={columns}
                  data={groups}
                  total={groups.length}
                  page={1}
                  pageSize={groups.length || 1}
                  onPageChange={() => { }}
                  filters={{
                    title: { type: 'text', label: 'Title', placeholder: 'Search title' },
                    group_type: { type: 'text', label: 'Group Type', placeholder: 'Search type' },
                  }}
                  onFilterChange={filterData}
                />
              )}
            </div>
          }
          right={sidebar}
        />
      </div>

      <ConfirmModal
        isOpen={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete Group"
        message={groupToDelete ? `Are you sure you want to delete the group "${groupToDelete.title}"?` : ''}
        confirmLabel="Delete"
        isLoading={isDeleting}
        variant="danger"
      />
    </div>
  );
};

export default AdminGroupsDashboard;
