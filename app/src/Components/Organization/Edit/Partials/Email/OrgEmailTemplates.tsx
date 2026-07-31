import React, { useCallback, useEffect, useMemo, useState, type SyntheticEvent } from 'react';
import { useHttpClient } from '../../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit, Cancel } from '../../../../../Shared/Components/Button';
import { EditAction, DeleteAction } from '../../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../../Shared/Components/Modal';
import TableView from '../../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../../Shared/Components/Table/Table';
import { Switch } from '../../../../../Shared/Components/Switch';
import { FormInput, FormTextarea } from '../../../../../Shared/Components/Form';
import ScopeBasedComponentAccess from '../../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../../config/security/scopes';
import { formatDate } from '../../../../../config/data/date/date';
import { mapObjects } from '../../../../../config/data/mapper/mapper';
import { OrganizationResource } from '../../../model/organization';
import { type EmailTemplate, EmailTemplateSchema, TEMPLATE_NAME_PATTERN } from '../../../../Admin/Settings/EmailTemplates/model/emailTemplate';

interface Props {
    organizationId: string;
}

const WRITE_SCOPES = [AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin];

type Mode = 'list' | 'create' | 'edit';

interface TemplatesListResponse {
    items?: unknown[];
    total?: number;
}

/**
 * OrgEmailTemplates — the org-scoped equivalent of the global
 * Admin Settings → Email Templates page. An org template with the
 * same name as a system event (e.g. "user_invite") is used instead of
 * the global one whenever this org's mail settings bind that event —
 * see api/src/mail/scopedsettings.ScopedResolver.RenderTemplate.
 */
export const OrgEmailTemplates: React.FC<Props> = ({ organizationId }) => {
    const { get, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const base = `organizations/{${OrganizationResource}}/{id:${organizationId}}/mail/templates`;

    const [items, setItems] = useState<EmailTemplate[]>([]);
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
            const response = await get<{ message?: TemplatesListResponse }>(`${base}?limit=500`);
            const rawItems = Array.isArray(response?.message?.items) ? response.message!.items : [];
            const mapped = mapObjects(EmailTemplateSchema, rawItems as object[]) as unknown as EmailTemplate[];
            setItems(mapped);
        } catch (err: unknown) {
            let msg = 'Failed to load email templates';
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

    const promptDelete = useCallback((id: string, name: string) => {
        setPending({ id, name });
        setConfirmOpen(true);
    }, []);

    const confirmDelete = useCallback(async () => {
        if (!pending) return;
        setDeleting(true);
        try {
            await del(`${base}/{templateId:${pending.id}}`);
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
    }, [pending, del, base, successMessage, errorMessage, fetchList]);

    const columns = useMemo<TableColumn<EmailTemplate>[]>(() => [
        { key: 'name', label: 'Name', render: (_v, row) => <span className="font-mono text-sm">{row.name}</span> },
        { key: 'description', label: 'Description' },
        { key: 'isActive', label: 'Active', render: (v) => v ? 'Yes' : 'No' },
        { key: 'updatedAt', label: 'Updated', render: (v) => formatDate(v as string) },
        {
            key: 'id',
            label: 'Actions',
            render: (_v, row) => (
                <div className="flex items-center gap-2 justify-end">
                    <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                        <EditAction onClick={() => { setEditingId(row.id); setMode('edit'); }} />
                    </ScopeBasedComponentAccess>
                    <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                        <DeleteAction onClick={() => promptDelete(row.id, row.name)} />
                    </ScopeBasedComponentAccess>
                </div>
            ),
        },
    ], [promptDelete]);

    if (mode !== 'list') {
        return (
            <OrgEmailTemplateForm
                mode={mode}
                templateId={editingId}
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
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Email templates</h3>
                    <p className="text-sm text-gray-500">
                        Name a template after a system event (e.g. <code className="font-mono">user_invite</code>) and
                        bind it under Settings to override the global template for this organization.
                    </p>
                </div>
                <ScopeBasedComponentAccess requiredScopes={WRITE_SCOPES}>
                    <Submit type="button" onClick={() => setMode('create')} label="Create template" />
                </ScopeBasedComponentAccess>
            </div>

            {loading && items.length === 0 ? (
                <div className="text-sm text-gray-500 py-2">Loading templates…</div>
            ) : (
                <TableView
                    columns={columns}
                    data={pagedItems}
                    total={items.length}
                    page={page}
                    pageSize={PAGE_SIZE}
                    onPageChange={setPage}
                    onFilterChange={() => { /* client-side filtering disabled */ }}
                    rowKey={(row) => row.id}
                    emptyText="No email templates for this organization yet."
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

interface FormProps {
    mode: 'create' | 'edit';
    templateId: string | null;
    base: string;
    onDone: () => void;
    onCancel: () => void;
}

const OrgEmailTemplateForm: React.FC<FormProps> = ({ mode, templateId, base, onDone, onCancel }) => {
    const { get, post, patch } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [subject, setSubject] = useState('');
    const [content, setContent] = useState('');
    const [isHtml, setIsHtml] = useState(true);
    const [isActive, setIsActive] = useState(true);
    const [fetching, setFetching] = useState(mode === 'edit');
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        if (mode !== 'edit' || !templateId) return;
        let cancelled = false;
        (async () => {
            setFetching(true);
            try {
                const response = await get<{ message?: unknown }>(`${base}/{templateId:${templateId}}`);
                const raw = response?.message;
                if (!raw) { errorMessage('Template not found'); return; }
                const mapped = mapObjects(EmailTemplateSchema, [raw]) as unknown as EmailTemplate[];
                const t = mapped[0];
                if (cancelled || !t) return;
                setName(t.name);
                setDescription(t.description);
                setSubject(t.subject);
                if (t.htmlBody) { setContent(t.htmlBody); setIsHtml(true); }
                else { setContent(t.textBody); setIsHtml(false); }
                setIsActive(t.isActive);
            } catch (err: unknown) {
                let msg = 'Failed to load template';
                if (err instanceof Error) msg = err.message || msg;
                errorMessage(msg);
            } finally {
                if (!cancelled) setFetching(false);
            }
        })();
        return () => { cancelled = true; };
    }, [mode, templateId, base, get, errorMessage]);

    const handleSubmit = useCallback(async (e: SyntheticEvent) => {
        e.preventDefault();
        if (mode === 'create') {
            const trimmed = name.trim();
            if (!trimmed) { errorMessage('Name is required'); return; }
            if (!TEMPLATE_NAME_PATTERN.test(trimmed)) {
                errorMessage('Name must start with a letter and contain only lowercase letters, digits, underscore or hyphen.');
                return;
            }
        }
        if (!content.trim()) { errorMessage('Content cannot be empty.'); return; }

        const textBody = isHtml ? '' : content;
        const htmlBody = isHtml ? content : '';

        setSubmitting(true);
        try {
            if (mode === 'create') {
                await post(base, {
                    name: name.trim(), description, subject, text_body: textBody, html_body: htmlBody, is_active: isActive,
                });
                successMessage(`Template ${name} created.`);
            } else if (templateId) {
                await patch(`${base}/{templateId:${templateId}}`, {
                    description, subject, text_body: textBody, html_body: htmlBody, is_active: isActive,
                });
                successMessage(`Template ${name} updated.`);
            }
            onDone();
        } catch (err: unknown) {
            let msg = mode === 'create' ? 'Failed to create template' : 'Failed to update template';
            if (err instanceof Error) msg = err.message || msg;
            errorMessage(msg);
        } finally {
            setSubmitting(false);
        }
    }, [mode, name, description, subject, content, isHtml, isActive, templateId, base, post, patch, successMessage, errorMessage, onDone]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-4">Loading template…</div>;
    }

    return (
        <div className="max-w-3xl mt-4">
            <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                {mode === 'create' ? 'Create email template' : `Edit template: ${name}`}
            </h3>

            <form onSubmit={handleSubmit} className="mt-6 space-y-5">
                <FormInput
                    id="name"
                    label="Name"
                    value={name}
                    onChange={setName}
                    disabled={mode === 'edit'}
                    required
                    placeholder="e.g. user_invite"
                    description="Lowercase letters, digits, underscore, hyphen; must start with a letter. Cannot be renamed after creation. Name it after a system event key to override that event's global template."
                    inputClassName="font-mono text-sm"
                />
                <FormInput id="description" label="Description" value={description} onChange={setDescription} />
                <FormInput id="subject" label="Subject" value={subject} onChange={setSubject} />

                <div>
                    <div className="flex items-center justify-between mb-1">
                        <label htmlFor="content" className="block text-sm font-medium text-gray-700 dark:text-gray-300">
                            Content {isHtml
                                ? <span className="text-xs text-indigo-600 dark:text-indigo-400 font-normal">(HTML)</span>
                                : <span className="text-xs text-gray-500 font-normal">(plain text)</span>}
                        </label>
                        <Switch checked={isHtml} onChange={setIsHtml} label={isHtml ? 'HTML' : 'Text'} />
                    </div>
                    <FormTextarea id="content" value={content} onChange={setContent} rows={16} textareaClassName="font-mono text-sm" />
                    <p className="text-xs text-gray-500 mt-1">
                        Use <code>{'{{ .Name }}'}</code> syntax for template variables.
                    </p>
                </div>

                <div className="flex items-center gap-3">
                    <Switch checked={isActive} onChange={setIsActive} />
                    <span className="text-sm text-gray-700 dark:text-gray-300">Active</span>
                </div>

                <div className="flex items-center justify-end gap-3 pt-2">
                    <Cancel onClick={onCancel} />
                    <Submit loading={submitting} loadingText="Saving…" label={mode === 'create' ? 'Create template' : 'Save changes'} />
                </div>
            </form>
        </div>
    );
};

export default OrgEmailTemplates;
