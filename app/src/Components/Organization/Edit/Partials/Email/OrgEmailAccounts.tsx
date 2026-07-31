import React, { useCallback, useEffect, useMemo, useState, type SyntheticEvent } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit, SubmitSmall, Cancel } from '../../../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../../Shared/Components/Modal';
import TableView from '../../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import { Switch } from '../../../../../Shared/Components/Switch';
import { FormInput } from '../../../../../Shared/Components/Form';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';
import { formatDate } from '../../../../../config/data/date/date';
import { OrganizationResource } from '../../../model/organization';
import { type EmailAccount, ACCOUNT_NAME_PATTERN, EMPTY_ACCOUNT } from '../../../../Admin/Settings/EmailAccounts/model/emailAccount';

interface Props {
    organizationId: string;
}

const WRITE_SCOPES = [AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin];

type Mode = 'list' | 'create' | 'edit';

/**
 * OrgEmailAccounts — the org-scoped equivalent of the global
 * Admin Settings → Email Accounts page. Inline list + create/edit
 * form (not separate routes) since this lives inside an Organization
 * Edit tab, not a top-level page. An org with no accounts of its own
 * falls back to the global active account at send time.
 */
export const OrgEmailAccounts: React.FC<Props> = ({ organizationId }) => {
    const { get, post, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const base = `organizations/{${OrganizationResource}}/{id:${organizationId}}/mail/accounts`;

    const [items, setItems] = useState<EmailAccount[]>([]);
    const [loading, setLoading] = useState(true);
    const [page, setPage] = useState(1);
    const PAGE_SIZE = 10;

    const [mode, setMode] = useState<Mode>('list');
    const [editingId, setEditingId] = useState<string | null>(null);

    const [confirmOpen, setConfirmOpen] = useState(false);
    const [pending, setPending] = useState<{ id: string; name: string } | null>(null);
    const [deleting, setDeleting] = useState(false);

    const fetchList = useCallback(async () => {
        setLoading(true);
        try {
            const response = await get<{ message?: EmailAccount[] }>(base);
            setItems(Array.isArray(response?.message) ? response.message : []);
        } catch (err: unknown) {
            let msg = 'Failed to load email accounts';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setLoading(false);
        }
    }, [get, base, errorMessage]);

    useEffect(() => {
        void fetchList();
    }, [fetchList]);

    const pagedItems = useMemo(
        () => items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
        [items, page],
    );

    const activate = useCallback(async (id: string, name: string) => {
        try {
            await post(`${base}/{accountId:${id}}/activate`, {});
            successMessage(`Account ${name} is now this organization's active account.`);
            void fetchList();
        } catch (err: unknown) {
            let msg = 'Failed to activate account';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        }
    }, [post, base, successMessage, errorMessage, fetchList]);

    const promptDelete = useCallback((id: string, name: string) => {
        setPending({ id, name });
        setConfirmOpen(true);
    }, []);

    const confirmDelete = useCallback(async () => {
        if (!pending) return;
        setDeleting(true);
        try {
            await del(`${base}/{accountId:${pending.id}}`);
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
    }, [pending, del, base, successMessage, errorMessage, fetchList]);

    const columns = useMemo<TableColumn<EmailAccount>[]>(() => [
        {
            key: 'name',
            label: 'Name',
            render: (_v, row) => (
                <span className="font-mono text-sm flex items-center gap-2">
                    {row.name}
                    {row.is_active && (
                        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-200">
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
        { key: 'updated_at', label: 'Updated', render: (v) => formatDate(v as string) },
        {
            key: 'id',
            label: 'Actions',
            render: (_v, row) => (
                <div className="flex items-center gap-2 justify-end">
                    {!row.is_active && (
                        <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                            <SubmitSmall type="button" onClick={() => void activate(row.id, row.name)}>Set active</SubmitSmall>
                        </ScopeBasedComponentAccess>
                    )}
                    <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                        <EditAction onClick={() => { setEditingId(row.id); setMode('edit'); }} />
                    </ScopeBasedComponentAccess>
                    <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                        <DeleteAction onClick={() => promptDelete(row.id, row.name)} disabled={row.is_active} />
                    </ScopeBasedComponentAccess>
                </div>
            ),
        },
    ], [activate, promptDelete]);

    if (mode !== 'list') {
        return (
            <OrgEmailAccountForm
                mode={mode}
                accountId={editingId}
                base={base}
                onDone={() => { setMode('list'); setEditingId(null); void fetchList(); }}
                onCancel={() => { setMode('list'); setEditingId(null); }}
            />
        );
    }

    return (
        <div className="mt-4">
            <div className="flex justify-between items-start flex-wrap gap-3 mb-4">
                <div>
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Email accounts</h3>
                    <p className="text-sm text-gray-500">
                        Named SMTP accounts for this organization. When none is active here, sends fall back to the
                        global active account.
                    </p>
                </div>
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Submit type="button" onClick={() => setMode('create')} label="Create account" />
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
                    emptyText="No email accounts for this organization yet."
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

interface FormProps {
    mode: 'create' | 'edit';
    accountId: string | null;
    base: string;
    onDone: () => void;
    onCancel: () => void;
}

const OrgEmailAccountForm: React.FC<FormProps> = ({ mode, accountId, base, onDone, onCancel }) => {
    const { get, post, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [account, setAccount] = useState<EmailAccount>(EMPTY_ACCOUNT);
    const [newPassword, setNewPassword] = useState('');
    const [fetching, setFetching] = useState(mode === 'edit');
    const [submitting, setSubmitting] = useState(false);
    const [testing, setTesting] = useState(false);
    const [testTo, setTestTo] = useState('');

    useEffect(() => {
        if (mode !== 'edit' || !accountId) return;
        let cancelled = false;
        (async () => {
            setFetching(true);
            try {
                const response = await get<{ message?: EmailAccount }>(`${base}/{accountId:${accountId}}`);
                if (!cancelled && response?.message) setAccount(response.message);
            } catch (err: unknown) {
                let msg = 'Failed to load account';
                if (err instanceof Error) msg = err.message || msg;
                errorMessage(msg);
            } finally {
                if (!cancelled) setFetching(false);
            }
        })();
        return () => { cancelled = true; };
    }, [mode, accountId, base, get, errorMessage]);

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        const name = account.name.trim();
        if (mode === 'create') {
            if (!name) { errorMessage('Name is required'); return; }
            if (!ACCOUNT_NAME_PATTERN.test(name)) {
                errorMessage('Name must start with a letter and contain only lowercase letters, digits, underscore or hyphen.');
                return;
            }
        }
        if (!account.host.trim()) { errorMessage('Host is required'); return; }
        if (account.port < 1 || account.port > 65535) { errorMessage('Port must be between 1 and 65535.'); return; }

        setSubmitting(true);
        try {
            if (mode === 'create') {
                await post(base, {
                    name, host: account.host, port: account.port, username: account.username,
                    password: newPassword, from_name: account.from_name, from_email: account.from_email,
                    use_tls: account.use_tls, is_active: account.is_active,
                });
                successMessage(`Account ${name} created.`);
            } else if (accountId) {
                const body: Record<string, unknown> = {
                    host: account.host, port: account.port, username: account.username,
                    from_name: account.from_name, from_email: account.from_email, use_tls: account.use_tls,
                };
                if (newPassword !== '') body.password = newPassword;
                await patch(`${base}/{accountId:${accountId}}`, body);
                successMessage(`Account ${name} updated.`);
            }
            onDone();
        } catch (err: unknown) {
            let msg = mode === 'create' ? 'Failed to create account' : 'Failed to update account';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [mode, account, newPassword, accountId, base, post, patch, successMessage, errorMessage, onDone]);

    const handleTest = useCallback(async () => {
        if (mode !== 'edit' || !accountId) return;
        const to = testTo.trim();
        if (!to) { errorMessage('Enter a recipient email for the test.'); return; }
        setTesting(true);
        try {
            await post(`${base}/{accountId:${accountId}}/test`, { to, name: 'admin' });
            successMessage(`Test email sent to ${to} via ${account.name}.`);
        } catch (err: unknown) {
            let msg = 'SMTP test failed';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setTesting(false);
        }
    }, [mode, accountId, base, testTo, account.name, post, successMessage, errorMessage]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-4">Loading account…</div>;
    }

    return (
        <div className="max-w-3xl mt-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                {mode === 'create' ? 'Create email account' : `Edit account: ${account.name}`}
            </h3>

            <form onSubmit={handleSubmit} className="mt-6 space-y-5">
                <FormInput
                    id="name"
                    label="Name"
                    value={account.name}
                    onChange={v => setAccount(a => ({ ...a, name: v }))}
                    disabled={mode === 'edit'}
                    required
                    placeholder="e.g. org-transactional"
                    description="Lowercase letters, digits, underscore, hyphen; must start with a letter. Cannot be renamed after creation."
                    inputClassName="font-mono text-sm"
                />

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <FormInput id="host" label="Host" value={account.host} onChange={v => setAccount(a => ({ ...a, host: v }))} required placeholder="smtp.example.com" />
                    <FormInput id="port" type="number" label="Port" value={account.port} onChange={v => setAccount(a => ({ ...a, port: parseInt(v, 10) || 0 }))} min={1} max={65535} />
                    <FormInput id="username" label="Username" value={account.username} onChange={v => setAccount(a => ({ ...a, username: v }))} placeholder="(optional)" />
                    <FormInput id="newPassword" type="password" label="Password" value={newPassword} onChange={setNewPassword} placeholder={mode === 'edit' ? 'Leave blank to keep the stored password' : ''} autoComplete="new-password" />
                    <FormInput id="fromName" label="From name" value={account.from_name} onChange={v => setAccount(a => ({ ...a, from_name: v }))} placeholder="Org Support" />
                    <FormInput id="fromEmail" type="email" label="From email" value={account.from_email} onChange={v => setAccount(a => ({ ...a, from_email: v }))} placeholder="noreply@example.com" />
                </div>

                <div className="flex items-center gap-6">
                    <Switch checked={account.use_tls} onChange={v => setAccount(a => ({ ...a, use_tls: v }))} label="Use TLS" />
                    {mode === 'create' && (
                        <Switch checked={account.is_active} onChange={v => setAccount(a => ({ ...a, is_active: v }))} label="Activate immediately" />
                    )}
                </div>

                {mode === 'edit' && (
                    <div className="p-4 border border-gray-200 dark:border-surface-800 rounded-lg bg-gray-50 dark:bg-surface-900/40">
                        <div className="flex items-center justify-between gap-3 flex-wrap">
                            <div>
                                <div className="text-sm font-medium text-gray-800 dark:text-gray-200">Send a test email</div>
                                <div className="text-xs text-gray-500">Uses this account regardless of which one is active.</div>
                            </div>
                            <div className="flex items-center gap-2">
                                <input
                                    type="email"
                                    value={testTo}
                                    onChange={e => setTestTo(e.target.value)}
                                    placeholder="recipient@example.com"
                                    className="px-3 py-2 border border-gray-300 dark:border-surface-700 rounded-lg bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-400"
                                />
                                <SubmitSmall type="button" onClick={handleTest} disabled={testing}>
                                    {testing ? 'Sending…' : 'Send test'}
                                </SubmitSmall>
                            </div>
                        </div>
                    </div>
                )}

                <div className="flex items-center justify-end gap-3 pt-2">
                    <Cancel onClick={onCancel} />
                    <Submit loading={submitting} loadingText="Saving…" label={mode === 'create' ? 'Create account' : 'Save changes'} />
                </div>
            </form>
        </div>
    );
};

export default OrgEmailAccounts;
