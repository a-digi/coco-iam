import React, { useCallback, useEffect, useState } from 'react';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import { Switch } from '../../../../Shared/Components/Switch';
import ScopeBasedComponentAccess from '../../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../../config/security/scopes';
import { EditAction, DeleteAction } from '../../../../Shared/Components/Actions';
import { ConfirmModal } from '../../../../Shared/Components/Modal';

export type ProfileFieldType =
  | 'text'
  | 'long_text'
  | 'number'
  | 'date'
  | 'email'
  | 'url'
  | 'select'
  | 'choice'
  | 'multiple'
  | 'file';

export interface ProfileField {
  id: string;
  name: string;
  label: string;
  description: string;
  data_type: ProfileFieldType;
  is_required: boolean;
  min_value: number | null;
  max_value: number | null;
  options: string[];
  regex: string;
  accept_mime: string;
  max_bytes: number;
  order_index: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

const EMPTY_FORM: Omit<ProfileField, 'id' | 'order_index' | 'is_active' | 'created_at' | 'updated_at'> = {
  name: '',
  label: '',
  description: '',
  data_type: 'text',
  is_required: false,
  min_value: null,
  max_value: null,
  options: [],
  regex: '',
  accept_mime: '',
  max_bytes: 0,
};

const TYPE_LABELS: Record<ProfileFieldType, string> = {
  text:      'Short text',
  long_text: 'Long text',
  number:    'Number',
  date:      'Date',
  email:     'Email',
  url:       'URL',
  select:    'Single-select',
  choice:    'Choice (single)',
  multiple:  'Choice (multiple)',
  file:      'File upload',
};

interface Props {
  organizationId: string;
}

export const ProfileFields: React.FC<Props> = ({ organizationId }) => {
  const { get, post, patch, del } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const [fields, setFields] = useState<ProfileField[]>([]);
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState<typeof EMPTY_FORM>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);

  const [confirmDelete, setConfirmDelete] = useState<ProfileField | null>(null);
  const [deleting, setDeleting] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await get<{ message: ProfileField[] }>(`organizations/{res:organizations}/{id:${organizationId}}/profile-fields`);
      setFields(Array.isArray(response?.message) ? response.message : []);
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to load profile fields');
    } finally {
      setLoading(false);
    }
  }, [get, organizationId, errorMessage]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const startCreate = () => {
    setEditingId(null);
    setForm(EMPTY_FORM);
    setFormOpen(true);
  };

  const startEdit = (f: ProfileField) => {
    setEditingId(f.id);
    setForm({
      name: f.name,
      label: f.label,
      description: f.description,
      data_type: f.data_type,
      is_required: f.is_required,
      min_value: f.min_value,
      max_value: f.max_value,
      options: f.options,
      regex: f.regex,
      accept_mime: f.accept_mime ?? '',
      max_bytes: f.max_bytes ?? 0,
    });
    setFormOpen(true);
  };

  const cancelForm = () => {
    setFormOpen(false);
    setEditingId(null);
    setForm(EMPTY_FORM);
  };

  const saveForm = async () => {
    const needsOptions = form.data_type === 'select'
      || form.data_type === 'choice'
      || form.data_type === 'multiple';
    const payload = {
      ...form,
      options: needsOptions ? form.options : [],
    };
    if (!payload.name.trim() || !payload.label.trim()) {
      errorMessage('Name and label are required');
      return;
    }
    if (needsOptions && payload.options.length === 0) {
      errorMessage('This field type requires at least one option');
      return;
    }
    setSaving(true);
    try {
      if (editingId) {
        await patch(`organizations/{res:organizations}/{id:${organizationId}}/profile-fields/{fieldId:${editingId}}`, payload);
        successMessage('Field updated');
      } else {
        await post(`organizations/{res:organizations}/{id:${organizationId}}/profile-fields`, payload);
        successMessage('Field created');
      }
      cancelForm();
      await refresh();
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to save field');
    } finally {
      setSaving(false);
    }
  };

  const runReorder = async (newOrder: ProfileField[]) => {
    setFields(newOrder);
    try {
      await post(`organizations/{res:organizations}/{id:${organizationId}}/profile-fields/reorder`, {
        ordered_ids: newOrder.map(f => f.id),
      });
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to reorder');
      await refresh();
    }
  };

  const moveUp = (idx: number) => {
    if (idx === 0) return;
    const next = [...fields];
    [next[idx - 1], next[idx]] = [next[idx], next[idx - 1]];
    void runReorder(next);
  };

  const moveDown = (idx: number) => {
    if (idx === fields.length - 1) return;
    const next = [...fields];
    [next[idx], next[idx + 1]] = [next[idx + 1], next[idx]];
    void runReorder(next);
  };

  const doDelete = async () => {
    if (!confirmDelete) return;
    setDeleting(true);
    try {
      await del(`organizations/{res:organizations}/{id:${organizationId}}/profile-fields/{fieldId:${confirmDelete.id}}`);
      successMessage(`Field "${confirmDelete.label}" deleted`);
      setConfirmDelete(null);
      await refresh();
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to delete field');
    } finally {
      setDeleting(false);
    }
  };

  const activeFields = fields.filter(f => f.is_active);
  const archivedFields = fields.filter(f => !f.is_active);

  return (
    <div className="mt-4">
      <div className="mb-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Profile fields</h3>
        <p className="text-sm text-gray-500">
          Custom fields that every org user is expected to fill. Each field has a type and optional min/max or options.
          Deleting a field is a soft-delete — existing user values are kept but the field disappears from forms.
        </p>
      </div>

      <ScopeBasedComponentAccess
        requiredScopes={[AppScopes.OrganizationsProfileFieldsWrite, AppScopes.OrganizationsProfileFields, AppScopes.Organizations, AppScopes.SuperAdmin]}
      >
        <div className="mb-4">
          <Submit type="button" onClick={startCreate} label="Add field" />
        </div>
      </ScopeBasedComponentAccess>

      {formOpen && (
        <div className="mb-6 border border-gray-200 dark:border-surface-700 rounded-xl p-4 bg-white dark:bg-surface-800">
          <h4 className="font-semibold text-gray-900 dark:text-gray-100 mb-3">
            {editingId ? 'Edit field' : 'New field'}
          </h4>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <FormInput
              label="Name (machine key)"
              value={form.name}
              onChange={v => setForm({ ...form, name: v })}
              disabled={!!editingId}
              placeholder="e.g. date_of_birth"
            />
            <FormInput
              label="Label"
              value={form.label}
              onChange={v => setForm({ ...form, label: v })}
              placeholder="e.g. Date of birth"
            />
          </div>

          <div className="mt-3">
            <FormTextarea
              label="Description"
              value={form.description}
              onChange={v => setForm({ ...form, description: v })}
              placeholder="Optional help text shown to the user"
            />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Type</label>
              <select
                value={form.data_type}
                onChange={e => setForm({ ...form, data_type: e.target.value as ProfileFieldType })}
                className="w-full px-3 py-2 rounded-lg border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
              >
                {Object.entries(TYPE_LABELS).map(([v, label]) => (
                  <option key={v} value={v}>{label}</option>
                ))}
              </select>
            </div>
            <div className="flex items-end pb-1">
              <Switch checked={form.is_required} onChange={v => setForm({ ...form, is_required: v })} label="Required" />
            </div>
          </div>

          {(form.data_type === 'text' || form.data_type === 'long_text' || form.data_type === 'number') && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <FormInput
                label={form.data_type === 'number' ? 'Min value' : 'Min length'}
                type="number"
                value={form.min_value ?? ''}
                onChange={v => setForm({ ...form, min_value: v === '' ? null : Number(v) })}
              />
              <FormInput
                label={form.data_type === 'number' ? 'Max value' : 'Max length'}
                type="number"
                value={form.max_value ?? ''}
                onChange={v => setForm({ ...form, max_value: v === '' ? null : Number(v) })}
              />
            </div>
          )}

          {(form.data_type === 'select' || form.data_type === 'choice' || form.data_type === 'multiple') && (
            <div className="mt-3">
              <FormTextarea
                label="Options (one per line)"
                value={form.options.join('\n')}
                onChange={v => setForm({ ...form, options: v.split('\n').map(s => s.trim()).filter(Boolean) })}
              />
            </div>
          )}

          {(form.data_type === 'text' || form.data_type === 'long_text') && (
            <div className="mt-3">
              <FormInput
                label="Regex (optional)"
                value={form.regex}
                onChange={v => setForm({ ...form, regex: v })}
                placeholder="e.g. ^[A-Z]{2}-\\d{4}$"
              />
            </div>
          )}

          {form.data_type === 'file' && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3">
              <FormInput
                label="Accepted MIME types (comma-separated, optional)"
                value={form.accept_mime}
                onChange={v => setForm({ ...form, accept_mime: v })}
                placeholder="image/png,image/jpeg,application/pdf"
              />
              <FormInput
                label="Max size in bytes (0 = default)"
                type="number"
                value={form.max_bytes === 0 ? '' : form.max_bytes}
                onChange={v => setForm({ ...form, max_bytes: v === '' ? 0 : Number(v) })}
                placeholder="0"
              />
            </div>
          )}

          <div className="flex gap-2 mt-4">
            <Submit type="button" onClick={() => void saveForm()} label={editingId ? 'Update' : 'Create'} disabled={saving} />
            <button
              type="button"
              onClick={cancelForm}
              className="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-surface-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-surface-700"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {loading ? (
        <div className="text-sm text-gray-500">Loading fields…</div>
      ) : (
        <>
          {activeFields.length === 0 ? (
            <div className="text-sm text-gray-500 py-3">No fields defined yet.</div>
          ) : (
            <div className="border border-gray-200 dark:border-surface-700 rounded-xl overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-gray-50 dark:bg-surface-800">
                  <tr>
                    <th className="text-left py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Order</th>
                    <th className="text-left py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Name</th>
                    <th className="text-left py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Label</th>
                    <th className="text-left py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Type</th>
                    <th className="text-left py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Required</th>
                    <th className="text-right py-2 px-3 text-[0.6875rem] font-semibold uppercase tracking-wide text-gray-500">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {activeFields.map((f, idx) => (
                    <tr key={f.id} className="border-t border-gray-100 dark:border-surface-700">
                      <td className="py-2 px-3 whitespace-nowrap">
                        <ScopeBasedComponentAccess
                          requiredScopes={[AppScopes.OrganizationsProfileFieldsWrite, AppScopes.OrganizationsProfileFields, AppScopes.Organizations, AppScopes.SuperAdmin]}
                        >
                          <span className="inline-flex items-center gap-1">
                            <button type="button" onClick={() => moveUp(idx)} disabled={idx === 0} className="px-1 disabled:opacity-30">▲</button>
                            <button type="button" onClick={() => moveDown(idx)} disabled={idx === activeFields.length - 1} className="px-1 disabled:opacity-30">▼</button>
                          </span>
                        </ScopeBasedComponentAccess>
                      </td>
                      <td className="py-2 px-3 font-mono text-[0.8125rem]">{f.name}</td>
                      <td className="py-2 px-3">{f.label}</td>
                      <td className="py-2 px-3">{TYPE_LABELS[f.data_type]}</td>
                      <td className="py-2 px-3">{f.is_required ? 'Yes' : 'No'}</td>
                      <td className="py-2 px-3 text-right">
                        <div className="inline-flex gap-1">
                          <ScopeBasedComponentAccess
                            requiredScopes={[AppScopes.OrganizationsProfileFieldsWrite, AppScopes.OrganizationsProfileFields, AppScopes.Organizations, AppScopes.SuperAdmin]}
                          >
                            <EditAction onClick={() => startEdit(f)} />
                          </ScopeBasedComponentAccess>
                          <ScopeBasedComponentAccess
                            requiredScopes={[AppScopes.OrganizationsProfileFieldsDelete, AppScopes.OrganizationsProfileFields, AppScopes.Organizations, AppScopes.SuperAdmin]}
                          >
                            <DeleteAction onClick={() => setConfirmDelete(f)} />
                          </ScopeBasedComponentAccess>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {archivedFields.length > 0 && (
            <div className="mt-6">
              <h4 className="text-sm font-semibold text-gray-500 uppercase tracking-wide mb-2">Archived</h4>
              <ul className="text-sm text-gray-500 space-y-1">
                {archivedFields.map(f => (
                  <li key={f.id} className="font-mono">{f.name} — {f.label}</li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}

      <ConfirmModal
        isOpen={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={doDelete}
        title="Delete Profile Field"
        message={confirmDelete ? `Soft-delete "${confirmDelete.label}"? Existing user values are preserved but the field won't appear on forms.` : ''}
        confirmLabel="Delete"
        isLoading={deleting}
        variant="danger"
      />
    </div>
  );
};

export default ProfileFields;
