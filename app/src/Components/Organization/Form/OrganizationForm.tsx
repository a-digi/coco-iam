import React, { useState } from 'react';

import { Switch } from '../../../Shared/Components/Switch';
import { Submit } from '../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../Shared/Components/Form';
import ScopeBasedComponentAccess from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';

export interface OrganizationFormData {
  organization_id: string;
  title: string;
  description: string;
  is_active: boolean;
}

interface OrganizationFormProps {
  initialData?: Partial<OrganizationFormData>;
  onSubmit: (data: OrganizationFormData) => Promise<void>;
  loading: boolean;
  submitLabel?: string;
}

// URL-friendly slug: lowercase alphanumerics + hyphens, ≤ 60 chars.
const slugify = (s: string): string =>
  s.toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60);

export const OrganizationForm: React.FC<OrganizationFormProps> = ({
  initialData,
  onSubmit,
  loading,
  submitLabel = 'Save',
}) => {
  const [title, setTitle] = useState(initialData?.title || '');
  const [organizationIdInput, setOrganizationIdInput] = useState(initialData?.organization_id || '');
  const [organizationIdTouched, setOrganizationIdTouched] = useState(!!initialData?.organization_id);
  const [description, setDescription] = useState(initialData?.description || '');
  const [isActive, setIsActive] = useState(initialData?.is_active ?? true);

  // Auto-derive organization_id from title until the admin edits it.
  const effectiveOrganizationId = organizationIdTouched
    ? organizationIdInput
    : slugify(title);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    await onSubmit({
      organization_id: effectiveOrganizationId,
      title,
      description,
      is_active: isActive,
    });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6 mt-6">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <FormInput
          id="title"
          label="Title"
          value={title}
          onChange={setTitle}
          required
          autoComplete="off"
        />
        <FormInput
          id="organization_id"
          label="Organization ID"
          value={effectiveOrganizationId}
          onChange={v => { setOrganizationIdInput(v); setOrganizationIdTouched(true); }}
          required
          placeholder="auto-generated from title"
          description="Globally unique, lowercase, hyphens, no spaces."
          inputClassName="font-mono text-sm"
        />
        <FormTextarea
          id="description"
          label="Description"
          value={description}
          onChange={setDescription}
          className="md:col-span-2"
        />
      </div>

      <div className="flex items-center space-x-6 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
        <Switch
          id="is_active"
          checked={isActive}
          onChange={setIsActive}
          label="Is Active"
        />
      </div>

      <div className="flex justify-start gap-4 items-center pt-4">
        <ScopeBasedComponentAccess
          requiredScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}
        >
          <Submit
            loading={loading}
            label={submitLabel}
          />
        </ScopeBasedComponentAccess>
      </div>
    </form>
  );
};
