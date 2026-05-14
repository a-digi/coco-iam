import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit, SubmitSmall } from '../../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';
import TableView from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { formatDate } from '../../../../config/data/date/date';
import type { EmailAccount } from './model/emailAccount';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';

export interface EmailAccountsManagerProps {
    className?: string;
    onEdit?: (id: string) => void;
    onCreate?: () => void;
}

const PAGE_SIZE = 20;

export const EmailAccountsManager: React.FC<EmailAccountsManagerProps> = ({
    className = '',
    onEdit,
    onCreate,
}) => {
    useBreadcrumbItems([{ label: 'Admin' }, { label: 'Settings' }, { label: 'Email Accounts' }]);
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();
    const navigate = useNavigate();

    const [items, setItems] = useState<EmailAccount[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);

    const [confirmOpen, setConfirmOpen] = useState(false);
    const [pending, setPending] = useState<{ id: string; name: string } | null>(null);
    const [deleting, setDeleting] = useState(false);

    const fetchList = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: EmailAccount[] }>('admin/mail/accounts');
            const body = response?.message;
            setItems(Array.isArray(body) ? body : []);
        } catch (err: unknown) {
            let msg = 'Failed to load email accounts';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, errorMessage]);

    useEffect(() => {
        void fetchList();
    }, [fetchList]);

    const pagedItems = useMemo(
        () => items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
        [items, page],
    );

    const handleEdit = useCallback((id: string) => {
        if (onEdit) onEdit(id);
        else navigate(`/admin/settings/email-accounts/edit/${id}`);
    }, [onEdit, navigate]);

    const handleCreate = useCallback(() => {
        if (onCreate) onCreate();
        else navigate('/admin/settings/email-accounts/create');
    }, [onCreate, navigate]);

    const activate = useCallback(async (id: string, name: string) => {
        try {
            await post(`admin/mail/accounts/{id:${id}}/activate`, {});
            successMessage(`Account ${name} is now active.`);
            void fetchList();
        } catch (err: unknown) {
            let msg = 'Failed to activate account';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        }
    }, [post, successMessage, errorMessage, fetchList]);

    const promptDelete = useCallback((id: string, name: string) => {
        setPending({ id, name });
        setConfirmOpen(true);
    }, []);

    const confirmDelete = useCallback(async () => {
        if (!pending) return;
        setDeleting(true);
        try {
            await del(`admin/mail/accounts/{id:${pending.id}}`);
            successMessage(`Account ${pending.name} deleted.`);
            setConfirmOpen(false);
            setPending(null);
            void fetchList();
        } catch (err: unknown) {
            let msg = 'Failed to delete account';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setDeleting(false);
        }
    }, [pending, del, successMessage, errorMessage, fetchList]);

    const columns = useMemo<TableColumn<EmailAccount>[]>(() => [
        {
            key: 'name',
            label: 'Name',
            render: (_v, row) => (
                <span className="font-mono text-sm flex items-center gap-2">
                    {row.name}
                    {row.is_active && (
                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200">
                            <svg className="w-3 h-3" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.286 3.957a1 1 0 00.95.69h4.162c.969 0 1.371 1.24.588 1.81l-3.37 2.448a1 1 0 00-.364 1.118l1.287 3.957c.3.921-.755 1.688-1.54 1.118l-3.37-2.448a1 1 0 00-1.176 0l-3.37 2.448c-.784.57-1.838-.197-1.539-1.118l1.287-3.957a1 1 0 00-.364-1.118L2.05 9.384c-.783-.57-.38-1.81.588-1.81h4.162a1 1 0 00.95-.69l1.299-3.957z" />
                            </svg>
                            Active
                        </span>
                    )}
                </span>
            ),
        },
        { key: 'host', label: 'Host' },
        { key: 'port', label: 'Port' },
        { key: 'from_email', label: 'From' },
        { key: 'use_tls', label: 'TLS', render: (v) => v ? 'Yes' : 'No' },
        {
            key: 'updated_at',
            label: 'Updated',
            render: (v) => formatDate(v as string),
        },
        {
            key: 'id',
            label: 'Actions',
            render: (_v, row) => (
                <div className="flex items-center gap-2 justify-end">
                    {!row.is_active && (
                        <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                            <SubmitSmall type="button" onClick={() => void activate(row.id, row.name)}>Set active</SubmitSmall>
                        </ScopeBasedComponentAccess>
                    )}
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                        <EditAction onClick={() => handleEdit(row.id)} />
                    </ScopeBasedComponentAccess>
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                        <DeleteAction onClick={() => promptDelete(row.id, row.name)} disabled={row.is_active} />
                    </ScopeBasedComponentAccess>
                </div>
            ),
        },
    ], [activate, handleEdit, promptDelete]);

    return (
        <div className={className}>
            <div className="flex justify-between items-start flex-wrap gap-3 mb-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Email accounts</h3>
                    <p className="text-sm text-gray-500">
                        Named SMTP accounts. Exactly one is active at a time — the mail engine uses that one for every send.
                    </p>
                </div>
                <ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminMailSettingsWrite, AppScopes.AdminMailSettings, AppScopes.SuperAdmin]}>
                    <Submit type="button" onClick={handleCreate} label="Create account" />
                </ScopeBasedComponentAccess>
            </div>

            {loading && items.length === 0 ? (
                <div className="text-sm text-gray-500 py-2">Loading accounts…</div>
            ) : (
                <TableView
                    columns={columns}
                    data={pagedItems}
                    total={items.length}
                    page={page}
                    pageSize={PAGE_SIZE}
                    onPageChange={setPage}
                    onFilterChange={() => { /* client-side filtering disabled for accounts */ }}
                    rowKey={(row) => row.id}
                    emptyText="No email accounts yet. Create one to enable sending."
                />
            )}

            <ConfirmModal
                isOpen={confirmOpen}
                onClose={() => setConfirmOpen(false)}
                onConfirm={confirmDelete}
                title="Delete account"
                message={pending ? `Delete account "${pending.name}"? This cannot be undone.` : ''}
                confirmLabel="Delete"
                isLoading={deleting}
                variant="danger"
            />
        </div>
    );
};

export default EmailAccountsManager;
