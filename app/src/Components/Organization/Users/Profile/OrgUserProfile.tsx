import React, { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useHttpClient } from '../../../../api/http/useHttpClient';
import { useSnackBar } from '../../../../Shared/Components/SnackBar/SnackBarContext';
import Title from '../../../../Shared/Components/Font/Title';
import { Submit } from '../../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../../Shared/Components/Form';
import { OrganizationHeader } from '../../Shared/OrganizationHeader';
import { SideMenu, type SideMenuItem } from '../../../../Shared/Components/SideMenu';
import { AppScopes } from '../../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../../Layout/Breadcrumb/useBreadcrumb';
import type { ProfileField } from '../../Edit/Partials/ProfileFields';

interface ProfileResponse {
  user_id: string;
  fields: ProfileField[];
  profile_data: Record<string, unknown>;
  updated_at?: string;
}

interface ValidationError {
  field: string;
  message: string;
}

interface UpsertResponse {
  status: string;
  errors?: ValidationError[];
}

const asString = (v: unknown): string => {
  if (v === undefined || v === null) return '';
  return String(v);
};

export const OrgUserProfile: React.FC = () => {
  const { orgId, userId } = useParams<{ orgId: string; userId: string }>();
  useBreadcrumbItems([
    { label: 'Organizations', href: '/organizations' },
    { label: 'Users' },
    { label: 'Profile' },
  ]);

  const { get, put } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();

  const [fields, setFields] = useState<ProfileField[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    if (!orgId || !userId) return;
    setLoading(true);
    try {
      const response = await get<{ message: ProfileResponse }>(
        `organizations/{res:organizations}/{id:${orgId}}/users/{userId:${userId}}/profile`
      );
      const payload = response?.message;
      if (payload) {
        setFields(payload.fields ?? []);
        const nextValues: Record<string, string> = {};
        (payload.fields ?? []).forEach(f => {
          nextValues[f.name] = asString(payload.profile_data?.[f.name]);
        });
        setValues(nextValues);
      }
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to load profile');
    } finally {
      setLoading(false);
    }
  }, [orgId, userId, get, errorMessage]);

  useEffect(() => {
    void load();
  }, [load]);

  const save = async () => {
    if (!orgId || !userId) return;
    setSaving(true);
    setFieldErrors({});
    try {
      const profileData: Record<string, unknown> = {};
      for (const f of fields) {
        const raw = values[f.name] ?? '';
        if (raw === '') continue;
        profileData[f.name] = f.data_type === 'number' ? Number(raw) : raw;
      }
      const response = await put<UpsertResponse>(
        `organizations/{res:organizations}/{id:${orgId}}/users/{userId:${userId}}/profile`,
        { profile_data: profileData }
      );
      if (response?.status === 'validation_failed' && response.errors) {
        const errMap: Record<string, string> = {};
        response.errors.forEach(e => { errMap[e.field] = e.message; });
        setFieldErrors(errMap);
        errorMessage('Please correct the highlighted fields');
      } else {
        successMessage('Profile saved');
      }
    } catch (err: unknown) {
      errorMessage(err instanceof Error ? err.message : 'Failed to save profile');
    } finally {
      setSaving(false);
    }
  };

  if (!orgId || !userId) return <div>Missing organization or user id.</div>;

  if (loading) {
    return (
      <div className="max-w-full">
        <OrganizationHeader organizationId={orgId} />
        <Title>Manage Organization User</Title>
        <div className="mt-6 text-sm text-gray-500">Loading...</div>
      </div>
    );
  }

  const profileContent = fields.length === 0 ? (
    <p className="text-sm text-gray-500 mt-4">
      This organization has no profile fields defined. There&apos;s nothing to fill in.
    </p>
  ) : (
    <div className="space-y-4">
      <p className="text-sm text-gray-500 mb-6">
        Fill in the fields defined by your organization.
      </p>
      {fields.map(f => {
        const err = fieldErrors[f.name];
        const setValue = (v: string) => setValues({ ...values, [f.name]: v });

        if (f.data_type === 'long_text') {
          return (
            <div key={f.id}>
              <FormTextarea
                label={f.label + (f.is_required ? ' *' : '')}
                value={values[f.name] ?? ''}
                onChange={setValue}
                placeholder={f.description}
              />
              {err && <span className="text-xs text-red-500">{err}</span>}
            </div>
          );
        }
        if (f.data_type === 'select') {
          return (
            <div key={f.id}>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                {f.label}{f.is_required ? ' *' : ''}
              </label>
              <select
                value={values[f.name] ?? ''}
                onChange={e => setValue(e.target.value)}
                className="w-full px-3 py-2 rounded-lg border border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 text-gray-900 dark:text-gray-100"
              >
                <option value="">— select —</option>
                {f.options.map(opt => (
                  <option key={opt} value={opt}>{opt}</option>
                ))}
              </select>
              {f.description && <p className="text-xs text-gray-500 mt-1">{f.description}</p>}
              {err && <span className="text-xs text-red-500">{err}</span>}
            </div>
          );
        }

        const inputType = f.data_type === 'number' ? 'number'
          : f.data_type === 'date' ? 'date'
          : f.data_type === 'email' ? 'email'
          : f.data_type === 'url' ? 'url'
          : 'text';

        return (
          <div key={f.id}>
            <FormInput
              label={f.label + (f.is_required ? ' *' : '')}
              type={inputType}
              value={values[f.name] ?? ''}
              onChange={setValue}
              placeholder={f.description}
            />
            {err && <span className="text-xs text-red-500">{err}</span>}
          </div>
        );
      })}
      <div className="mt-6">
        <Submit type="button" onClick={() => void save()} label="Save profile" disabled={saving} />
      </div>
    </div>
  );

  const sideMenuItems: SideMenuItem[] = [
    {
      id: 'details',
      label: 'Details',
      href: `/organizations/${orgId}/users/edit/${userId}`,
    },
    {
      id: 'scopes',
      label: 'Scopes',
      href: `/organizations/${orgId}/users/edit/${userId}/scopes`,
      scopes: [
        AppScopes.OrganizationsUsersAclRead,
        AppScopes.OrganizationsUsersAcl,
        AppScopes.OrganizationsUsers,
        AppScopes.Organizations,
        AppScopes.SuperAdmin,
      ],
    },
    {
      id: 'profile',
      label: 'Profile',
      content: profileContent,
    },
  ];

  return (
    <div className="max-w-full">
      <OrganizationHeader organizationId={orgId} />
      <Title>Manage Organization User</Title>
      <div className="mt-6">
        <SideMenu items={sideMenuItems} initialActiveId="profile" width="md" />
      </div>
    </div>
  );
};

export default OrgUserProfile;
