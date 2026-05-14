# Scope-Based ACL Management Implementation Plan

## Overview
This document outlines a step-by-step plan for implementing a scope-based Access Control List (ACL) management system within `api/src/admin/acl`. As suggested, the system will use a hard-coded JSON file as the source of truth for all available scopes.

## Phase 1: Define the Scope Schema (JSON)
We will leverage a JSON file to define all available scopes in the system.

- **Action**: Create `scopes.json` in `api/src/admin/acl` (or a dedicated `config` folder).
- **Structure Example**:
  ```json
  {
    "admin": {
      "description": "Administrator scope",
      "scopes": [
        {
          "id": "admin:users:read",
          "description": "Allows reading user details."
        },
        {
          "id": "admin:users:write",
          "description": "Allows creating or modifying users."
        },
        {
          "id": "admin:acl:manage",
          "description": "Allows managing the ACLs and roles."
        }
      ]
    }
  }
  ```
- **Rationale**: This guarantees version control over scope definitions and simplifies reading them programmatically. It organizes scopes logically by domain (e.g., `admin`).

## Phase 2: Loading and Parsing Scopes in Go
The application needs to load these scopes into memory upon startup for fast validation.

- **Action**: Define Go `structs` that map precisely to the JSON structure.
- **Action**: Create a loader component (e.g., `ScopeRegistry`) that reads `scopes.json` during the service initialization phase (`init()` or app bootstrap).
- **Action**: Implement internal methods like `IsValidScope(scope string) bool` to prevent the assignment of non-existent scopes.

## Phase 3: Implement ACL Middleware
To actually enforce the scopes, we need a middleware that sits in front of the API endpoints.

- **Action**: Create a middleware function (e.g., `RequireScope(requiredScope string) func(http.Handler) http.Handler`).
- **Flow**:
  1. The middleware extracts the user's currently granted scopes from their context (e.g., from a parsed JWT token or session).
  2. It checks if the user possesses the `requiredScope`.
  3. If they don't, the middleware halts the request and returns an `HTTP 403 Forbidden`.
  4. If they do, it passes execution to the next handler. 

## Phase 4: Integration
Once the middleware is built, we apply it to existing resources.

- **Action**: Review routes in `api/src/admin/users` and other admin resources.
- **Action**: Wrap these route handlers with the `RequireScope("specific:scope")` middleware.

## Phase 5: Testing
- **Action**: Write unit tests for the `ScopeRegistry` to ensure parsing logic acts appropriately on startup (handling valid and invalid JSON).
- **Action**: Write isolated tests for the ACL middleware with mock user contexts to verify 403 and 200 responses.

Let me know if you would like to proceed with implementing any of these phases!
