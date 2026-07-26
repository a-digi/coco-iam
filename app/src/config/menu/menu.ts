import type { MenuItem } from '../../Components/Menu/type/type.ts';
import { AppScopes } from '../security/scopes.ts';

export const defaultMenuItems: MenuItem[] = [
  { name: 'Dashboard', href: '/dashboard', icon: 'dashboard' },
  { name: 'Organizations', href: '/organizations', icon: 'organizations', accessScopes: [AppScopes.Organizations, AppScopes.OrganizationsRead, AppScopes.SuperAdmin] },
  {
    name: 'Admin', icon: 'admin', children: [
      { name: 'Users', href: '/admin/users', icon: 'users', accessScopes: [AppScopes.SuperAdmin] },
      { name: 'Groups', href: '/admin/groups', icon: 'groups', accessScopes: [AppScopes.SuperAdmin] },
      { name: 'Queue', href: '/admin/queue', icon: 'queue', accessScopes: [AppScopes.SuperAdmin, AppScopes.AdminQueue, AppScopes.AdminQueueRead] },
      { name: 'Observe', href: '/admin/observe', icon: 'observe', accessScopes: [AppScopes.SuperAdmin, AppScopes.ObserveView, AppScopes.ObserveManage] },
      { name: 'Security', href: '/admin/security', icon: 'security', accessScopes: [AppScopes.SuperAdmin, AppScopes.AdminSecurityIpBansRead, AppScopes.AdminSecurityIpAllowlistRead, AppScopes.AdminSecurityAttacksRead] },
      { name: 'Settings', href: '/admin/settings', icon: 'settings', accessScopes: [AppScopes.SuperAdmin, AppScopes.AdminSettingsGeneral, AppScopes.AdminSettingsGeneralRead, AppScopes.AdminMailTemplates, AppScopes.AdminMailTemplatesRead, AppScopes.AdminMailSettings, AppScopes.AdminMailSettingsRead] }
    ]
  }
];
