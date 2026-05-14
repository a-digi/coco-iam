import React, { useEffect, useState } from 'react';
import { type User, StandardSchema } from '../model/user';
import { CardView } from '../../../../Shared/Components/CardView';
import { formatDateOnly } from '../../../../config/data/date/date';
import type { FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import Title from '../../../../Shared/Components/Font/Title.tsx';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useNavigate } from 'react-router-dom';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import { DotsDropdown } from '../../../../Shared/Components/DotsDropdown';
import { useResourceRepository } from '../../../../config/data/resource/repository';
import type { ResourceFilter } from '../../../../config/data/resource/filters';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

const AdminUsersDashboard: React.FC = () => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Users' }]);
  const [users, setUsers] = useState<User[]>([]);
  const navigate = useNavigate();
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [userToDelete, setUserToDelete] = useState<{ id: string; username: string } | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const { del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const { fetchCollection } = useResourceRepository();

  const fetchUsers = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const mappedUsers = await fetchCollection<User>('admin/{res:users}', StandardSchema, { filters: currentFilters });
      setUsers(mappedUsers);
    } catch {
      const msg = 'Failed to fetch users';
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
      void fetchUsers();
    }
  }, [fetchUsers, currentFilters]);

  const promptDelete = React.useCallback((id: string, username: string) => {
    setUserToDelete({ id, username });
    setIsConfirmOpen(true);
  }, []);

  const confirmDelete = React.useCallback(async () => {
    if (!userToDelete) return;
    setIsDeleting(true);
    try {
      await del(`admin/{res:users}/{id:${userToDelete.id}}`);
      successMessage(`User ${userToDelete.username} deleted successfully!`);
      void fetchUsers();
      setIsConfirmOpen(false);
    } catch (e: unknown) {
      errorMessage(e instanceof Error ? e.message : 'Failed to delete user');
      setIsConfirmOpen(false);
    } finally {
      setIsDeleting(false);
    }
  }, [del, fetchUsers, successMessage, errorMessage, userToDelete]);

  const columns = React.useMemo<TableColumn<User>[]>(() => [
    {
      key: 'username',
      label: '',
      render: (value, row) => (
        <div className="flex items-center gap-2">
          <span className="font-semibold text-gray-900 dark:text-gray-100">{String(value)}</span>
          <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold ${row.isSuperAdmin
            ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300'
            : 'bg-gray-100 text-gray-600 dark:bg-surface-700 dark:text-gray-400'
            }`}>
            {row.isSuperAdmin ? 'Super Admin' : 'User'}
          </span>
        </div>
      ),
    },
    {
      key: 'email',
      label: 'Email',
      render: (value) => (
        <span className="text-sm text-gray-700 dark:text-gray-300 break-all">{String(value)}</span>
      ),
    },
    {
      key: 'createdAt',
      label: 'Member since',
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
              href: `/admin/users/edit/${row.id}`,
              icon: (
                <svg fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
                </svg>
              ),
            },
            {
              label: 'Delete',
              variant: 'danger',
              onClick: () => promptDelete(row.id, row.username),
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
  ], [promptDelete]);

  const filterData = React.useCallback((values: FilteredValue[]) => {
    const newFilters: ResourceFilter[] = [];
    if (values.length > 0) {
      const activeFilters = values[0];
      Object.entries(activeFilters).forEach(([key, val]) => {
        if (val === undefined || val === null || val === '') return;
        newFilters.push({
          field: String(key),
          operator: key === 'role' ? 'exact' : 'like',
          value: String(val),
        });
      });
    }
    setCurrentFilters(newFilters);
  }, []);

  if (loading && users.length === 0) return <div>Loading users...</div>;
  if (error) return <div>Error: {error}</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <Title>Admin Users</Title>
        <Submit
          type="button"
          onClick={() => navigate('/admin/users/create')}
          label="Create User"
        />
      </div>

      <CardView
        columns={columns}
        data={users}
        total={users.length}
        page={1}
        pageSize={users.length || 1}
        onPageChange={() => { }}
        filters={{
          username: { type: 'text', label: 'Username', placeholder: 'Search username' },
          email: { type: 'text', label: 'Email', placeholder: 'Search email' },
          is_super_admin: {
            type: 'select',
            label: 'Role',
            options: [
              { label: 'All roles', value: '' },
              { label: 'Super Admin', value: '1' },
              { label: 'User', value: '0' },
            ],
            placeholder: 'Select role',
          },
        }}
        onFilterChange={filterData}
        rowKey={(row) => row.id}
        actionsKey="id"
      />

      <ConfirmModal
        isOpen={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete User"
        message={userToDelete ? `Are you sure you want to delete the user "${userToDelete.username}"?` : ''}
        confirmLabel="Delete"
        isLoading={isDeleting}
        variant="danger"
      />
    </div>
  );
};

export default AdminUsersDashboard;
