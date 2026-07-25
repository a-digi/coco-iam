// Package main is the entry point for the CoCo IAM API server.
//
//	@title			CoCo IAM API
//	@version		1.0
//	@description	Multi-tenant Identity and Access Management.
//	@description
//	@description	This API has two independent Bearer-token systems. Both use the same
//	@description	`Authorization: Bearer <token>` header, but the tokens are NOT interchangeable:
//	@description
//	@description	- **Admin console** (every tag below not prefixed `public-app-`): an HS256 JWT
//	@description	issued by the admin login flow, scoped via strings like `organizations:read`,
//	@description	`applications:write`, `super:admin`. Manages organizations, workspaces,
//	@description	applications, and admin users.
//	@description
//	@description	- **Per-application public API** (tags prefixed `public-app-`, routes under
//	@description	`/public/applications/{id}/...`): an RS256 JWT signed with that *specific
//	@description	application's own keypair*, scoped via that application's own
//	@description	`application_user_acl.roles` (e.g. `users:read`, `scopes:write`). A token
//	@description	minted for one application returns 401 against any other application's
//	@description	`{id}` — these are end-user tokens for that one app, never admin credentials.
//	@host			localhost:2026
//	@BasePath		/api/v1
//
//	@tag.name		public-app-users
//	@tag.description	Manage an application's own end-users (create, list, patch, soft-delete, set password). Requires that application's `users:*` scope.
//
//	@tag.name		public-app-user-acl
//	@tag.description	Read or replace a single user's role assignment within an application. Requires that application's `acl:*` scope.
//
//	@tag.name		public-app-groups
//	@tag.description	Manage user groups scoped to an application (create, list, patch, soft-delete). Requires that application's `groups:*` scope.
//
//	@tag.name		public-app-group-members
//	@tag.description	Add, remove, and list which users belong to an application's groups. Requires that application's `groups:*` scope.
//
//	@tag.name		public-app-group-acl
//	@tag.description	Read or replace a group's role assignment within an application. Requires that application's `acl:*` scope.
//
//	@tag.name		public-app-scopes
//	@tag.description	Manage an application's own scope catalog — the vocabulary of permissions it exposes to its users. Requires that application's `scopes:*` scope.
//
//	@tag.name		public-app-me
//	@tag.description	Self-service preferences for the currently authenticated application end-user (e.g. password-expiry notification settings).
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer <JWT token>. See the API description above — which JWT you need depends on which tag the operation carries.
package main
