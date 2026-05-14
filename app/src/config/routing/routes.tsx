import AuthGuard from '../../Components/Auth/Guard/AuthGuard';
import Dashboard from '../../Components/Dashboard/Dashboard';
import AdminLogin from '../../Components/Auth/Login/Admin/AdminLogin';
import Logout from '../../Components/Auth/Logout/Logout';
import ActivatePage from '../../Components/Auth/Activation/ActivatePage';
import AdminActivatePage from '../../Components/Auth/Activation/AdminActivatePage';
import ChangePasswordPage from '../../Components/Auth/PasswordChange/ChangePasswordPage';
import ForgotPasswordPage from '../../Components/Auth/PasswordRecovery/ForgotPasswordPage';
import ResetPasswordPage from '../../Components/Auth/PasswordRecovery/ResetPasswordPage';
import AppLoginPage from '../../Components/Auth/AppLogin/AppLoginPage';
import AppRecoveryPage from '../../Components/Auth/AppRecovery/AppRecoveryPage';
import type { RouteConfig } from './RouteConfig';
import AdminUsersDashboard from '../../Components/Admin/Users/Dashboard/Dashboard.tsx';
import CreateUser from '../../Components/Admin/Users/Create/CreateUser';
import EditUser from '../../Components/Admin/Users/Edit/EditUser';
import AdminGroupsDashboard from '../../Components/Admin/Groups/Dashboard/Dashboard.tsx';
import CreateGroup from '../../Components/Admin/Groups/Create/CreateGroup.tsx';
import EditGroup from '../../Components/Admin/Groups/Edit/EditGroup.tsx';
import OrganizationsDashboard from '../../Components/Organization/Dashboard/Dashboard.tsx';
import CreateOrganization from '../../Components/Organization/Create/CreateOrganization.tsx';
import EditOrganization from '../../Components/Organization/Edit/EditOrganization.tsx';
import OrganizationUsersDashboard from '../../Components/Organization/Users/Dashboard/Dashboard.tsx';
import CreateOrganizationUser from '../../Components/Organization/Users/Create/CreateOrganizationUser.tsx';
import EditOrganizationUser from '../../Components/Organization/Users/Edit/EditOrganizationUser.tsx';
import OrgUserProfile from '../../Components/Organization/Users/Profile/OrgUserProfile.tsx';
import OrganizationUserScopesPage from '../../Components/Organization/Users/Scopes/OrganizationUserScopesPage.tsx';
import OrganizationGroupsDashboard from '../../Components/Organization/Groups/Dashboard/Dashboard.tsx';
import CreateOrganizationGroup from '../../Components/Organization/Groups/Create/CreateOrganizationGroup.tsx';
import EditOrganizationGroup from '../../Components/Organization/Groups/Edit/EditOrganizationGroup.tsx';
import CreateWorkspace from '../../Components/Workspace/Create/CreateWorkspace.tsx';
import EditWorkspace from '../../Components/Workspace/Edit/EditWorkspace.tsx';
import ApplicationsDashboard from '../../Components/Application/Dashboard/Dashboard.tsx';
import CreateApplication from '../../Components/Application/Create/CreateApplication.tsx';
import EditApplication from '../../Components/Application/Edit/EditApplication.tsx';
import QueueDashboard from '../../Components/Queue/Dashboard/Dashboard.tsx';
import QueueDetail from '../../Components/Queue/Detail/QueueDetail.tsx';
import QueueTaskDetail from '../../Components/Queue/Task/TaskDetail.tsx';
import CreateQueue from '../../Components/Queue/Create/CreateQueue.tsx';
import { Navigate } from 'react-router-dom';
import SettingsPage from '../../Components/Admin/Settings/SettingsPage.tsx';
import EmailTemplatesManager from '../../Components/Admin/Settings/EmailTemplates/EmailTemplatesManager.tsx';
import EmailTemplateForm from '../../Components/Admin/Settings/EmailTemplates/EmailTemplateForm.tsx';
import EmailSettingsManager from '../../Components/Admin/Settings/Email/EmailSettingsManager.tsx';
import GeneralSettingsManager from '../../Components/Admin/Settings/General/GeneralSettingsManager.tsx';
import UserRulesManager from '../../Components/Admin/Settings/UserRules/UserRulesManager.tsx';
import EmailAccountsManager from '../../Components/Admin/Settings/EmailAccounts/EmailAccountsManager.tsx';
import EmailAccountForm from '../../Components/Admin/Settings/EmailAccounts/EmailAccountForm.tsx';
import { AppScopes } from '../security/scopes.ts';
import ProfilePage from '../../Components/Admin/Profile/ProfilePage';
import ObserveDashboard from '../../Components/Observe/ObserveDashboard';
import AgentMetricsPage from '../../Components/Observe/Agent/AgentMetricsPage';

export const routes: RouteConfig[] = [
  {
    path: '/',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminDashboardRead]}>
        <Dashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/dashboard',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminDashboardRead]}>
        <Dashboard />
      </AuthGuard>
    ),
  },
  {
    // Admin-self profile page. Scope `admin:me` covers any
    // authenticated admin; `SuperAdmin` is listed so the existing
    // guard pattern (superadmin overrides everything) continues
    // to apply.
    path: '/profile',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminMe, AppScopes.SuperAdmin]}>
        <ProfilePage />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/users',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminUsersRead, AppScopes.SuperAdmin]}>
        <AdminUsersDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/groups',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminGroupsRead, AppScopes.SuperAdmin]}>
        <AdminGroupsDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/groups/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminGroupsWrite, AppScopes.SuperAdmin]}>
        <CreateGroup />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/groups/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminGroupsRead, AppScopes.SuperAdmin]}>
        <EditGroup />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/users/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminUsersWrite, AppScopes.SuperAdmin]}>
        <CreateUser />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/users/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.UserMe, AppScopes.AdminUsersRead, AppScopes.SuperAdmin]}>
        <EditUser />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/queue',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminQueueRead, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
        <QueueDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/queue/tasks/:taskId',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminQueueRead, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
        <QueueTaskDetail />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/queue/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminQueueWrite, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
        <CreateQueue />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/queue/:queueName',
    element: (
      <AuthGuard accessScopes={[AppScopes.AdminQueueRead, AppScopes.AdminQueue, AppScopes.SuperAdmin]}>
        <QueueDetail />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsRead, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <OrganizationsDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsWrite, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <CreateOrganization />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsRead, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <EditOrganization />
      </AuthGuard>
    ),
  },
  {
    path: '/workspaces/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}>
        <CreateWorkspace />
      </AuthGuard>
    ),
  },
  {
    path: '/workspaces/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.WorkspacesRead, AppScopes.Workspaces, AppScopes.SuperAdmin]}>
        <EditWorkspace />
      </AuthGuard>
    ),
  },
  {
    path: '/workspaces/:workspaceId/applications',
    element: (
      <AuthGuard accessScopes={[AppScopes.ApplicationsRead, AppScopes.Applications, AppScopes.SuperAdmin]}>
        <ApplicationsDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/workspaces/:workspaceId/applications/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.ApplicationsWrite, AppScopes.Applications, AppScopes.SuperAdmin]}>
        <CreateApplication />
      </AuthGuard>
    ),
  },
  {
    path: '/workspaces/:workspaceId/applications/edit/:appId',
    element: (
      <AuthGuard accessScopes={[AppScopes.ApplicationsRead, AppScopes.Applications, AppScopes.SuperAdmin]}>
        <EditApplication />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/workspaces/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.WorkspacesWrite, AppScopes.Workspaces, AppScopes.SuperAdmin]}>
        <CreateWorkspace />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/users',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsUsersRead, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <OrganizationUsersDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/users/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsUsersWrite, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <CreateOrganizationUser />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/users/edit/:userId',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsUsersRead, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <EditOrganizationUser />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/users/edit/:userId/scopes',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsUsersAclRead, AppScopes.OrganizationsUsersAcl, AppScopes.OrganizationsUsers, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <OrganizationUserScopesPage />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/users/:userId/profile',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsUsersProfileRead, AppScopes.OrganizationsUsersProfile, AppScopes.Organizations, AppScopes.UserMe, AppScopes.SuperAdmin]}>
        <OrgUserProfile />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/groups',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsGroupsRead, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <OrganizationGroupsDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/groups/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsGroupsWrite, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <CreateOrganizationGroup />
      </AuthGuard>
    ),
  },
  {
    path: '/organizations/:orgId/groups/edit/:groupId',
    element: (
      <AuthGuard accessScopes={[AppScopes.OrganizationsGroupsRead, AppScopes.OrganizationsGroups, AppScopes.Organizations, AppScopes.SuperAdmin]}>
        <EditOrganizationGroup />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminSettingsGeneral, AppScopes.AdminSettingsGeneralRead, AppScopes.AdminMailTemplates, AppScopes.AdminMailTemplatesRead, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsRead]}>
        <Navigate to="/admin/settings/general" replace />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/general',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminSettingsGeneral, AppScopes.AdminSettingsGeneralRead]}>
        <SettingsPage>
          <GeneralSettingsManager />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/user-rules',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminSettingsUserRules, AppScopes.AdminSettingsUserRulesRead]}>
        <SettingsPage>
          <UserRulesManager />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-templates',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailTemplates, AppScopes.AdminMailTemplatesRead]}>
        <SettingsPage>
          <EmailTemplatesManager />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-templates/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailTemplates, AppScopes.AdminMailTemplatesWrite]}>
        <SettingsPage>
          <EmailTemplateForm mode="create" />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-templates/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailTemplates, AppScopes.AdminMailTemplatesRead]}>
        <SettingsPage>
          <EmailTemplateForm mode="edit" />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsRead]}>
        <SettingsPage>
          <EmailSettingsManager />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-accounts',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsRead]}>
        <SettingsPage>
          <EmailAccountsManager />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-accounts/create',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsWrite]}>
        <SettingsPage>
          <EmailAccountForm mode="create" />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/settings/email-accounts/edit/:id',
    element: (
      <AuthGuard accessScopes={[AppScopes.SuperAdmin, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsRead]}>
        <SettingsPage>
          <EmailAccountForm mode="edit" />
        </SettingsPage>
      </AuthGuard>
    ),
  },
  {
    path: '/admin/observe',
    element: (
      <AuthGuard accessScopes={[AppScopes.ObserveView, AppScopes.ObserveManage, AppScopes.SuperAdmin]}>
        <ObserveDashboard />
      </AuthGuard>
    ),
  },
  {
    path: '/admin/observe/agents/:agentId',
    element: (
      <AuthGuard accessScopes={[AppScopes.ObserveView, AppScopes.SuperAdmin]}>
        <AgentMetricsPage />
      </AuthGuard>
    ),
  },
  {
    path: '/login',
    element: <AdminLogin />,
  },
  {
    path: '/logout',
    element: <Logout />,
  },
  {
    path: '/activate',
    element: <ActivatePage />,
  },
  {
    path: '/activation/a',
    element: <AdminActivatePage />,
  },
  {
    path: '/forgot-password',
    element: <ForgotPasswordPage />,
  },
  {
    path: '/reset-password',
    element: <ResetPasswordPage />,
  },
  {
    path: '/login/a/:org/:ws/:app',
    element: <AppLoginPage />,
  },
  {
    path: '/recover/a/:org/:ws/:app',
    element: <AppRecoveryPage mode="request" />,
  },
  {
    path: '/recover/a/:org/:ws/:app/reset',
    element: <AppRecoveryPage mode="reset" />,
  },
  {
    path: '/account/change-password',
    element: (
      <AuthGuard>
        <ChangePasswordPage />
      </AuthGuard>
    ),
  },
];
