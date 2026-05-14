import React, { useMemo, useState } from 'react';
import { Modal } from '../../../../Shared/Components/Modal';
import { Submit } from '../../../../Shared/Components/Button';

/**
 * AclEditorModal edits one ACL row's three authorisation dimensions:
 *
 *   roles           — scopes the caller holds
 *   grantable_roles — subset of `roles` the caller can bestow on others
 *   resource_ids    — per-scope id allow-lists; missing key = unconstrained
 *
 * Invariants enforced client-side (the server re-checks):
 * - `grantable ⊆ roles`. Unchecking a role auto-drops it from
 *   grantable. Grantable rows for unheld roles are disabled.
 * - `resource_ids` keys are restricted to scopes currently in `roles`.
 *   Rows whose scope is no longer held get pruned on save.
 */
export interface AclScope {
    id: string;
    scopeId: string;
    description?: string;
}

export interface AclEditorValue {
    roles: string[];
    grantable_roles: string[];
    resource_ids: Record<string, string[]>;
}

interface Props {
    isOpen: boolean;
    onClose: () => void;
    onSave: (value: AclEditorValue) => Promise<void> | void;
    scopes: AclScope[];
    initial: AclEditorValue;
    title?: string;
    saving?: boolean;
}

type ResourceRow = { scope: string; idsText: string };

const toRows = (src: Record<string, string[]>): ResourceRow[] =>
    Object.entries(src).map(([scope, ids]) => ({ scope, idsText: ids.join(', ') }));

const fromRows = (rows: ResourceRow[], allowedScopes: Set<string>): Record<string, string[]> => {
    const out: Record<string, string[]> = {};
    for (const row of rows) {
        const scope = row.scope.trim();
        if (!scope || !allowedScopes.has(scope)) continue;
        const ids = row.idsText
            .split(',')
            .map(s => s.trim())
            .filter(Boolean);
        out[scope] = ids;
    }
    return out;
};

// `initialKey` is a cheap, stable-for-equal-values hash of the initial
// ACL value. It becomes the `key` on the inner body, so opening the
// modal with a different row — or reopening at all — remounts the body
// and the useState initializers re-read from the fresh `initial`. This
// replaces the old "reset via useEffect" pattern, which tripped the
// react-hooks/set-state-in-effect rule.
const initialKey = (v: AclEditorValue): string =>
    `${v.roles.join('|')}::${v.grantable_roles.join('|')}::${Object.entries(v.resource_ids)
        .map(([k, ids]) => `${k}=${ids.join(',')}`)
        .join(';')}`;

export const AclEditorModal: React.FC<Props> = (props) => {
    const { isOpen, onClose, title = 'Edit ACL' } = props;
    return (
        <Modal isOpen={isOpen} onClose={onClose} title={title} maxWidth="3xl">
            {isOpen && <AclEditorBody key={initialKey(props.initial)} {...props} />}
        </Modal>
    );
};

type BodyProps = Omit<Props, 'isOpen' | 'title'>;

const AclEditorBody: React.FC<BodyProps> = ({
    onClose, onSave, scopes, initial, saving = false,
}) => {
    const [roles, setRoles] = useState<string[]>(initial.roles);
    const [grantable, setGrantable] = useState<string[]>(initial.grantable_roles);
    const [rows, setRows] = useState<ResourceRow[]>(toRows(initial.resource_ids));

    const rolesSet = useMemo(() => new Set(roles), [roles]);

    const toggleRole = (scopeId: string) => {
        setRoles(prev => {
            const has = prev.includes(scopeId);
            const next = has ? prev.filter(r => r !== scopeId) : [...prev, scopeId];
            // Auto-drop grantable when the parent role is removed.
            if (has) setGrantable(g => g.filter(r => r !== scopeId));
            return next;
        });
    };

    const toggleGrantable = (scopeId: string) => {
        if (!rolesSet.has(scopeId)) return;
        setGrantable(prev => prev.includes(scopeId)
            ? prev.filter(r => r !== scopeId)
            : [...prev, scopeId]);
    };

    const addRow = () => {
        // Pick the first currently-held scope that doesn't already
        // have a row; if none, default to empty and let the admin
        // type.
        const used = new Set(rows.map(r => r.scope));
        const nextScope = roles.find(r => !used.has(r)) ?? '';
        setRows(prev => [...prev, { scope: nextScope, idsText: '' }]);
    };

    const updateRow = (idx: number, patch: Partial<ResourceRow>) => {
        setRows(prev => prev.map((r, i) => i === idx ? { ...r, ...patch } : r));
    };

    const removeRow = (idx: number) => {
        setRows(prev => prev.filter((_, i) => i !== idx));
    };

    const handleSave = async (e: React.FormEvent) => {
        e.preventDefault();
        await onSave({
            roles,
            grantable_roles: grantable.filter(g => rolesSet.has(g)),
            resource_ids: fromRows(rows, rolesSet),
        });
    };

    return (
        <form onSubmit={handleSave} className="space-y-6">
                {/* -- Roles -- */}
                <section>
                    <h4 className="text-sm font-medium text-gray-800 dark:text-gray-200 mb-2">Roles</h4>
                    {scopes.length === 0 ? (
                        <p className="text-xs text-gray-500 italic">Define application scopes first.</p>
                    ) : (
                        <div className="flex flex-wrap gap-2">
                            {scopes.map(s => {
                                const checked = rolesSet.has(s.scopeId);
                                return (
                                    <label
                                        key={s.id}
                                        className={`inline-flex items-center gap-2 px-2 py-1 rounded border text-xs cursor-pointer ${checked
                                            ? 'border-indigo-400 bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300'
                                            : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300'
                                            }`}
                                    >
                                        <input
                                            type="checkbox"
                                            className="accent-indigo-600"
                                            checked={checked}
                                            onChange={() => toggleRole(s.scopeId)}
                                        />
                                        <span className="font-mono">{s.scopeId}</span>
                                    </label>
                                );
                            })}
                        </div>
                    )}
                </section>

                {/* -- Grantable -- */}
                <section>
                    <h4 className="text-sm font-medium text-gray-800 dark:text-gray-200 mb-1">Grantable roles</h4>
                    <p className="text-xs text-gray-500 mb-2">
                        Subset of the roles above. Must be held by the caller before it can be bestowed on
                        new or existing users.
                    </p>
                    {scopes.length === 0 ? (
                        <p className="text-xs text-gray-500 italic">No scopes available.</p>
                    ) : (
                        <div className="flex flex-wrap gap-2">
                            {scopes.map(s => {
                                const held = rolesSet.has(s.scopeId);
                                const checked = held && grantable.includes(s.scopeId);
                                return (
                                    <label
                                        key={s.id}
                                        className={`inline-flex items-center gap-2 px-2 py-1 rounded border text-xs ${held ? 'cursor-pointer' : 'cursor-not-allowed opacity-40'
                                            } ${checked
                                                ? 'border-amber-400 bg-amber-50 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300'
                                                : 'border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300'
                                            }`}
                                        title={held ? undefined : 'Enable the role above first.'}
                                    >
                                        <input
                                            type="checkbox"
                                            className="accent-amber-600"
                                            checked={checked}
                                            disabled={!held}
                                            onChange={() => toggleGrantable(s.scopeId)}
                                        />
                                        <span className="font-mono">{s.scopeId}</span>
                                    </label>
                                );
                            })}
                        </div>
                    )}
                </section>

                {/* -- Resource IDs -- */}
                <section>
                    <div className="flex items-center justify-between mb-1">
                        <h4 className="text-sm font-medium text-gray-800 dark:text-gray-200">Resource IDs</h4>
                        <button
                            type="button"
                            onClick={addRow}
                            disabled={roles.length === 0}
                            className="px-2 py-1 text-xs rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800 disabled:opacity-40"
                        >
                            + Add constraint
                        </button>
                    </div>
                    <p className="text-xs text-gray-500 mb-2">
                        Limits a scope to specific entity ids. Leave the list empty to deny-all. A scope
                        with no row here is unconstrained.
                    </p>
                    {rows.length === 0 ? (
                        <p className="text-xs text-gray-500 italic">No resource-id constraints.</p>
                    ) : (
                        <div className="space-y-2">
                            {rows.map((row, idx) => {
                                const missing = row.scope && !rolesSet.has(row.scope);
                                return (
                                    <div key={idx} className="flex items-start gap-2">
                                        <select
                                            value={row.scope}
                                            onChange={e => updateRow(idx, { scope: e.target.value })}
                                            className={`px-2 py-1.5 text-xs rounded-md border bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 font-mono ${missing ? 'border-red-400' : 'border-gray-300 dark:border-gray-600'}`}
                                        >
                                            <option value="">select scope</option>
                                            {roles.map(r => (
                                                <option key={r} value={r}>{r}</option>
                                            ))}
                                            {missing && <option value={row.scope}>{row.scope} (removed)</option>}
                                        </select>
                                        <input
                                            value={row.idsText}
                                            onChange={e => updateRow(idx, { idsText: e.target.value })}
                                            placeholder="comma-separated ids"
                                            className="flex-1 px-3 py-1.5 text-xs border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100 font-mono"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => removeRow(idx)}
                                            className="px-2 py-1 text-xs rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                                        >
                                            ✕
                                        </button>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </section>

                <div className="flex justify-end gap-2 pt-2">
                    <button
                        type="button"
                        onClick={onClose}
                        className="px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-surface-700 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800"
                    >
                        Cancel
                    </button>
                    <Submit loading={saving} label="Save" />
                </div>
        </form>
    );
};

export default AclEditorModal;
