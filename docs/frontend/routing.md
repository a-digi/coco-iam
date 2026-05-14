# Frontend Routing

**Router:** React Router DOM 7 (`BrowserRouter` with `<Routes>` and `<Route>`)

Routes are defined in `src/config/routing/routes.tsx` as a `RouteConfig[]` array and rendered in `App.tsx`. Every protected route is wrapped in `AuthGuard` at definition time — there is no separate route-level middleware.

---

## Full Route Table

| Path | Component | Required Scope(s) | Public |
|---|---|---|---|
| `/` | `Dashboard` | `user:me` (any authenticated user) | No |
| `/login` | `AdminLogin` | — | Yes |
| `/logout` | `Logout` | any authenticated session | No |
| `/standards` | `StandardDashboard` | any authenticated session | No |
| `/standards/import` | `StandardImport` | any authenticated session | No |
| `/admin/users` | `AdminUsersDashboard` | `admin:users:read` or `super:admin` | No |
| `/admin/users/create` | `CreateUser` | `admin:users:write` or `super:admin` | No |
| `/admin/users/edit/:id` | `EditUser` | `user:me`, `admin:users:read`, or `super:admin` | No |
| `/admin/groups` | `AdminGroupsDashboard` | `admin:groups:read` or `super:admin` | No |
| `/admin/groups/create` | `CreateGroup` | `admin:groups:write` or `super:admin` | No |
| `/admin/groups/edit/:id` | `EditGroup` | `admin:groups:read` or `super:admin` | No |

Notes:
- Routes with no `accessScopes` prop on `AuthGuard` (Dashboard, Standards, Logout) require only a valid authenticated session.
- `/admin/users/edit/:id` includes `user:me` in its scope list, meaning an ordinary authenticated user can access their own edit page. The `EditUser` component receives an `accessMe` prop from `ScopeBasedComponentAccess` to conditionally restrict which fields are editable.
- `super:admin` is always accepted in place of any more specific admin scope. This is enforced inside `AuthGuard`, not as a separate route.

---

## How `AuthGuard` Works

`AuthGuard` (`src/Components/Auth/Guard/AuthGuard.tsx`) is a wrapper component placed around every protected route element. It performs two sequential checks.

### Check 1: Authentication

```typescript
const { authenticated, authToken } = useAuth();

if (!authenticated || !authToken?.access_token) {
  return <Navigate to="/login" state={{ from: location }} replace />;
}
```

`authenticated` is a boolean stored in `AuthProvider` state, initialised synchronously from `localStorage` key `auth_token` on first render. If it is false, or if the token object lacks an `access_token` string, the user is immediately redirected to `/login`. The current `location` is passed in router state so a post-login redirect can restore the original destination.

### Check 2: Scope

This check only runs when `AuthGuard` receives a non-empty `accessScopes` prop.

```typescript
if (accessScopes && accessScopes.length > 0) {
  const payload = parseJwt(authToken.access_token);
  const userScopes: string[] = payload?.scopes ?? [];

  const allowsUserMe    = accessScopes.includes(AppScopes.UserMe);
  const hasStandardAccess = accessScopes.some(scope => userScopes.includes(scope));
  const isSuperAdmin    = userScopes.includes(AppScopes.SuperAdmin);

  if (!isSuperAdmin && !allowsUserMe && !hasStandardAccess) {
    return <Navigate to="/" replace />;
  }
}
```

Pass conditions (any one is sufficient):

1. The user's JWT contains `super:admin` — grants access to everything regardless of the specific scopes listed.
2. `accessScopes` includes `user:me` — grants access to any authenticated user for self-service routes.
3. At least one scope in `accessScopes` is present in the user's decoded JWT scopes — standard permission check.

If none of the three conditions are met, the user is redirected to `/` (the dashboard) rather than to `/login`, since they are authenticated but unauthorised.

### `accessScopes` values in `routes.tsx`

The `accessScopes` arrays in the route definitions include both the specific scope and `super:admin` explicitly:

```typescript
// Example from routes.tsx
<AuthGuard accessScopes={[AppScopes.AdminUsersRead, AppScopes.SuperAdmin]}>
  <AdminUsersDashboard />
</AuthGuard>
```

While the `isSuperAdmin` check inside `AuthGuard` makes the explicit inclusion of `AppScopes.SuperAdmin` in the array redundant, it serves as documentation at the route definition level.

---

## How `ScopeBasedComponentAccess` Works

`ScopeBasedComponentAccess` (`src/Shared/Components/Access/ScopeBasedComponentAccess.tsx`) controls the visibility of individual UI elements — buttons, sections, action columns — based on the current user's scopes. It is distinct from `AuthGuard` in that it does not redirect; it either renders its child or renders nothing.

```typescript
<ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminUsersWrite]}>
  <CreateAction to="/admin/users/create" />
</ScopeBasedComponentAccess>
```

### Rendering logic

| Condition | Output |
|---|---|
| No valid `authToken` or JWT parse fails | `null` |
| `requiredScopes` is empty | Renders child with `accessMe: false` |
| Any non-`user:me` scope in `requiredScopes` is in the user's scopes | Renders child with `accessMe: false` |
| `requiredScopes` contains `user:me` but user lacks all other scopes | Renders child with `accessMe: true` |
| None of the above | `null` |

The `accessMe` prop is injected via `React.cloneElement`. Child components that implement the `ScopeAccessAware` interface can inspect this prop to apply additional restrictions — for example, an edit form that shows all fields to admins but only certain fields to the account owner.

`super:admin` is not given special treatment inside `ScopeBasedComponentAccess`. A `super:admin` user must still have the relevant non-`user:me` scope in their JWT for the component to render. In practice, the backend grants all scopes to `super:admin` users.

---

## JWT Decode on the Frontend

JWT decoding happens client-side in `src/config/security/jtw.ts` via `parseJwt()`. No third-party library is used.

```
Header.Payload.Signature
       ^^^^^^^ extracted
```

The function:

1. Splits the token on `.` and takes the second segment (the payload).
2. Replaces URL-safe Base64 characters (`-` → `+`, `_` → `/`) to produce standard Base64.
3. Decodes with `window.atob()`.
4. Re-encodes each character as a URI percent-escape, then decodes the whole string with `decodeURIComponent` to correctly handle multi-byte UTF-8 characters.
5. Parses the resulting string as JSON.

After parsing, there is a normalisation step: if the payload contains a space-separated `scope` string (OAuth 2.0 standard format) rather than a `scopes` array, the function splits it and assigns the result to `result.scopes`. This means the rest of the application always reads `payload.scopes` as a string array regardless of which format the server uses.

`parseJwt` returns `null` on any error (malformed token, JSON parse failure, etc.) and logs the error to the console. Callers treat a `null` result as equivalent to no scopes.

**Where it is called:**

- `AuthGuard.tsx` — to extract scopes for route protection.
- `ScopeBasedComponentAccess.tsx` — to extract scopes for UI element visibility.

The signature is not verified client-side. JWT verification is the responsibility of the backend on every API request.

---

## AppScopes Reference

All scope constants are defined in `src/config/security/scopes.ts` as a single `as const` object. The `AppScope` type is derived from the object values, so TypeScript enforces that only valid scope strings are used where a scope type is expected.

| Constant | Scope string | Controls |
|---|---|---|
| `AppScopes.UserMe` | `user:me` | Self-service access; any authenticated user |
| `AppScopes.SuperAdmin` | `super:admin` | Full unrestricted access to all resources |
| `AppScopes.AdminAcl` | `admin:acl` | Top-level ACL management |
| `AppScopes.AdminAclRead` | `admin:acl:read` | Read ACL entries |
| `AppScopes.AdminUsers` | `admin:users` | Top-level user management |
| `AppScopes.AdminUsersRead` | `admin:users:read` | List and view users |
| `AppScopes.AdminUsersWrite` | `admin:users:write` | Create and update users |
| `AppScopes.AdminUsersDelete` | `admin:users:delete` | Delete users |
| `AppScopes.AdminUsersAcl` | `admin:users:acl` | Top-level user ACL management |
| `AppScopes.AdminUsersAclRead` | `admin:users:acl:read` | Read user ACL entries |
| `AppScopes.AdminUsersAclWrite` | `admin:users:acl:write` | Write user ACL entries |
| `AppScopes.AdminUsersAclDelete` | `admin:users:acl:delete` | Delete user ACL entries |
| `AppScopes.AdminGroups` | `admin:groups` | Top-level group management |
| `AppScopes.AdminGroupsRead` | `admin:groups:read` | List and view groups |
| `AppScopes.AdminGroupsWrite` | `admin:groups:write` | Create and update groups |
| `AppScopes.AdminGroupsDelete` | `admin:groups:delete` | Delete groups |
| `AppScopes.AdminGroupsAcl` | `admin:groups:acl` | Top-level group ACL management |
| `AppScopes.AdminGroupsAclRead` | `admin:groups:acl:read` | Read group ACL entries |
| `AppScopes.AdminGroupsAclWrite` | `admin:groups:acl:write` | Write group ACL entries |
| `AppScopes.AdminGroupsAclDelete` | `admin:groups:acl:delete` | Delete group ACL entries |

---

## Adding a New Protected Route

The following steps cover the complete process from scope definition to a working route with controlled sidebar visibility.

### Step 1: Define scopes (if new ones are needed)

Open `src/config/security/scopes.ts` and add the new entries to `AppScopes`:

```typescript
AdminReportsRead: 'admin:reports:read',
AdminReportsWrite: 'admin:reports:write',
```

No additional type declarations are required. The `AppScope` union type is derived automatically.

### Step 2: Create the component

Place the component under `src/Components/<Domain>/`. See the structure document for the recommended directory layout. The component itself has no knowledge of routing — it is a plain React component.

### Step 3: Register the route

Open `src/config/routing/routes.tsx`. Import the component and add an entry to the `routes` array:

```typescript
import ReportsDashboard from '../../Components/Reports/Dashboard/Dashboard';
import CreateReport from '../../Components/Reports/Create/CreateReport';
import { AppScopes } from '../security/scopes';

// Add to the routes array:
{
  path: '/reports',
  element: (
    <AuthGuard accessScopes={[AppScopes.AdminReportsRead]}>
      <ReportsDashboard />
    </AuthGuard>
  ),
},
{
  path: '/reports/create',
  element: (
    <AuthGuard accessScopes={[AppScopes.AdminReportsWrite]}>
      <CreateReport />
    </AuthGuard>
  ),
},
```

`AuthGuard` will redirect unauthenticated users to `/login` and users without the required scope to `/`.

### Step 4: Add to the sidebar menu

Open `src/config/menu/menu.ts` and add the entry to `defaultMenuItems`. The `accessScopes` field on a menu item controls whether that item is rendered in the sidebar for the current user:

```typescript
{
  name: 'Reports', children: [
    { name: 'Reports', href: '/reports', accessScopes: [AppScopes.AdminReportsRead] },
    { name: 'New Report', href: '/reports/create', accessScopes: [AppScopes.AdminReportsWrite] },
  ]
}
```

### Step 5: Guard UI elements within the component (optional)

To show or hide individual buttons or sections based on scope, wrap them in `ScopeBasedComponentAccess`:

```typescript
import { ScopeBasedComponentAccess } from '../../../Shared/Components/Access/ScopeBasedComponentAccess';
import { AppScopes } from '../../../config/security/scopes';

// Inside JSX:
<ScopeBasedComponentAccess requiredScopes={[AppScopes.AdminReportsWrite]}>
  <Submit type="button" onClick={handleCreate} label="New Report" />
</ScopeBasedComponentAccess>
```

The button will not render for users who lack `admin:reports:write`. No error is shown; the element is simply absent from the DOM.
