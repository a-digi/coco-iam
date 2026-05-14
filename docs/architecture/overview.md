# coco-iam — Architecture Overview

## What is coco-iam?

coco-iam is a full-stack Identity and Access Management (IAM) system. It provides centralized user authentication, authorization, and access control for administrative interfaces and downstream services. The system manages users, user groups, and fine-grained permission scopes that are enforced on every protected API route.

The backend exposes a REST API secured with JWT Bearer tokens. The frontend is a single-page React application that consumes this API and renders UI conditionally based on the authenticated user's scopes.

---

## Tech Stack

| Layer      | Technology                                   |
|------------|----------------------------------------------|
| Backend    | Go 1.26                                      |
| Frontend   | React 19, TypeScript 5.9, Vite               |
| Styling    | Tailwind CSS 4                               |
| Routing    | React Router 7                               |
| Database   | SQLite (`./data/db/users.db`)                |
| Auth       | bcrypt (password hashing), JWT (HS256)       |

---

## Custom Microlibraries

coco-iam is built on a set of internal microlibraries vendored under `plugins/` and referenced via Go module paths under `github.com/a-digi/`.

| Library         | Version | Role                                                                                   |
|-----------------|---------|----------------------------------------------------------------------------------------|
| `coco-orm`      | v0.1.5  | SQLite ORM: schema migrations, query builder, hydration, M2M/M2O relations             |
| `coco-server`   | v0.1.8  | HTTP server lifecycle, route registration, middleware pipeline, request/response helpers |
| `coco-oauth`    | v0.1.0  | JWT signing (HS256), token validation, scope extraction, Bearer header parsing          |
| `coco-logger`   | v0.1.0  | Structured file-based logger with daily rotation                                        |
| `coco-filer`    | v0.1.0  | File system utilities used for configuration and asset management                       |

---

## Component Diagram

```
+---------------------------+
|   Browser (React SPA)     |
|  - AuthContext            |
|  - AuthGuard              |
|  - ScopeBasedComponentAccess |
|  - HTTP client (Bearer)   |
+------------+--------------+
             |
             | REST API (HTTP/JSON)
             | Authorization: Bearer <JWT>
             |
+------------+--------------+
|  Go HTTP Server           |
|  port 2026                |
|                           |
|  coco-server              |
|   - Route builder         |
|   - Request/response      |
|                           |
|  ScopeSecurityLayer       |
|   - JWT validation        |
|   - Scope enforcement     |
|                           |
|  Route handlers           |
|   - Login / Renew         |
|   - Users, Groups, ACL    |
+------------+--------------+
             |
             | coco-orm
             |
+------------+--------------+
|  SQLite                   |
|  ./data/db/users.db       |
|                           |
|  Tables:                  |
|   admin_users             |
|   user_auth_password      |
|   user_acl                |
|   admin_groups            |
|   user_group_members      |
|   user_group_acl          |
+---------------------------+
```

---

## Key Runtime Details

| Detail            | Value                                          |
|-------------------|------------------------------------------------|
| API port          | `2026`                                         |
| Frontend dev port | `5173`                                         |
| Database path     | `./data/db/users.db`                           |
| Log path pattern  | `./data/logs/{YYYY}/{MM}/{DD}/server_*.log`    |
| PID file          | `./server.pid`                                 |
| JWT algorithm     | HS256                                          |
| JWT TTL           | 30 minutes                                     |
| Renewal window    | Up to 15 minutes after expiry                  |

Logs are written by `coco-logger` with filenames in the format `server_YYYYMMDD_HHMMSS.log`, organized under a daily directory path.

---

## Route Configuration

All API routes are declared in `api/config/routes/routes.yaml`. Each route entry specifies:

- `path` — the URI path segment
- `method` — HTTP verb
- `executor` — the Go handler name
- `security` — `public` or `authenticated`
- `scopes` — list of scopes required (OR logic)

`ScopeSecurityLayer` reads this file at startup, flattens the route tree, and enforces security on every incoming request before dispatching to a handler.

---

## Related Documentation

- [Authentication Flow](./auth-flow.md) — Login, JWT issuance, token renewal, logout, and frontend auth lifecycle
- [Scope System](./scope-system.md) — Full scope catalog, database storage, route declarations, and frontend enforcement
