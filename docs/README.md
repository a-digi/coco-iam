# coco-iam Documentation

coco-iam is a full-stack Identity and Access Management system. It provides user management, group management, and fine-grained ACL (Access Control List) enforcement via a scope-based permission model. The backend is a Go HTTP server backed by SQLite; the frontend is a React + TypeScript SPA.

---

## Quick start

→ [setup/getting-started.md](setup/getting-started.md)

---

## Contents

### Architecture
| File | Description |
|------|-------------|
| [architecture/overview.md](architecture/overview.md) | System overview, tech stack, component map, runtime details |
| [architecture/auth-flow.md](architecture/auth-flow.md) | End-to-end authentication flow: login → JWT → request authorization |
| [architecture/scope-system.md](architecture/scope-system.md) | How scopes are defined, stored, and enforced across the stack |

### API Reference
| File | Description |
|------|-------------|
| [api/authentication.md](api/authentication.md) | OAuth authenticate, renew, and scope listing endpoints |
| [api/users.md](api/users.md) | User CRUD endpoints and self-service /me endpoints |
| [api/groups.md](api/groups.md) | Group and group member endpoints |
| [api/acl.md](api/acl.md) | User ACL and group ACL endpoints |
| [api/workspaces.md](api/workspaces.md) | Workspace CRUD, workspace↔organization memberships, and organizations read-only endpoints |

### Setup & Workflow
| File | Description |
|------|-------------|
| [setup/getting-started.md](setup/getting-started.md) | Prerequisites, first build, configuration, creating the admin user |
| [setup/makefile-reference.md](setup/makefile-reference.md) | All Make targets and binary CLI commands |

### Backend
| File | Description |
|------|-------------|
| [backend/structure.md](backend/structure.md) | Go package layout, key patterns, how to add a new endpoint |
| [backend/database.md](backend/database.md) | Schema reference, migration system, coco-orm usage |

### Frontend
| File | Description |
|------|-------------|
| [frontend/structure.md](frontend/structure.md) | Component tree, context providers, HTTP client, repository pattern |
| [frontend/routing.md](frontend/routing.md) | Route table, AuthGuard, ScopeBasedComponentAccess, AppScopes reference |

### Observability
| File | Description |
|------|-------------|
| [observability/README.md](observability/README.md) | coco-observe: architecture, build, agent management, metrics reference, API reference |
