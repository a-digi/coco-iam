# Frontend Structure

**Stack:** React 19.2, TypeScript 5.9, Vite, Tailwind CSS 4, React Router DOM 7

---

## Component Tree

The application mounts from `main.tsx`, which renders `App.tsx` inside React's `StrictMode`. All global providers are composed in `App.tsx` before any routing or layout logic runs.

```
main.tsx
└── App (StrictMode)
    └── HttpClientProvider
        └── SnackBarProvider
            └── AuthProvider
                └── MenuProvider
                    └── BrowserRouter
                        └── Layout
                            └── LayoutProvider
                                └── ThemeProvider
                                    └── SidebarProvider
                                        ├── TopBar
                                        ├── Sidebar (Layout/Sidebar)
                                        │   └── SideBarMenu (Components/Menu/Sidebar)
                                        └── Content
                                            └── <Routes> (feature components)
```

Provider nesting order is significant. `HttpClientProvider` sits at the outermost position so every component — including auth — can make HTTP requests. `AuthProvider` is inside `SnackBarProvider` so auth actions can surface toast notifications. `BrowserRouter` is inside `MenuProvider` so menu items can use React Router links.

The `Layout` component (`Layout/index.tsx`) is a thin wrapper that renders `LayoutProvider`. `LayoutProvider` (`Layout/LayoutContext.tsx`) composes `ThemeProvider` and `SidebarProvider` and orchestrates the three structural regions: `TopBar`, `Sidebar`, and `Content`. When the user is not authenticated, the layout renders only the `Content` region — the sidebar and top bar are hidden entirely.

---

## Directory Conventions

### `src/Components/` — Feature-specific components

Each subdirectory corresponds to a product domain. Components here own their own state, make their own API calls, and are not intended to be reused across features.

```
Components/
├── Auth/
│   ├── Guard/        AuthProvider, AuthGuard, AuthConstants, useAuth hook
│   ├── Login/Admin/  AdminLogin form
│   ├── Logout/       Logout component
│   └── Scopes/       Scope display utilities
├── Admin/
│   ├── Users/        Dashboard, Create, Edit, Form, model
│   └── Groups/       Dashboard, Create, Edit, Form, model
├── Dashboard/        Root dashboard
├── Menu/             MenuProvider, sidebar menu, MenuContext
└── Standard/         StandardDashboard, StandardImport, model
```

Each domain folder typically contains:

- `Dashboard/` — list view with table, filters, and delete confirmation
- `Create/` — form wrapped in a create handler
- `Edit/` — form pre-populated from an API fetch, wrapped in an update handler
- `Form/` — the shared form markup used by both Create and Edit
- `model/` — TypeScript interfaces and the field mapping `Schema` for that resource

### `src/Shared/Components/` — Reusable UI primitives

These components have no knowledge of any particular domain. They accept props and emit callbacks.

| Component | Purpose |
|---|---|
| `Access/ScopeBasedComponentAccess` | Renders children only when the current user holds a required scope |
| `Accordion/` | Collapsible section UI — `Accordions` (list) + `AccordionItem` |
| `Tabs/` | Horizontal tab UI — `Tabs` component takes `items: TabData[]` with `{ id, title, content, scopes? }`; renders a tab bar and the active panel |
| `Actions/` | `EditAction`, `DeleteAction`, `CreateAction`, `ViewAction`, `ActionButton`, `Link` |
| `Badge/` | Coloured badge variants: Default, Info, Success, Warning, Danger |
| `Button/` | `Submit`, `SubmitSmall`, `Add`, `Cancel`, `Close`, `Remove` |
| `Dropdown/` | Generic dropdown |
| `Filter/` | `Filter`, `FilterItem` for table filter bars |
| `Font/Title` | Page-level heading typography |
| `Grid/` | `Grid`, `GridCard` layout helpers |
| `Masonry/` | Masonry layout |
| `Modal/` | `Modal`, `ConfirmModal` (with danger/default variant and loading state) |
| `NoEntries/` | Empty-state placeholder |
| `Pagination/` | Page navigation control |
| `SnackBar/` | Toast notification system (see Context Providers below) |
| `Switch/` | Toggle switch |
| `Table/` | Base `Table` component |
| `TableView/` | Opinionated table with built-in filter bar and pagination |
| `User/Menu/UserMenu` | Authenticated user avatar/menu in the top bar |

---

## Context Providers

All providers are React contexts with a paired `use*` hook. Attempting to call a hook outside its provider throws at runtime.

### `AuthProvider` (`Components/Auth/Guard/AuthContext.tsx`)

Manages the authentication lifecycle.

| Value | Type | Description |
|---|---|---|
| `authenticated` | `boolean` | Derived from whether `auth_token` exists in `localStorage` |
| `authToken` | `AuthToken \| null` | Parsed token object: `access_token`, `refresh_token`, `token_type`, `expires_at`, `user.id` |
| `login(token)` | `(AuthToken) => void` | Persists token to `localStorage`, sets both state values |
| `logout()` | `() => void` | Removes token from `localStorage`, clears both state values |
| `setAuthenticated` | setter | Direct setter, used internally |
| `setAuthToken` | setter | Direct setter, used internally |

Consumed via `useAuth()` from `Components/Auth/Guard/useAuth.ts`.

Initial state is hydrated synchronously from `localStorage` so there is no unauthenticated flash on page reload.

### `LayoutProvider` (`Layout/LayoutContext.tsx`)

Owns three injectable content slots: `sidebarContent`, `topbarContent`, and `mainContent`. Feature components can push content into these slots via setters. The provider itself orchestrates `ThemeProvider` and `SidebarProvider`.

Consumed via `useContext(LayoutContext)` from `Layout/LayoutContextContext.ts`.

### `ThemeProvider` (`Layout/ThemeContext.tsx`)

| Value | Type | Description |
|---|---|---|
| `theme` | `'light' \| 'dark'` | Current colour scheme |
| `toggleTheme()` | `() => void` | Flips between light and dark |

On mount, reads from `localStorage` key `theme`. Falls back to `window.matchMedia('(prefers-color-scheme: dark)')`. On every change, writes back to `localStorage`, and toggles the `dark` class on both `<html>` and `<body>` so Tailwind's dark-mode utilities activate.

Consumed via `useTheme()` from `Layout/ThemeContextContext.ts`.

### `SidebarProvider` (`Layout/Sidebar/SidebarContext.tsx`)

| Value | Type | Description |
|---|---|---|
| `open` | `boolean` | Whether the sidebar is expanded |
| `toggle()` | `() => void` | Flips `open` |
| `setOpen` | setter | Direct setter for programmatic control |

The `LayoutProvider` reads `open` to apply a left margin to the main content area (`md:ml-[300px]`). On mobile, an overlay intercepts clicks and calls `setOpen(false)`.

Consumed via `useSidebar()` from `Layout/Sidebar/SidebarContextContext.ts`.

### `SnackBarProvider` (`Shared/Components/SnackBar/SnackBarProvider.tsx`)

Toast notification system. Renders fixed-position containers for each of the six supported positions.

| Method | Signature | Description |
|---|---|---|
| `infoMessage` | `(msg, duration?, position?) => void` | Blue info toast |
| `successMessage` | `(msg, duration?, position?) => void` | Green success toast |
| `dangerMessage` | `(msg, duration?, position?) => void` | Orange danger toast |
| `errorMessage` | `(msg, duration?, position?) => void` | Red error toast |
| `removeMessage` | `(id) => void` | Dismisses a specific toast |

Default duration is 5000 ms. Default position is `top-center`. Valid positions: `top-left`, `top-center`, `top-right`, `bottom-left`, `bottom-center`, `bottom-right`.

Consumed via `useSnackBar()` from `Shared/Components/SnackBar/SnackBarContext.tsx`.

### `HttpClientProvider` (`api/http/HttpClient.tsx`)

HTTP client with request deduplication. Wraps all HTTP methods and exposes them through context. See the HTTP Client section below for full details.

Consumed via `useHttpClient()` from `api/http/useHttpClient.ts`.

---

## HTTP Client

### Architecture

The HTTP layer is split into three files:

| File | Responsibility |
|---|---|
| `api/client.ts` | Raw `fetch` wrappers for every HTTP method; builds headers; handles token expiry and renewal |
| `api/get.ts` | GET-specific wrapper that calls `buildHeaders` and `handleResponse` from `client.ts` |
| `api/http/HttpClient.tsx` | React provider that wraps the above functions with request deduplication |

All requests flow through `buildHeaders()` in `client.ts`. That function:

1. Reads the current `AuthToken` from `localStorage`.
2. Checks `token.expires_at` against the current Unix timestamp.
3. If expired, calls `renewToken()` (`api/tokenRenew.ts`), which POSTs to `admin/oauth/renew` with the existing access token as `refresh_token`. On success it persists the new token; on failure it calls `logoutUser()`, which removes the token from `localStorage` and hard-navigates to `/login`.
4. Sets `Authorization: Bearer <access_token>` on the request.

`postPublicApi` bypasses all of this — it sends no `Authorization` header. It is used by the login form and the token renewal call itself.

The base URL for all requests is `http://localhost:2026/api/v1/`.

### Request Deduplication

`HttpClientProvider` maintains a `recentRequests` ref (a plain object, not state — mutations do not trigger renders). Every method call constructs a cache key from the HTTP verb, endpoint, and serialised options string.

```
key = `${verb}:${endpoint}:${JSON.stringify(options)}`
```

Before issuing a fetch, the provider checks whether an in-flight promise for the same key was started within the last 100 ms. If so, the existing promise is returned instead of creating a new request. This prevents duplicate network calls caused by multiple components mounting simultaneously and requesting the same resource.

Once a promise settles (via `.finally()`), its entry is removed from the ref so subsequent calls are not suppressed.

### 401 Handling

`handleResponse()` in `client.ts` throws an `Error` for any non-2xx response. Components catch this error and display it via `useSnackBar`. Separately, if the token is found to be expired before a request is dispatched and renewal fails, `logoutUser()` immediately navigates to `/login` — the error never reaches the component.

### Available Methods on `useHttpClient()`

| Method | Sends auth header | Notes |
|---|---|---|
| `get(endpoint, options?)` | Yes | |
| `post(endpoint, data?)` | Yes | JSON body |
| `postMultipart(endpoint, formData)` | No | Used for file uploads; no `Content-Type` override |
| `postPublicApi(endpoint, data?)` | No | For unauthenticated endpoints |
| `put(endpoint, data?)` | Yes | JSON body |
| `patch(endpoint, data?)` | Yes | JSON body |
| `del(endpoint, options?)` | Yes | |

---

## Repository Pattern (`config/data/resource/repository.ts`)

`useResourceRepository()` is a hook that provides a single method, `fetchCollection`, which abstracts the three steps common to every list view: fetching, mapping, and filtering.

```typescript
const { fetchCollection } = useResourceRepository();

const items = await fetchCollection<User>(
  'admin/{res:users}',
  StandardSchema,
  { filters: currentFilters }
);
```

**Step 1 — Filter query string.** If `options.filters` is provided, `buildFilterQueryString()` (`config/data/resource/filters.ts`) serialises each `ResourceFilter` into a query parameter of the form:

```
filter[@<operator>:<field>]=<value>
```

Supported operators: `like`, `exact`, `date-gte`, `date-lte`, `gte`, `lte`.

**Step 2 — Fetch.** Calls `get()` from `useHttpClient()`. The response is expected to conform to `ApiCollectionResponse<T>`, which has shape `{ message: T[] | null, success: boolean }`.

**Step 3 — Map.** Passes the raw `message` array through `mapObjects()` (`config/data/mapper/mapper.ts`) with the provided `Schema`. A `Schema` is a plain object where keys are the frontend field names and values are the backend field names:

```typescript
export const StandardSchema: Schema = {
  id: 'id',
  username: 'username',
  email: 'email',
  createdAt: 'created_at',
  isActive: 'is_active',
  isSuperAdmin: 'is_super_admin',
};
```

This decouples the frontend's camelCase property names from the backend's snake_case field names without requiring a transformation in every component.

---

## Adding a New Feature Component

### 1. Create the domain directory

Place all files for the new feature under `src/Components/<Domain>/`. Follow the existing pattern:

```
src/Components/Billing/
├── Dashboard/
│   └── Dashboard.tsx
├── Create/
│   └── CreateInvoice.tsx
├── Edit/
│   └── EditInvoice.tsx
├── Form/
│   └── InvoiceForm.tsx
└── model/
    └── invoice.ts
```

### 2. Define the model

In `model/invoice.ts`, declare the TypeScript interface and the `Schema` mapping:

```typescript
import type { Schema } from '../../../../config/data/mapper/mapper';

export interface Invoice {
  id: string;
  amount: number;
  createdAt: string;
}

export const InvoiceSchema: Schema = {
  id: 'id',
  amount: 'amount',
  createdAt: 'created_at',
};
```

### 3. Build the Dashboard component

Use `useResourceRepository` for fetching, `useHttpClient` for mutations, and `useSnackBar` for feedback. The `TableView` shared component handles columns, filters, and pagination in one place.

### 4. Register the routes

Open `src/config/routing/routes.tsx` and add entries. Wrap each element in `AuthGuard` with the appropriate `accessScopes`:

```typescript
import BillingDashboard from '../../Components/Billing/Dashboard/Dashboard';
import CreateInvoice from '../../Components/Billing/Create/CreateInvoice';
import { AppScopes } from '../security/scopes';

// Inside the routes array:
{
  path: '/billing',
  element: (
    <AuthGuard accessScopes={[AppScopes.AdminUsersRead]}>
      <BillingDashboard />
    </AuthGuard>
  ),
},
{
  path: '/billing/create',
  element: (
    <AuthGuard accessScopes={[AppScopes.AdminUsersWrite]}>
      <CreateInvoice />
    </AuthGuard>
  ),
},
```

### 5. Add a menu entry

Open `src/config/menu/menu.ts` and append to `defaultMenuItems`:

```typescript
{
  name: 'Billing', children: [
    { name: 'Invoices', href: '/billing', accessScopes: [AppScopes.AdminUsersRead] },
    { name: 'New Invoice', href: '/billing/create', accessScopes: [AppScopes.AdminUsersWrite] },
  ]
}
```

Menu items with `accessScopes` are filtered by the `MenuProvider` before rendering, so users without the required scope will not see the link.

### 6. Add a new scope constant (if needed)

If the feature requires scopes that do not yet exist in `config/security/scopes.ts`, add them to the `AppScopes` object:

```typescript
AdminBillingRead: 'admin:billing:read',
AdminBillingWrite: 'admin:billing:write',
```

The `as const` assertion on the object means TypeScript will infer the literal string types automatically. No further type declarations are needed.
