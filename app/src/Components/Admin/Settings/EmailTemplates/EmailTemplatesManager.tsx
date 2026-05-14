import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import { type ResourceFilter } from '../../../../config/data/resource/filters';
import { formatDate } from '../../../../config/data/date/date';
import {
  type EmailTemplate,
  EmailTemplateSchema,
} from './model/emailTemplate';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export interface EmailTemplatesManagerProps {
  /** Extra classes applied to the outer container. */
  className?: string;
  /**
   * Override navigation when the user edits a template. Defaults to routing
   * to `/admin/settings/email-templates/edit/:id`.
   */
  onEdit?: (id: string) => void;
  /**
   * Override navigation for Create. Defaults to routing to
   * `/admin/settings/email-templates/create`.
   */
  onCreate?: () => void;
}

interface ListResponse {
  items?: unknown[];
}

const PAGE_SIZE = 20;

export const EmailTemplatesManager: React.FC<EmailTemplatesManagerProps> = ({
  className = '',
  onEdit,
  onCreate,
}) => {
  useBreadcrumbItems([{ label: 'Admin' }, { label: 'Settings' }, { label: 'Email Templates' }]);
  const { get, del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const [items, setItems] = useState<EmailTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);

  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pending, setPending] = useState<{ id: string; name: string } | null>(null);
  const [deleting, setDeleting] = useState(false);

  // Single fetch on mount (and after create/edit/delete). Filtering and
  // pagination happen client-side from the fetched slice — matches the
  // pattern of the other dashboards in this project and keeps the
  // useEffect dep graph free of array-ref churn.
  const fetchList = useCallback(async () => {
    setLoading(true);
    try {
      const response = await get<{ message?: ListResponse }>('admin/mail/templates?limit=500');
      const body = response?.message ?? {};
      const raw = Array.isArray(body.items) ? body.items : [];
      setItems(mapObjects(EmailTemplateSchema, raw as object[]) as unknown as EmailTemplate[]);
    } catch (err: unknown) {
      let msg = 'Failed to load email templates';
      if (err instanceof Error) msg = err.message || msg;
      errorMessage(msg);
    } finally {
      setLoading(false);
    }
  }, [get, errorMessage]);

  useEffect(() => {
    void fetchList();
  }, [fetchList]);

  const filteredItems = useMemo(() => {
    if (currentFilters.length === 0) return items;
    return items.filter(t => currentFilters.every(f => {
      const raw = t[f.field as keyof EmailTemplate];
      if (raw == null) return false;
      const hay = String(raw).toLowerCase();
      const needle = String(f.value).toLowerCase();
      return f.operator === 'like' ? hay.includes(needle) : hay === needle;
    }));
  }, [items, currentFilters]);

  const pagedItems = useMemo(
    () => filteredItems.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
    [filteredItems, page],
  );

  useEffect(() => {
    const totalPages = Math.max(1, Math.ceil(filteredItems.length / PAGE_SIZE));
    if (page > totalPages) setPage(totalPages);
  }, [filteredItems.length, page]);

  const filterData = useCallback((values: FilteredValue[]) => {
    const next: ResourceFilter[] = [];
    if (values.length > 0) {
      Object.entries(values[0]).forEach(([key, val]) => {
        if (val === undefined || val === null || val === '') return;
        next.push({ field: key, operator: 'like', value: String(val) });
      });
    }
    // Short-circuit the very common empty → empty transition so we don't
    // churn references (TableView's debounced filter fires on mount with
    // an empty filteredValues map).
    setCurrentFilters(prev => {
      if (prev.length === 0 && next.length === 0) return prev;
      if (
        prev.length === next.length &&
        prev.every((p, i) => p.field === next[i].field && p.value === next[i].value)
      ) {
        return prev;
      }
      return next;
    });
    setPage(1);
  }, []);

  const handleEdit = useCallback((id: string) => {
    if (onEdit) onEdit(id);
    else navigate(`/admin/settings/email-templates/edit/${id}`);
  }, [onEdit, navigate]);

  const handleCreate = useCallback(() => {
    if (onCreate) onCreate();
    else navigate('/admin/settings/email-templates/create');
  }, [onCreate, navigate]);

  const promptDelete = useCallback((id: string, name: string) => {
    setPending({ id, name });
    setConfirmOpen(true);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!pending) return;
    setDeleting(true);
    try {
      await del(`admin/mail/templates/{id:${pending.id}}`);
      successMessage(`Template ${pending.name} deleted.`);
      setConfirmOpen(false);
      setPending(null);
      void fetchList();
    } catch (err: unknown) {
      let msg = 'Failed to delete template';
      if (err instanceof Error) msg = err.message || msg;
      errorMessage(msg);
    } finally {
      setDeleting(false);
    }
  }, [pending, del, successMessage, errorMessage, fetchList]);

  const columns = useMemo<TableColumn<EmailTemplate>[]>(() => [
    { key: 'name', label: 'Name', render: (v) => <span className="font-mono text-sm">{String(v)}</span> },
    { key: 'description', label: 'Description' },
    { key: 'subject', label: 'Subject' },
    {
      key: 'updatedAt',
      label: 'Updated',
      render: (v) => formatDate(v as string),
    },
    {
      key: 'id',
      label: 'Actions',
      render: (_v, row) => (
        <div className="flex items-center gap-2 justify-end">
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailTemplatesWrite, AppScopes.AdminMailTemplates, AppScopes.SuperAdmin]}>
            <EditAction onClick={() => handleEdit(row.id)} />
          </ScopeBasedComponentAccess>
          <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailTemplatesDelete, AppScopes.AdminMailTemplates, AppScopes.SuperAdmin]}>
            <DeleteAction onClick={() => promptDelete(row.id, row.name)} />
          </ScopeBasedComponentAccess>
        </div>
      ),
    },
  ], [handleEdit, promptDelete]);

  return (
    <div className={className}>
      <div className="flex justify-between items-start flex-wrap gap-3 mb-4">
        <div>
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Email templates</h3>
          <p className="text-sm text-gray-500">Manage reusable email bodies used by the mail engine. Edits apply on the next send — no process restart needed.</p>
        </div>
      </div>
      <div className='mt-6 mb-6'>
        <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailTemplatesWrite, AppScopes.AdminMailTemplates, AppScopes.SuperAdmin]}>
          <Submit type="button" onClick={handleCreate} label="Create template" />
        </ScopeBasedComponentAccess>
      </div>

      {loading && items.length === 0 ? (
        <div className="text-sm text-gray-500 py-2">Loading templates...</div>
      ) : (
        <TableView
          columns={columns}
          data={pagedItems}
          total={filteredItems.length}
          page={page}
          pageSize={PAGE_SIZE}
          onPageChange={setPage}
          filters={{
            name: { type: 'text', label: 'Name', placeholder: 'Search name' },
            description: { type: 'text', label: 'Description', placeholder: 'Search description' },
          }}
          onFilterChange={filterData}
          rowKey={(row) => row.id}
          emptyText={items.length === 0
            ? 'No templates yet. Create one to reuse across outbound emails.'
            : 'No templates match the current filter.'}
        />
      )}

      <ConfirmModal
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={confirmDelete}
        title="Delete template"
        message={pending ? `Delete template "${pending.name}"? This cannot be undone.` : ''}
        confirmLabel="Delete"
        isLoading={deleting}
        variant="danger"
      />
    </div>
  );
};

export default EmailTemplatesManager;
