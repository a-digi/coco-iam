export const AppScopes = {
    // User himself
    UserMe: 'user:me',

    // Super Admin
    SuperAdmin: 'super:admin',

    // ACL
    AdminAcl: 'admin:acl',
    AdminAclRead: 'admin:acl:read',

    // Users
    AdminUsers: 'admin:users',
    AdminUsersRead: 'admin:users:read',
    AdminUsersWrite: 'admin:users:write',
    AdminUsersDelete: 'admin:users:delete',

    // User ACLs
    AdminUsersAcl: 'admin:users:acl',
    AdminUsersAclRead: 'admin:users:acl:read',
    AdminUsersAclWrite: 'admin:users:acl:write',
    AdminUsersAclDelete: 'admin:users:acl:delete',

    // Groups
    AdminGroups: 'admin:groups',
    AdminGroupsRead: 'admin:groups:read',
    AdminGroupsWrite: 'admin:groups:write',
    AdminGroupsDelete: 'admin:groups:delete',

    // Group ACLs
    AdminGroupsAcl: 'admin:groups:acl',
    AdminGroupsAclRead: 'admin:groups:acl:read',
    AdminGroupsAclWrite: 'admin:groups:acl:write',
    AdminGroupsAclDelete: 'admin:groups:acl:delete',

    // Workspaces
    Workspaces: 'workspaces',
    WorkspacesRead: 'workspaces:read',
    WorkspacesWrite: 'workspaces:write',
    WorkspacesDelete: 'workspaces:delete',

    // Organizations
    Organizations: 'organizations',
    OrganizationsRead: 'organizations:read',
    OrganizationsWrite: 'organizations:write',
    OrganizationsDelete: 'organizations:delete',

    // Organization Users
    OrganizationsUsers: 'organizations:users',
    OrganizationsUsersRead: 'organizations:users:read',
    OrganizationsUsersWrite: 'organizations:users:write',
    OrganizationsUsersDelete: 'organizations:users:delete',

    // Organization User ACL
    OrganizationsUsersAcl: 'organizations:users:acl',
    OrganizationsUsersAclRead: 'organizations:users:acl:read',
    OrganizationsUsersAclWrite: 'organizations:users:acl:write',
    OrganizationsUsersAclDelete: 'organizations:users:acl:delete',

    // Organization Groups
    OrganizationsGroups: 'organizations:groups',
    OrganizationsGroupsRead: 'organizations:groups:read',
    OrganizationsGroupsWrite: 'organizations:groups:write',
    OrganizationsGroupsDelete: 'organizations:groups:delete',

    // Organization Group ACL
    OrganizationsGroupsAcl: 'organizations:groups:acl',
    OrganizationsGroupsAclRead: 'organizations:groups:acl:read',
    OrganizationsGroupsAclWrite: 'organizations:groups:acl:write',
    OrganizationsGroupsAclDelete: 'organizations:groups:acl:delete',

    // Applications
    Applications: 'applications',
    ApplicationsRead: 'applications:read',
    ApplicationsWrite: 'applications:write',
    ApplicationsDelete: 'applications:delete',

    // Application Users
    ApplicationsUsers: 'applications:users',
    ApplicationsUsersRead: 'applications:users:read',
    ApplicationsUsersWrite: 'applications:users:write',
    ApplicationsUsersDelete: 'applications:users:delete',

    // Application Scopes
    ApplicationsScopes: 'applications:scopes',
    ApplicationsScopesRead: 'applications:scopes:read',
    ApplicationsScopesWrite: 'applications:scopes:write',
    ApplicationsScopesDelete: 'applications:scopes:delete',

    // Application ACL
    ApplicationsAcl: 'applications:acl',
    ApplicationsAclRead: 'applications:acl:read',
    ApplicationsAclWrite: 'applications:acl:write',
    ApplicationsAclDelete: 'applications:acl:delete',

    // Application keys
    ApplicationsKeys: 'applications:keys',
    ApplicationsKeysRead: 'applications:keys:read',
    ApplicationsKeysReadPrivate: 'applications:keys:read_private',

    // Application machine-auth API credentials (for /a/... endpoints)
    ApplicationsApiCredentials: 'applications:api_credentials',
    ApplicationsApiCredentialsRead: 'applications:api_credentials:read',

    // Application external-IdP OAuth providers — admin config for
    // Google / GitHub / Microsoft "Continue with …" buttons.
    ApplicationsOauth: 'applications:oauth',
    ApplicationsOauthRead: 'applications:oauth:read',

    // Application registration schema (steps + fields published at
    // /a/{org}/{ws}/{app}/registration-fields)
    ApplicationsRegistrationFields: 'applications:registration_fields',
    ApplicationsRegistrationFieldsRead: 'applications:registration_fields:read',

    // Application login templates (layout + branding only; the
    // allow_recovery / allow_registration feature flags live on the
    // Application entity itself, edited via applications:write).
    ApplicationsLoginTemplates: 'applications:login_templates',
    ApplicationsLoginTemplatesRead: 'applications:login_templates:read',
    ApplicationsLoginTemplatesWrite: 'applications:login_templates:write',

    // Application analytics (Detail panel widgets)
    ApplicationsAnalytics: 'applications:analytics',
    ApplicationsAnalyticsRead: 'applications:analytics:read',
    ApplicationsAnalyticsUsersRead: 'applications:analytics:users:read',
    ApplicationsAnalyticsGroupsRead: 'applications:analytics:groups:read',
    ApplicationsAnalyticsScopesRead: 'applications:analytics:scopes:read',
    ApplicationsAnalyticsRecentGrantsRead: 'applications:analytics:recent_grants:read',
    ApplicationsAnalyticsPendingRecoveriesRead: 'applications:analytics:pending_recoveries:read',

    // Admin Queue
    AdminQueue: 'admin:queue',
    AdminQueueRead: 'admin:queue:read',
    AdminQueueWrite: 'admin:queue:write',

    // Admin Mail Templates
    AdminMailTemplates: 'admin:mail:templates',
    AdminMailTemplatesRead: 'admin:mail:templates:read',
    AdminMailTemplatesWrite: 'admin:mail:templates:write',
    AdminMailTemplatesDelete: 'admin:mail:templates:delete',

    // Admin Mail Settings
    AdminMailSettings: 'admin:mail:settings',
    AdminMailSettingsRead: 'admin:mail:settings:read',
    AdminMailSettingsWrite: 'admin:mail:settings:write',

    // Admin General Settings
    AdminSettingsGeneral: 'admin:settings:general',
    AdminSettingsGeneralRead: 'admin:settings:general:read',
    AdminSettingsGeneralWrite: 'admin:settings:general:write',

    // User rules (admin-wide)
    AdminSettingsUserRules: 'admin:settings:user_rules',
    AdminSettingsUserRulesRead: 'admin:settings:user_rules:read',
    AdminSettingsUserRulesWrite: 'admin:settings:user_rules:write',

    // Admin self — the authenticated admin's own profile/groups/acl.
    // Shared by /me endpoints on the backend.
    AdminMe: 'admin:me',

    // Admin Dashboard
    AdminDashboard: 'admin:dashboard',
    AdminDashboardRead: 'admin:dashboard:read',
    AdminDashboardStatsRead: 'admin:dashboard:stats:read',
    AdminDashboardRegistrationsRead: 'admin:dashboard:registrations:read',
    AdminDashboardTopOrgsRead: 'admin:dashboard:top_orgs:read',
    AdminDashboardQueueRead: 'admin:dashboard:queue:read',
    AdminDashboardRecentUsersRead: 'admin:dashboard:recent_users:read',
    AdminDashboardFailedTasksRead: 'admin:dashboard:failed_tasks:read',

    // Organization Profile Fields (admin schema)
    OrganizationsProfileFields: 'organizations:profile_fields',
    OrganizationsProfileFieldsRead: 'organizations:profile_fields:read',
    OrganizationsProfileFieldsWrite: 'organizations:profile_fields:write',
    OrganizationsProfileFieldsDelete: 'organizations:profile_fields:delete',

    // Organization User Profile Values
    OrganizationsUsersProfile: 'organizations:users:profile',
    OrganizationsUsersProfileRead: 'organizations:users:profile:read',
    OrganizationsUsersProfileWrite: 'organizations:users:profile:write',

    // coco-observe — system observability
    ObserveView: 'observe:view',
    ObserveManage: 'observe:manage',
} as const;

export type AppScope = typeof AppScopes[keyof typeof AppScopes];
