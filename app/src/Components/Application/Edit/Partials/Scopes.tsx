import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { SubmitSmall, Add, Submit } from '../../../../Shared/Components/Button';
import { DeleteAction } from '../../../../Shared/Components/Actions/DeleteAction';
import { EditAction } from '../../../../Shared/Components/Actions';
import { Close } from '../../../../Shared/Components/Button/Close';
import { FormInput } from '../../../../Shared/Components/Form';
import { Modal } from '../../../../Shared/Components/Modal';
import {
    type ApplicationScope,
    ApplicationScopeSchema,
    ApplicationScopeResource,
    SCOPE_ID_PATTERN,
    parseResourceIds,
} from '../../model/applicationScope';
import { mapObjects } from '../../../../config/data/mapper/mapper';
import TableView, { type FilteredValue } from '../../../../Shared/Components/TableView/TableView';
import type { TableColumn } from '../../../../Shared/Components/Table/Table';
import { type ResourceFilter, buildFilterQueryString } from '../../../../config/data/resource/filters';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';

interface ScopesProps {
    applicationId: string;
}

// csvToIds / idsToCsv normalise the admin's comma-separated input into
// the JSON-array shape the backend stores.
const csvToIds = (csv: string): string[] =>
    csv
        .split(',')
        .map(s => s.trim())
        .filter(Boolean);

const idsToCsv = (ids: string[]): string => ids.join(', ');

export const Scopes: React.FC<ScopesProps> = ({ applicationId }) => {
    const { get, post, patch, del } = useHttpClient();
    const { successMessage, errorMessage } = useSnackBar();

    const [scopes, setScopes] = useState<ApplicationScope[]>([]);
    const [loading, setLoading] = useState(false);
    const [fetching, setFetching] = useState(true);

    const [isFormVisible, setIsFormVisible] = useState(false);
    const [newScopeID, setNewScopeID] = useState('');
    const [newDescription, setNewDescription] = useState('');
    const [newResourceIDs, setNewResourceIDs] = useState('');

    const [editingScope, setEditingScope] = useState<ApplicationScope | null>(null);
    const [editDescription, setEditDescription] = useState('');
    const [editResourceIDs, setEditResourceIDs] = useState('');
    const [savingEdit, setSavingEdit] = useState(false);

    const [page, setPage] = useState(1);
    const [currentFilters, setCurrentFilters] = useState<ResourceFilter[]>([]);
    const PAGE_SIZE = 20;

    const fileInputRef = useRef<HTMLInputElement | null>(null);

    const fetchScopes = React.useCallback(async () => {
        if (!applicationId) return;
        setFetching(true);
        try {
            const qs = buildFilterQueryString([{ field: 'application_id', operator: 'exact', value: applicationId }]);
            const response = await get<{ message?: unknown }>(`applications/{${ApplicationScopeResource}}?${qs}`);
            const data = response?.message || response || [];
            if (Array.isArray(data)) {
                const mapped = mapObjects(ApplicationScopeSchema, data) as unknown as ApplicationScope[];
                setScopes(mapped);
            } else {
                setScopes([]);
            }
        } catch (err: unknown) {
            let errorMsg = 'Failed to load scopes';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setFetching(false);
        }
    }, [applicationId, get, errorMessage]);

    useEffect(() => {
        void fetchScopes();
    }, [fetchScopes]);

    const handleCreate = React.useCallback(async (e: React.FormEvent) => {
        e.preventDefault();
        const trimmed = newScopeID.trim();
        if (!trimmed) return;
        if (!SCOPE_ID_PATTERN.test(trimmed)) {
            errorMessage('Scope ID must match pattern: letters or underscores, separated by colons (e.g. "docs:read").');
            return;
        }
        setLoading(true);
        try {
            await post(`applications/{${ApplicationScopeResource}}`, {
                application_id: applicationId,
                scope_id: trimmed,
                description: newDescription,
                // Backend stores resource_ids as a JSON-string column.
                // Serialise on the wire so both paths (create + edit)
                // produce the same shape downstream.
                resource_ids: JSON.stringify(csvToIds(newResourceIDs)),
                is_active: true,
            });
            successMessage(`Scope ${trimmed} created.`);
            setNewScopeID('');
            setNewDescription('');
            setNewResourceIDs('');
            setIsFormVisible(false);
            void fetchScopes();
        } catch (err: unknown) {
            let errorMsg = 'Failed to create scope';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [post, applicationId, newScopeID, newDescription, newResourceIDs, successMessage, errorMessage, fetchScopes]);

    const openEdit = React.useCallback((row: ApplicationScope) => {
        setEditingScope(row);
        setEditDescription(row.description ?? '');
        setEditResourceIDs(idsToCsv(parseResourceIds(row.resourceIds)));
    }, []);

    const closeEdit = React.useCallback(() => {
        setEditingScope(null);
        setEditDescription('');
        setEditResourceIDs('');
    }, []);

    const handleEditSave = React.useCallback(async (e: React.FormEvent) => {
        e.preventDefault();
        if (!editingScope) return;
        setSavingEdit(true);
        try {
            await patch(`applications/{${ApplicationScopeResource}}/{id:${editingScope.id}}`, {
                description: editDescription,
                resource_ids: JSON.stringify(csvToIds(editResourceIDs)),
            });
            successMessage(`Scope ${editingScope.scopeId} updated.`);
            closeEdit();
            void fetchScopes();
        } catch (err: unknown) {
            let errorMsg = 'Failed to update scope';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setSavingEdit(false);
        }
    }, [editingScope, editDescription, editResourceIDs, patch, successMessage, errorMessage, fetchScopes, closeEdit]);

    const handleDelete = React.useCallback(async (id: string, scopeId: string) => {
        setLoading(true);
        try {
            await del(`applications/{${ApplicationScopeResource}}/{id:${id}}`);
            successMessage(`Scope ${scopeId} deleted.`);
            void fetchScopes();
        } catch (err: unknown) {
            let errorMsg = 'Failed to delete scope';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
        }
    }, [del, successMessage, errorMessage, fetchScopes]);

    const handleExport = React.useCallback(async () => {
        if (!applicationId) return;
        try {
            const response = await get<{ message?: unknown }>(`applications/{res:applications}/{id:${applicationId}}/scopes/export`);
            const tree = response?.message ?? [];
            const json = JSON.stringify(tree, null, 4);
            const blob = new Blob([json], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `application-scopes-${applicationId}.json`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            successMessage('Scopes exported.');
        } catch (err: unknown) {
            let errorMsg = 'Failed to export scopes';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        }
    }, [applicationId, get, successMessage, errorMessage]);

    const { post: postRaw } = useHttpClient();

    const handleImportFile = React.useCallback(async (file: File) => {
        if (!applicationId) return;
        setLoading(true);
        try {
            const text = await file.text();
            let tree: unknown;
            try {
                tree = JSON.parse(text);
            } catch {
                errorMessage('Selected file is not valid JSON.');
                return;
            }
            if (!Array.isArray(tree)) {
                errorMessage('Expected a JSON array at the top level (same shape as admin.json).');
                return;
            }
            const response = await postRaw<{ message?: { inserted: number; updated: number; skipped: number; total: number } }>(
                `applications/{res:applications}/{id:${applicationId}}/scopes/import`,
                tree,
            );
            const r = response?.message;
            if (r) {
                successMessage(`Imported ${r.total} scopes — ${r.inserted} inserted, ${r.updated} updated.`);
            } else {
                successMessage('Scopes imported.');
            }
            void fetchScopes();
        } catch (err: unknown) {
            let errorMsg = 'Failed to import scopes';
            if (err instanceof Error) errorMsg = err.message || errorMsg;
            errorMessage(errorMsg);
        } finally {
            setLoading(false);
            if (fileInputRef.current) fileInputRef.current.value = '';
        }
    }, [applicationId, postRaw, successMessage, errorMessage, fetchScopes]);

    const filteredScopes = useMemo(() => {
        if (currentFilters.length === 0) return scopes;
        return scopes.filter(s => currentFilters.every(f => {
            const raw = s[f.field as keyof ApplicationScope];
            if (raw == null) return false;
            const hay = String(raw).toLowerCase();
            const needle = String(f.value).toLowerCase();
            return f.operator === 'like' ? hay.includes(needle) : hay === needle;
        }));
    }, [scopes, currentFilters]);

    const pagedScopes = useMemo(
        () => filteredScopes.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE),
        [filteredScopes, page],
    );

    useEffect(() => {
        const totalPages = Math.max(1, Math.ceil(filteredScopes.length / PAGE_SIZE));
        if (page > totalPages) setPage(totalPages);
    }, [filteredScopes.length, page]);

    const filterData = React.useCallback((values: FilteredValue[]) => {
        const next: ResourceFilter[] = [];
        if (values.length > 0) {
            Object.entries(values[0]).forEach(([key, val]) => {
                if (val === undefined || val === null || val === '') return;
                next.push({ field: key, operator: 'like', value: String(val) });
            });
        }
        setCurrentFilters(next);
        setPage(1);
    }, []);

    const columns = useMemo<TableColumn<ApplicationScope>[]>(() => [
        { key: 'scopeId', label: 'Scope ID', render: (value) => <span className="font-mono text-sm">{String(value)}</span> },
        { key: 'description', label: 'Description' },
        {
            key: 'resourceIds',
            label: 'Resource IDs',
            render: (value) => {
                const ids = parseResourceIds(value);
                if (ids.length === 0) return <span className="text-xs text-gray-400 italic">unconstrained</span>;
                return (
                    <div className="flex flex-wrap gap-1 max-w-md">
                        {ids.slice(0, 5).map(id => (
                            <span key={id} className="inline-block px-2 py-0.5 text-xs rounded border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 font-mono">
                                {id}
                            </span>
                        ))}
                        {ids.length > 5 && (
                            <span className="inline-block px-2 py-0.5 text-xs rounded text-gray-500">+{ids.length - 5}</span>
                        )}
                    </div>
                );
            },
        },
        {
            key: 'id',
            label: 'Action',
            render: (_value, row) => (
                <div className="flex justify-end gap-2">
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsScopesWrite, AppScopes.ApplicationsScopes, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <EditAction onClick={() => openEdit(row)} disabled={loading} />
                    </ScopeBasedComponentAccess>
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsScopesDelete, AppScopes.ApplicationsScopes, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <DeleteAction onClick={() => handleDelete(row.id, row.scopeId)} disabled={loading} />
                    </ScopeBasedComponentAccess>
                </div>
            )
        },
    ], [handleDelete, openEdit, loading]);

    if (fetching) {
        return <div className="text-sm text-gray-500 py-2">Loading scopes...</div>;
    }

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-start flex-wrap gap-3">
                <div>
                    <h4 className="text-sm font-semibold text-gray-800 dark:text-gray-200 mb-2">Application Scopes</h4>
                    <p className="text-sm text-gray-500 mb-4">The vocabulary of permissions this application exposes. Scope IDs use the same format as admin scopes: letters/underscores per segment, separated by colons (e.g. <span className="font-mono">docs:read</span>). Optionally constrain a scope to a fixed set of entity IDs — any user holding the scope inherits the constraint.</p>
                </div>
                <div className="flex items-center gap-2">
                    <SubmitSmall type="button" onClick={handleExport} disabled={loading}>
                        Export
                    </SubmitSmall>
                    <ScopeBasedComponentAccess requiredScopes={[AppScopes.ApplicationsScopesWrite, AppScopes.ApplicationsScopes, AppScopes.Applications, AppScopes.SuperAdmin]}>
                        <>
                            <SubmitSmall type="button" onClick={() => fileInputRef.current?.click()} disabled={loading}>
                                Import
                            </SubmitSmall>
                            <input
                                ref={fileInputRef}
                                type="file"
                                accept="application/json,.json"
                                className="hidden"
                                onChange={(e) => {
                                    const f = e.target.files?.[0];
                                    if (f) void handleImportFile(f);
                                }}
                            />
                            {!isFormVisible && <Add onClick={() => setIsFormVisible(true)} />}
                        </>
                    </ScopeBasedComponentAccess>
                </div>
            </div>

            {isFormVisible && (
                <form onSubmit={handleCreate} className="space-y-4 p-4 border border-gray-200 dark:border-surface-800 rounded-md bg-gray-50 dark:bg-surface-900">
                    <div className="flex justify-between items-center">
                        <h5 className="text-sm font-medium text-gray-700 dark:text-gray-300">Define a new scope</h5>
                        <Close
                            onClick={() => {
                                setIsFormVisible(false);
                                setNewScopeID('');
                                setNewDescription('');
                                setNewResourceIDs('');
                            }}
                            label="Close"
                        />
                    </div>
                    <FormInput
                        id="newScopeID"
                        label="Scope ID"
                        value={newScopeID}
                        onChange={setNewScopeID}
                        required
                        placeholder="e.g. read, docs:write, admin:users:manage"
                        description="Letters and underscores per segment, colons between segments."
                        inputClassName="font-mono text-sm"
                    />
                    <FormInput
                        id="newDescription"
                        label="Description"
                        value={newDescription}
                        onChange={setNewDescription}
                    />
                    <FormInput
                        id="newResourceIDs"
                        label="Resource IDs"
                        value={newResourceIDs}
                        onChange={setNewResourceIDs}
                        placeholder="comma-separated ids (leave empty for unconstrained)"
                        description="Opaque strings — can be user ids, group ids, or anything your application interprets."
                        inputClassName="font-mono text-sm"
                    />
                    <div className="flex justify-end">
                        <SubmitSmall type="submit" disabled={loading || !newScopeID}>
                            Create Scope
                        </SubmitSmall>
                    </div>
                </form>
            )}

            <div>
                <TableView
                    columns={columns}
                    data={pagedScopes}
                    total={filteredScopes.length}
                    page={page}
                    pageSize={PAGE_SIZE}
                    onPageChange={setPage}
                    filters={{
                        scopeId: { type: 'text', label: 'Scope ID', placeholder: 'Search by scope id' },
                        description: { type: 'text', label: 'Description', placeholder: 'Search description' },
                    }}
                    onFilterChange={filterData}
                    emptyText={scopes.length === 0
                        ? 'No scopes defined for this application. Add one to start assigning permissions.'
                        : 'No scopes match the current filter.'}
                />
            </div>

            <Modal
                isOpen={!!editingScope}
                onClose={closeEdit}
                title={editingScope ? `Edit scope — ${editingScope.scopeId}` : 'Edit scope'}
            >
                <form onSubmit={handleEditSave} className="space-y-4">
                    <FormInput
                        id="editDescription"
                        label="Description"
                        value={editDescription}
                        onChange={setEditDescription}
                    />
                    <FormInput
                        id="editResourceIDs"
                        label="Resource IDs"
                        value={editResourceIDs}
                        onChange={setEditResourceIDs}
                        placeholder="comma-separated ids (leave empty for unconstrained)"
                        description="Any user holding this scope is limited to these ids."
                        inputClassName="font-mono text-sm"
                    />
                    <div className="flex justify-end gap-2 pt-2">
                        <button
                            type="button"
                            onClick={closeEdit}
                            className="px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                        >
                            Cancel
                        </button>
                        <Submit loading={savingEdit} label="Save" />
                    </div>
                </form>
            </Modal>
        </div>
    );
};

export default Scopes;
