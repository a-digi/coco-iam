# Scope System

This document describes how scopes are defined, stored, declared on routes, and enforced in both the backend and the frontend.

---

## What Are Scopes?

Scopes are string identifiers that represent discrete permissions within coco-iam. A scope grants the right to perform a specific action (read, write, delete) on a specific resource (users, groups, ACL entries). Scopes are embedded in the JWT at login time and checked on every protected request.

Scopes follow a colon-separated naming convention: `<domain>:<resource>:<action>`. Broader scopes (e.g. `admin:users`) act as umbrella grants that allow routes accepting more specific scopes to be accessed.

---

## Full Scope Catalog

Scopes are defined as constants in both:
- Backend: `api/lift/security/scopes.go` and referenced throughout route and handler code
- Frontend: `app/src/config/security/scopes.ts` as the `AppScopes` object

| Scope                        | What It Grants                                              |
|------------------------------|-------------------------------------------------------------|
| `user:me`                    | Access to the authenticated user's own profile and ACL data |
| `super:admin`                | Unrestricted access to all resources and operations         |
| `admin:me`                   | Access to the current user's group memberships and ACL via `/me/*` routes |
| `admin:acl`                  | Full access to all ACL resources (users and groups)         |
| `admin:acl:read`             | Read access to all ACL resources                            |
| `admin:users:read`           | Read users (list and single record)                         |
| `admin:users:write`          | Create and update users                                     |
| `admin:users:delete`         | Delete users                                                |
| `admin:users:acl`            | Full access to user ACL entries                             |
| `admin:users:acl:read`       | Read user ACL entries                                       |
| `admin:users:acl:write`      | Create and update user ACL entries                          |
| `admin:users:acl:delete`     | Delete user ACL entries                                     |
| `admin:groups:read`          | Read groups, group members (list and single record)         |
| `admin:groups:write`         | Create and update groups and group memberships              |
| `admin:groups:delete`        | Delete groups and group memberships                         |
| `admin:groups:acl`           | Full access to group ACL entries                            |
| `admin:groups:acl:read`      | Read group ACL entries                                      |
| `admin:groups:acl:write`     | Create and update group ACL entries                         |
| `admin:groups:acl:delete`    | Delete group ACL entries                                    |

**Note:** `super:admin` is the only scope that is assigned implicitly — it is added to a user's effective scope list when their `is_super_admin` flag is `true` in `admin_users`, even if no corresponding row exists in `user_acl`.

---

## How Scopes Are Stored in the Database

Scopes are not stored as individual rows. They are serialized as a **JSON array of strings** in a `roles` column on the ACL tables.

### Tables

**`user_acl`** — direct scope assignments for individual users

| Column     | Type    | Description                          |
|------------|---------|--------------------------------------|
| `user_id`  | string  | Foreign key to `admin_users.id`      |
| `roles`    | JSON    | Array of scope strings               |
| `is_active`| boolean | Whether this ACL entry is active     |

**`user_group_acl`** — scope assignments at the group level

| Column     | Type    | Description                          |
|------------|---------|--------------------------------------|
| `group_id` | string  | Foreign key to `admin_groups.id`     |
| `roles`    | JSON    | Array of scope strings               |
| `is_active`| boolean | Whether this ACL entry is active     |

### Example `roles` value

```json
["admin:users:read", "admin:groups:read", "admin:acl:read"]
```

### Scope Resolution at Login

When a user logs in, `UserAclRepository.FindUserScopes` assembles their effective scope list:

1. If `admin_users.is_super_admin = true`, `super:admin` is added.
2. All active rows in `user_acl` where `user_id` matches are fetched. Each `roles` JSON array is unmarshalled and all scopes are added to a deduplication map.
3. All active rows in `user_group_acl` for any group the user belongs to (via `user_group_members`) are fetched. Their scopes are added to the same map.
4. The final deduplicated list is embedded in the JWT as the `scope` claim.

This means scope changes (adding or removing ACL entries) take effect at the user's next login or token renewal, not immediately.

---

## How Routes Declare Required Scopes

Routes are declared in `api/config/routes/routes.yaml`. Each route entry may include a `scopes` list:

```yaml
- path: /{res:users}
  method: GET
  executor: ApiResourceHandler
  security: authenticated
  scopes:
    - admin:users:read
```

```yaml
- path: /{res:user_acl}
  method: POST
  executor: ApiResourceHandler
  security: authenticated
  scopes:
    - admin:users:acl:write
    - admin:users:acl
```

**Scope inheritance:** Routes are organized in a parent-child hierarchy in the YAML file. When `ScopeSecurityLayer` starts up, it flattens this tree and merges parent scopes with child scopes. A child route's effective scope list is the union of all ancestor scopes and its own declared scopes, deduplicated.

**Public routes** declare `security: public` and require no token at all. The two public routes are:
- `POST /api/v1/admin/oauth/authenticate` (login)
- `POST /api/v1/admin/oauth/renew` (token renewal)

---

## How ScopeSecurityLayer Enforces Scopes

`ScopeSecurityLayer` (`api/lift/security/scope_based_auth_layer.go`) is registered as the security middleware for the HTTP server.

On each request:

1. The matched route's `security` and `scopes` values are read from the flattened route map.
2. If `security == "public"`, the request passes through without any token check.
3. If `security == "authenticated"` and `scopes` is empty, any valid JWT is sufficient.
4. If `scopes` is non-empty:
   a. The `Authorization: Bearer <token>` header is extracted.
   b. The HS256 signature and expiry are validated by `coco-oauth`'s `Validator`.
   c. The `sub` and `scopes` claims are extracted from the token.
   d. **OR logic** is applied: the request is authorized if the user's scopes contain **at least one** of the route's required scopes.
   e. Exception: if `admin:me` is in the route's required scope list, any authenticated user is allowed through — the scope content of their token is not checked beyond validity.
5. On authorization failure, the server responds with:
   - `401 {"message": "Access denied"}` — missing or invalid token
   - `403 {"message": "Access denied"}` — valid token but insufficient scopes

---

## OR Logic for Multi-Scope Routes

When a route lists multiple scopes, the check is satisfied by **any one** of them. This is used throughout the API to allow both broad and narrow grants to access the same route.

**Example:**

```yaml
scopes:
  - admin:users:acl:write
  - admin:users:acl
```

A user with only `admin:users:acl:write` is permitted. A user with only `admin:users:acl` (the umbrella scope) is also permitted. A user with `super:admin` is permitted on all routes because that scope is present in their token and any route that includes it in its effective scope list will match.

This design means you can assign granular scopes (`admin:users:acl:write`) for narrow delegation, or the umbrella scope (`admin:users:acl`) for broader access to a family of related routes, without needing separate route definitions.

---

## Frontend Scope Enforcement

The frontend enforces scopes at two levels: route access and component rendering.

### Route Access: AuthGuard

`AuthGuard` (`app/src/Components/Auth/Guard/AuthGuard.tsx`) wraps each React Router route that requires protection.

```tsx
<AuthGuard accessScopes={[AppScopes.AdminUsersRead]}>
  <UsersDashboard />
</AuthGuard>
```

On render:

1. If the user is not authenticated, they are redirected to `/login`.
2. If `accessScopes` is provided and non-empty, the JWT is parsed client-side with `parseJwt`. The `scope` claim is split on spaces to produce a `scopes` array.
3. The check passes if **any** of the following are true:
   - The user holds `super:admin`.
   - `user:me` is in `accessScopes` (self-service routes accessible to any authenticated user).
   - The user's scopes include at least one entry from `accessScopes`.
4. If the check fails, the user is redirected to `/`.

### Component Rendering: ScopeBasedComponentAccess

`ScopeBasedComponentAccess` (`app/src/Shared/Components/Access/ScopeBasedComponentAccess.tsx`) conditionally renders UI elements based on the current user's scopes. It is used to hide or show action buttons, form sections, and navigation items.

```tsx
<ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminUsersWrite]}>
  <CreateAction />
</ScopeBasedComponentAccess>
```

On render:

1. If no token is present, `null` is returned (nothing is rendered).
2. The JWT is parsed and the scopes array is extracted.
3. If `requiredScopes` is empty, the child is rendered unconditionally.
4. `user:me` is treated separately. Non-`user:me` scopes are checked first using OR logic.
5. If the user holds any non-`user:me` required scope, the child is rendered with `accessMe: false` injected as a prop.
6. If none of the non-`user:me` scopes match but `user:me` is in `requiredScopes`, the child is rendered with `accessMe: true` injected — indicating the component should restrict itself to showing only the user's own data.
7. If no condition is satisfied, `null` is returned.

The `accessMe: boolean` prop allows a single component to handle both full-access and self-service rendering modes without requiring two separate components.

---

## Scope Definitions Reference

| Location                                      | Language   | Purpose                            |
|-----------------------------------------------|------------|------------------------------------|
| `api/lift/security/scopes.go`                 | Go         | Backend constants for special scopes (`admin:me`) |
| `app/src/config/security/scopes.ts`           | TypeScript | Frontend `AppScopes` object, all scope strings |
| `api/config/routes/routes.yaml`               | YAML       | Route-level scope requirements     |

---

## Related Documentation

- [Architecture Overview](./overview.md)
- [Authentication Flow](./auth-flow.md)
- API Endpoint Reference — see `docs/api/` for per-endpoint scope requirements
