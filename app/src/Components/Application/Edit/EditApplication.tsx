import React, { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import Title from '../../../Shared/Components/Font/Title';
import { useHttpClient } from '../../../api/http/useHttpClient';
import { useSnackBar } from '../../../Shared/Components/SnackBar/SnackBarContext';
import { mapObjects } from '../../../config/data/mapper/mapper';
import { ApplicationResource, type Application, ApplicationSchema } from '../model/application';
import { ApplicationHeader } from '../Shared/ApplicationHeader';
import { Scopes } from './Partials/Scopes';
import { Users } from './Partials/Users';
import { Template } from './Partials/Template';
import { Security } from './Partials/Security';
import { ApiCredentials } from './Partials/ApiCredentials';
import { LoginLogSection } from './Partials/LoginLog/LoginLogSection';
import { RegistrationFields } from './Partials/RegistrationFields';
import { Authentication } from './Partials/Authentication';
import { OAuthClients } from './Partials/OAuthClients';
import { Media } from './Partials/Media';
import { ApplicationLogo } from './Partials/ApplicationLogo';
import { DetailPanel } from './Partials/Detail/DetailPanel';
import { SideMenu, type SideMenuItem } from '../../../Shared/Components/SideMenu';
import { Switch } from '../../../Shared/Components/Switch';
import { Submit, Cancel } from '../../../Shared/Components/Button';
import { FormInput, FormTextarea } from '../../../Shared/Components/Form';
import { AppScopes } from '../../../config/security/scopes';
import { useBreadcrumbItems } from '../../../Layout/Breadcrumb/useBreadcrumb';

export const EditApplication: React.FC = () => {
  useBreadcrumbItems([{ label: 'Workspaces', href: '/workspaces' }, { label: 'Applications' }, { label: 'Manage' }]);
  const { workspaceId, appId } = useParams<{ workspaceId: string; appId: string }>();
  const [fetching, setFetching] = useState(true);
  const [loading, setLoading] = useState(false);

  const [title, setTitle] = useState('');
  const [clientId, setClientId] = useState('');
  const [description, setDescription] = useState('');
  const [isActive, setIsActive] = useState(true);
  const [allowRecovery, setAllowRecovery] = useState(true);
  const [allowRegistration, setAllowRegistration] = useState(false);

  const { get, patch } = useHttpClient();
  const { successMessage, errorMessage } = useSnackBar();
  const navigate = useNavigate();

  const backTo = `/workspaces/${workspaceId}/applications`;

  const fetchApp = React.useCallback(async () => {
    if (!appId) return;
    setFetching(true);
    try {
      const response = await get<{ message: unknown }>(`applications/{${ApplicationResource}}/{id:${appId}}`);
      const raw = response?.message || response;
      if (raw) {
        const mapped = mapObjects(ApplicationSchema, [raw]) as unknown as Application[];
        const a = mapped[0];
        setTitle(a.title || '');
        setClientId(a.clientId || '');
        setDescription(a.description || '');
        setIsActive(a.isActive ?? true);
        setAllowRecovery(a.allowRecovery ?? true);
        setAllowRegistration(a.allowRegistration ?? false);
      } else {
        errorMessage('Application not found');
      }
    } catch (err: unknown) {
      let errorMsg = 'Failed to fetch application data';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
    } finally {
      setFetching(false);
    }
  }, [appId, get, errorMessage]);

  useEffect(() => {
    void fetchApp();
  }, [fetchApp]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!appId) return;
    setLoading(true);
    try {
      await patch(`applications/{${ApplicationResource}}/{id:${appId}}`, {
        title,
        client_id: clientId,
        description,
        is_active: isActive,
        allow_recovery: allowRecovery,
        allow_registration: allowRegistration,
      });
      successMessage(`Application ${title} updated successfully!`);
      navigate(backTo);
    } catch (err: unknown) {
      let errorMsg = 'Failed to update application';
      if (err instanceof Error) errorMsg = err.message || errorMsg;
      errorMessage(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  if (!workspaceId || !appId) return <div>Missing route parameters.</div>;

  if (fetching) {
    return (
      <div className="max-w-full">
        <ApplicationHeader workspaceId={workspaceId} applicationId={appId} />
        <Title>Manage Application</Title>
        <div className="mt-6 text-sm text-gray-500">Loading...</div>
      </div>
    );
  }

  const editForm = (
    <form onSubmit={handleSubmit} className="space-y-6 mt-2">
      <ApplicationLogo applicationId={appId} />

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <FormInput
          id="title"
          label="Title"
          value={title}
          onChange={setTitle}
          required
        />
        <FormInput
          id="clientId"
          label="Client ID"
          value={clientId}
          onChange={setClientId}
          required
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

      <div className="flex flex-col gap-3 mt-4 p-4 bg-gray-50 dark:bg-surface-900 rounded-lg border border-gray-100 dark:border-gray-600">
        <Switch id="is_active" checked={isActive} onChange={setIsActive} label="Is Active" />
        <Switch
          id="allow_recovery"
          checked={allowRecovery}
          onChange={setAllowRecovery}
          label="Allow password recovery"
        />
        <Switch
          id="allow_registration"
          checked={allowRegistration}
          onChange={setAllowRegistration}
          label="Allow registration"
        />
      </div>

      <div className="flex justify-start gap-4 items-center pt-4">
        <Submit loading={loading} label="Update Application" />
        <Cancel to={backTo} />
      </div>
    </form>
  );

  // Any of the analytics scopes lets the user see the Detail sidebar
  // entry. Without one, the entry is hidden and SideMenu falls back
  // to the first visible panel (Edit).
  const analyticsScopes = [
    AppScopes.ApplicationsAnalyticsRead,
    AppScopes.ApplicationsAnalyticsUsersRead,
    AppScopes.ApplicationsAnalyticsGroupsRead,
    AppScopes.ApplicationsAnalyticsScopesRead,
    AppScopes.ApplicationsAnalyticsRecentGrantsRead,
    AppScopes.ApplicationsAnalyticsPendingRecoveriesRead,
    AppScopes.SuperAdmin,
  ];

  // Menu is grouped by concern so admins don't scan a flat list
  // of ten unrelated items. Three groups — Authentication covers
  // identity verification (end-user + machine), Access covers
  // authorisation (permissions + grants + signup), Branding
  // covers everything end users see. Detail + Edit stay at the
  // top as the landing + general-settings pages.
  const sideMenuItems: SideMenuItem[] = [
    {
      id: 'detail',
      label: 'Detail',
      content: <DetailPanel applicationId={appId} />,
      scopes: analyticsScopes,
    },
    { id: 'edit', label: 'Edit', content: editForm },
    {
      id: 'group-authentication',
      label: 'Authentication',
      // Pure group header — click toggles children. Children
      // default to expanded (visible).
      children: [
        {
          id: 'authentication',
          label: 'Authentication',
          content: <Authentication applicationId={appId} appSlug={clientId} />,
          scopes: [
            AppScopes.ApplicationsOauthRead,
            AppScopes.ApplicationsOauth,
            AppScopes.Applications,
            AppScopes.SuperAdmin,
          ],
        },
        {
          id: 'oauth-clients',
          label: 'OAuth Clients',
          content: <OAuthClients applicationId={appId} />,
          scopes: [
            AppScopes.ApplicationsOauthRead,
            AppScopes.ApplicationsOauth,
            AppScopes.Applications,
            AppScopes.SuperAdmin,
          ],
        },
        { id: 'security', label: 'Security', content: <Security applicationId={appId} /> },
        {
          id: 'api-credentials',
          label: 'API Credentials',
          content: <ApiCredentials applicationId={appId} />,
          scopes: [
            AppScopes.ApplicationsApiCredentialsRead,
            AppScopes.ApplicationsApiCredentials,
            AppScopes.Applications,
            AppScopes.SuperAdmin,
          ],
        },
        {
          id: 'login-log',
          label: 'Login log',
          content: <LoginLogSection applicationId={appId} />,
          scopes: [
            AppScopes.ApplicationsLoginLogRead,
            AppScopes.Applications,
            AppScopes.SuperAdmin,
          ],
        },
      ],
    },
    {
      id: 'group-access',
      label: 'Access',
      children: [
        { id: 'scopes', label: 'Scopes', content: <Scopes applicationId={appId} /> },
        { id: 'users', label: 'Users', content: <Users applicationId={appId} workspaceId={workspaceId} /> },
        {
          id: 'registration',
          label: 'Registration',
          content: <RegistrationFields applicationId={appId} workspaceId={workspaceId ?? ''} />,
          scopes: [
            AppScopes.ApplicationsRegistrationFieldsRead,
            AppScopes.ApplicationsRegistrationFields,
            AppScopes.Applications,
            AppScopes.SuperAdmin,
          ],
        },
      ],
    },
    {
      id: 'group-branding',
      label: 'Branding',
      children: [
        { id: 'template', label: 'Template', content: <Template applicationId={appId} workspaceId={workspaceId} /> },
        { id: 'media', label: 'Media', content: <Media applicationId={appId} /> },
      ],
    },
  ];

  return (
    <div className="max-w-full">
      <ApplicationHeader workspaceId={workspaceId} applicationId={appId} />
      <Title>Manage Application</Title>
      <div className="mt-6">
        <SideMenu items={sideMenuItems} initialActiveId="detail" width="md" />
      </div>
    </div>
  );
};

export default EditApplication;
