# Backend Structure

The Go backend lives entirely under the `api/` directory. This document describes every major package, the key architectural patterns, and the steps required to add a new entity and endpoint.

---

## Top-level package map

```
api/
├── main.go                 # Entry point; dispatches start / shutdown / create-admin
├── go.mod / go.sum         # Module definition and checksums
├── config/                 # Static configuration: DI, DB, routes, embeds
├── src/                    # Domain logic: users, groups, ACL, auth
├── lift/                   # HTTP infrastructure: security, REST dispatch, URI parsing
├── chop/                   # ML / tensor utilities (not part of IAM serving path)
└── vendor/                 # Vendored dependencies
```

---

## `config/` — application bootstrap

### `config/di/di.go` — dependency injection container

`ContextBag` is the central service locator used throughout the application. It is created once in `main.go` and passed to the route initialiser.

```go
type ContextBag struct {
    items              map[string]interface{}
    DatabaseManager    *orm.DatabaseManager
    Logger             logger.Logger
    ApiResourceHandler *lift_api.ApiResourceHandler
}
```

The struct implements the `serverdi.Context` interface defined in `coco-server`. Any handler that needs the database or logger retrieves it from the `RequestContext` passed at request time:

```go
manager := reqCtx.GetDI().GetDatabaseManager()
log     := reqCtx.GetDI().GetLogger()
```

Arbitrary values can be stored and retrieved via the generic `Set(key, value)` / `Get(key)` methods. Strongly typed fields (`DatabaseManager`, `Logger`, `ApiResourceHandler`) should be accessed through their dedicated accessor methods.

### `config/db/` — database initialisation and migrations

| File | Purpose |
|------|---------|
| `install.go` | `Install(manager)` runs `SyncMigrations`; `EnsureHasSuperadmin` checks for at least one active superadmin; `AddSuperadminWithArgs` creates a superadmin from CLI arguments |
| `sql.go` | `ExecuteSQLFile` reads a `.sql` file, splits on the `/***Statement***/` delimiter, and executes each statement against a `*sql.DB` |
| `migrations/` | Embedded SQL migration files named by timestamp (e.g. `15_02_2026_12_21_22.sql`) |

The actual migration tracking is handled by `coco-orm`'s `DatabaseManager.SyncMigrations()`, which maintains a `migrations` table and skips files that have already been applied.

### `config/routes/` — route registration

| File | Purpose |
|------|---------|
| `routes.go` | `Init(ctx)` registers all handlers in a name-to-handler map and calls the YAML route loader |
| `routes.yaml` | Declarative route tree with paths, HTTP methods, executor names, security levels, and required scopes |

Routes are loaded at startup and never re-read at runtime. The YAML parser supports nested `children` arrays and inherits `security` and `scopes` values from parent nodes to child nodes.

### `config/resource/entities_api_resources.go` — entity registry

`GetApiResourceHandler()` returns a singleton `ApiResourceHandler` with an `EntityMap` that maps URL resource names to their Go entity types and optional custom handlers:

```go
"users":              { Entity: &entity.User{},           CustomHandlers: { POST: ..., PATCH: ..., DELETE: ... } }
"admin_groups":       { Entity: &admin_groups_entity.AdminGroup{} }
"user_acl":           { Entity: &acl_entity.UserAcl{} }
"user_group_acl":     { Entity: &acl_entity.UserGroupAcl{} }
"user_group_members": { Entity: &admin_groups_entity.AdminGroupMember{} }
```

When a resource name has no custom handler for an HTTP method, the generic ORM-backed handler for that method is used automatically.

### `config/embed.go` — static file embedding

All configuration assets are compiled into the binary via `go:embed`:

```go
//go:embed routes/** db/migrations/** config.json scopes/**
var ConfigFS embed.FS
```

Helper functions (`ReadConfigFile`, `ListConfigFiles`, `ExtractMigrationsToTemp`) provide access to the embedded filesystem. Migration files are extracted to a temporary directory on startup because `coco-orm` expects a filesystem path.

---

## `src/` — domain logic

### `src/admin/users/` — admin user management

| Path | Purpose |
|------|---------|
| `entity/admin.go` | `User` struct (maps to `admin_users` table); `UserLogin` and `Count` read-only projections |
| `repository/query/user_repository.go` | Read-only queries: find by username, email, pagination, superadmin check, `/me` group and ACL queries |
| `repository/persistent/user_repository.go` | Write operations: `Insert` using coco-orm's `InsertObjectQueryBuilder` |
| `create.go` | `AdminUserCreator` — creates a user and its bcrypt password record atomically; also exposes `CustomCreateUserHandler` for HTTP POST |
| `update.go` | `CustomUpdateUserHandler` for HTTP PATCH |
| `delete.go` | `CustomDeleteUserHandler` for HTTP DELETE |
| `validator/user.go` | `VerifySuperAdminPrivilege` — checks whether the requesting token carries `super:admin` scope |
| `authentication/login.go` | `DatabaseAuthenticationLogin` — verifies credentials, resolves scopes, issues a JWT |
| `me/me_groups.go` | `MeGroupsHandler` — returns the authenticated user's group memberships |
| `me/me_acl.go` | `MeAclHandler` — returns direct and inherited ACL scopes for the authenticated user |

### `src/admin/groups/` — group management

| Path | Purpose |
|------|---------|
| `entity/admin_group.go` | `AdminGroup` (maps to `user_groups`) and `AdminGroupMember` (maps to `user_group_members`) with many-to-one relations to users and groups |

Groups have no custom handlers; all CRUD operations go through the generic `ApiResourceHandler`.

### `src/admin/acl/` — access control lists

| Path | Purpose |
|------|---------|
| `entity/acl.go` | `UserAcl` and `UserGroupAcl` structs; roles are stored as a JSON array |
| `repository/user_acl.go` | `FindUserScopes` — resolves the full set of scopes for a user by combining the `is_super_admin` flag, direct `user_acl` entries, and scopes inherited through group membership |
| `scopes_handler.go` | `AclScopesHandler` — HTTP handler that returns the list of available scope strings |

### `src/auth/` — authentication infrastructure

| Path | Purpose |
|------|---------|
| `crypto/bcrypt/bcrypt.go` | `HashPassword` and `VerifyPassword` wrappers around `golang.org/x/crypto/bcrypt` |
| `database/entity/user.go` | `UserLogin` interface and `LoginCredentials` request struct |
| `database/repository/query/` | `PasswordQueryRepository` — retrieves the stored bcrypt hash for a user ID |
| `database/repository/persistent/` | `PasswordPersistentRepository` — inserts a new password record |
| `database/authenticate.go` | `PasswordAuthenticator` — fetches the stored hash and calls `VerifyPassword` |
| `oauth/oauth.go` | `IssueToken` — signs a HS256 JWT with a 30-minute TTL using the secret from `config.json` |
| `oauth/renew/renew.go` | `TokenRenewHandler` — validates an existing token and issues a new one |
| `oauth/server_middleware.go` | `WrapServerHandler` — wraps a handler with OAuth token validation |

### `src/xbrl/standard/` — XBRL handling

Handles XBRL standard import and dashboard functionality. This is a domain-specific module separate from the core IAM path.

---

## `lift/` — HTTP infrastructure

### `lift/security/` — scope-based authorisation middleware

`ScopeSecurityLayer` implements the `security.SecurityLayer` interface required by `coco-server`. It is created once at startup and installed as the global security layer for all routes.

For each incoming request it:
1. Looks up the matching route definition in the flattened route list (built from `routes.yaml`).
2. Skips validation entirely for routes marked `security: public`.
3. Extracts the `Authorization: Bearer <token>` header.
4. Validates the token using the HS256 secret from `config.json`.
5. Checks that at least one token scope matches the required scopes defined in the route. The special scope `admin:me` bypasses the scope check (any authenticated user can access their own data).

A 401 is returned for missing or invalid tokens; a 403 is returned when the token is valid but lacks the required scope.

### `lift/resource/` — generic REST dispatch

`ApiResourceHandler` is the central request dispatcher for all resource-based endpoints. It:
1. Extracts the resource name from the `{res:<name>}` segment in the URL path.
2. Looks up the entity and event listeners registered for that resource in `EntityMap`.
3. Calls any registered `BeforeExecution` listeners.
4. If the HTTP method has a custom handler registered, delegates to it.
5. Otherwise, dispatches to the appropriate typed handler: `GetResourceHandler`, `GetCollectionResourceHandler`, `PostResourceHandler`, `PutResourceHandler`, `PatchResourceHandler`, or `DeleteResourceHandler`.
6. Calls any registered `AfterExecution` listeners.

Each method handler in `lift/resource/rest/` uses `coco-orm` to perform the actual database operation against the entity type registered for the resource.

### `lift/resource/uri/` — URI parsing

| Function | Purpose |
|----------|---------|
| `ExtractResourceTypeFromURI(path)` | Finds and returns the value of the `{res:<name>}` segment; returns empty string if not exactly one such segment exists |
| `ExtractKeyAndValueFromURI(path)` | Returns the key and value from a `{key:value}` segment (used to detect single-resource vs. collection requests) |
| `ExtractURIParams(path, template)` | Matches a concrete path against a template and returns named parameters as a map |

### `lift/routes/` — YAML route loading and expansion

| File | Purpose |
|------|---------|
| `yaml_parser.go` | `LoadRoutesYAML` — reads all `route*.yaml` files from the embedded `routes/` directory, merges them into a single YAML document, and calls `ProcessApiResourceHandlerRoutesRecursive` to expand any route entry with `executor: ApiResourceHandler` into one entry per HTTP method |
| `routes_builder.go` | `WrapHandlersWithProtection` — wraps authenticated handlers with OAuth middleware; helper functions for parsing security and executor values from YAML nodes |

---

## Key architectural patterns

### ContextBag — dependency injection

There is no global state for the database connection or logger. All handlers receive a `request.RequestContext`, which exposes `GetDI()` returning the `ContextBag`. Services are retrieved from the bag rather than imported as package-level variables. This makes it straightforward to replace dependencies in tests.

### YAML-driven route loading

Routes are declared in `api/config/routes/routes.yaml` (and any additional `route*.yaml` files placed in the same embedded directory). The YAML is parsed at startup into a flat list of `RouteConfig` objects. Each object carries the full path, HTTP method, handler name, security level, and required scopes. No route is hard-coded in Go.

When a route entry specifies `executor: ApiResourceHandler` without an explicit `method`, the YAML processor expands it into five entries — one for each of GET, POST, PUT, PATCH, and DELETE. Individual methods can be excluded with the `exclude_method` list.

### ApiResourceHandler interface

Any domain entity that needs standard CRUD endpoints does not require a dedicated handler file. Register the entity in `config/resource/entities_api_resources.go` and add the corresponding path entries to `routes.yaml`. The `ApiResourceHandler` will reflect on the entity's struct tags to build queries via coco-orm.

Custom behaviour for a specific method (e.g., user creation which also writes a password record) is implemented by registering a `CustomHandlers` entry for that method in the `ResourceConfig`. Custom handlers receive the full `RequestContext` and can use any service from the DI container.

### Repository split: query vs persistent

Write operations (INSERT, UPDATE, DELETE) are in `repository/persistent/`. Read operations (SELECT) are in `repository/query/`. This separation keeps the read path free of side effects and makes it easy to locate all mutation points for a given entity.

---

## Adding a new entity and endpoint

The following steps are required to add a fully functional CRUD endpoint for a new entity, for example `widget`.

**1. Define the entity struct**

Create `api/src/admin/widgets/entity/widget.go`:

```go
package entity

type Widget struct {
    _         struct{} `table:"widgets"`
    ID        string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
    Name      string   `db:"name" dbtype:"TEXT" nullable:"false" json:"name"`
    CreatedAt string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
    IsActive  bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
```

The `table` struct tag tells coco-orm which table to use. The `db` and `dbtype` tags define the column name and SQL type.

**2. Write the migration**

Create a new SQL file in `api/config/db/migrations/` with a timestamp-based name, for example `19_04_2026_10_00_00.sql`:

```sql
/***Statement***/
CREATE TABLE widgets (
    id TEXT NOT NULL PRIMARY KEY UNIQUE,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
);
/***Statement***/
CREATE INDEX widgets_id_index ON widgets (id);
```

The migration will be applied automatically on the next server startup.

**3. Register the entity**

Add a new entry to the `EntityMap` in `api/config/resource/entities_api_resources.go`:

```go
"widgets": {
    Entity: func() interface{} { return &widget_entity.Widget{} },
},
```

Import the entity package at the top of the file.

**4. Add routes to routes.yaml**

Add entries to `api/config/routes/routes.yaml` under the appropriate parent path:

```yaml
- path: /{res:widgets}
  executor: ApiResourceHandler
  security: authenticated
  scopes:
    - admin:widgets:read
- path: /{res:widgets}/{id}
  method: GET
  executor: ApiResourceHandler
  security: authenticated
  scopes:
    - admin:widgets:read
```

Routes with `executor: ApiResourceHandler` and no `method` are automatically expanded to all five HTTP methods by the YAML processor.

**5. Add custom handlers (optional)**

If any operation requires logic beyond a direct ORM insert or update — for example hashing a field or validating a relationship — register a custom handler function in the `ResourceConfig`:

```go
"widgets": {
    Entity: func() interface{} { return &widget_entity.Widget{} },
    CustomHandlers: map[string]func(reqCtx request.RequestContext){
        http.MethodPost: widgets.CustomCreateWidgetHandler,
    },
},
```

Implement `CustomCreateWidgetHandler` in `api/src/admin/widgets/create.go`, following the same pattern as `CustomCreateUserHandler` in `src/admin/users/create.go`.

**6. Rebuild**

```sh
make build
```

The new migration, routes, and entity are embedded into the binary at compile time.
