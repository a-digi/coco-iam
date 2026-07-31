import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { OrganizationResource, type Organization, OrganizationSchema } from '../model/organization';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { OrganizationForm, type OrganizationFormData } from '../Form/OrganizationForm';
import { OrganizationPageHead } from '../Shared/OrganizationPageHead';
import { Workspaces } from './Partials/Workspaces';
import { UserRules } from './Partials/UserRules';
import { ProfileFields } from './Partials/ProfileFields';
import { OrgEmailSettings } from './Partials/Email/OrgEmailSettings';
import { OrgEmailAccounts } from './Partials/Email/OrgEmailAccounts';
import { OrgEmailTemplates } from './Partials/Email/OrgEmailTemplates';
import { SideMenu, type SideMenuItem } from '../../../Shared/Components/SideMenu';
import { AppScopes } from '../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

export const EditOrganization: React.FC = () => {
  useBreadcrumbItems([{ label: 'Organizations', href: '/organizations' }, { label: 'Manage' }]);
  const { id } = useParams<{ id: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);
  const [org, setOrg] = useState<Organization | null>(null);
  const [initialData, setInitialData] = useState<Partial<OrganizationFormData> | undefined>(undefined);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const fetchOrganization = React.useCallback(async () => {
    if (!id) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`organizations/{${OrganizationResource}}/{id:${id}}`);
      const raw = response?.message || response;

      if (raw) {
        const mapped = mapObjects(OrganizationSchema, [raw]) as unknown as Organization[];
        const o = mapped[0];
        setOrg(o);
        setInitialData({
          organization_id: o.organizationId || '',
          title: o.title || '',
          description: o.description || '',
          is_active: o.isActive ?? true,
        });
      } else {
        errorMessage('Organization not found');
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch organization data';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [id, get, errorMessage]);

  useEffect(() => {
    void fetchOrganization();
  }, [fetchOrganization]);

  const handleSubmit = async (data: OrganizationFormData) => {
    if (!id) return;
    setLoading(true);
    try {
      const payload: Record<string, unknown> = {
        organization_id: data.organization_id,
        title: data.title,
        description: data.description,
        is_active: data.is_active,
      };
      await patch(`organizations/{${OrganizationResource}}/{id:${id}}`, payload);
      successMessage(`Organization ${data.title} updated successfully!`);
      navigate('/organizations');
    } catch (err: unknown) {
      let errorMsg = 'Failed to update organization';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
      throw err;
    } finally {
      setLoading(false);
    }
  };

  if (fetching) {
    return (
      <div className="max-w-full">
        <div className="mt-6 flex items-center justify-center p-6">
          <svg className="animate-spin h-6 w-6 text-indigo-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"></path>
          </svg>
          <span className="ml-3 text-gray-500">Loading organization data...</span>
        </div>
      </div>
    );
  }

  // Details + Workspaces land at the top as the two primary
  // surfaces. Everything user-related — per-org rules, profile
  // schema, the two deep-dive pages — lives under a single
  // "Users" group so the admin doesn't scan a flat list where
  // policies and per-user management sit side by side.
  const usersChildren: SideMenuItem[] = [
    {
      id: 'user-rules',
      label: 'User rules',
      content: id ? <UserRules organizationId={id} /> : null,
    },
    {
      id: 'profile-fields',
      label: 'Profile fields',
      content: id ? <ProfileFields organizationId={id} /> : null,
      scopes: [AppScopes.OrganizationsProfileFieldsRead, AppScopes.OrganizationsProfileFields, AppScopes.Organizations, AppScopes.SuperAdmin],
    },
  ];

  if (id) {
    usersChildren.push({
      id: 'users',
      label: 'Manage Users',
      href: `/organizations/${id}/users`,
      scopes: [AppScopes.OrganizationsUsersRead, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin],
    });
    usersChildren.push({
      id: 'groups',
      label: 'Manage Groups',
      href: `/organizations/${id}/groups`,
      scopes: [AppScopes.OrganizationsGroupsRead, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin],
    });
  }

  const sideMenuItems: SideMenuItem[] = [
    {
      id: 'details',
      label: 'Details',
      content: (
        <OrganizationForm
          initialData={initialData}
          onSubmit={handleSubmit}
          loading={loading}
          submitLabel="Update Organization"
        />
      ),
    },
    {
      id: 'workspaces',
      label: 'Workspaces',
      content: id ? <Workspaces organizationId={id} /> : null,
    },
    {
      id: 'group-email',
      label: 'Email',
      // Falls back to the global mail engine wherever this org hasn't
      // customized a given concern — see plan/org-app-email-settings/plan.md.
      scopes: [AppScopes.OrganizationsRead, AppScopes.Organizations, AppScopes.SuperAdmin],
      children: [
        {
          id: 'email-settings',
          label: 'Settings',
          content: id ? <OrgEmailSettings organizationId={id} /> : null,
        },
        {
          id: 'email-accounts',
          label: 'Accounts',
          content: id ? <OrgEmailAccounts organizationId={id} /> : null,
        },
        {
          id: 'email-templates',
          label: 'Templates',
          content: id ? <OrgEmailTemplates organizationId={id} /> : null,
        },
      ],
    },
    {
      id: 'group-users',
      label: 'Users',
      // Pure group header — click toggles children. Default
      // expanded, matching the behaviour everywhere else in
      // the app.
      children: usersChildren,
    },
  ];

  return (
    <div className="max-w-full">
      <OrganizationPageHead
        organizationId={id ?? ''}
        organization={org}
      />

      <SideMenu items={sideMenuItems} initialActiveId="details" width="md" />
    </div>
  );
};

export default EditOrganization;
