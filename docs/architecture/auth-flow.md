# Authentication Flow

This document describes the complete authentication lifecycle in coco-iam: how users log in, how JWTs are issued and validated, how tokens are renewed, and how logout works on both the backend and the frontend.

---

## Login Flow (Backend)

Login is handled by `DatabaseAuthenticationLogin` in `api/src/admin/users/authentication/login.go`.

**Endpoint:** `POST /api/v1/admin/oauth/authenticate`  
**Security:** `public` (no token required)

**Request body:**
```json
{
  "username": "string",
  "password": "string"
}
```

**Steps:**

1. The request body is decoded into a `LoginCredentials` struct. Missing or extra fields result in `400 Bad Request`.
2. The `AdminUserQueryRepository` looks up the user by username in the `admin_users` table.
3. If no user is found or the user's `is_active` flag is `false`, the handler returns `401 Unauthorized` with the message `"invalid credentials"`. The exact reason (not found vs. inactive) is intentionally not disclosed.
4. The `PasswordAuthenticator` fetches the stored bcrypt hash from the `user_auth_password` table and calls `bcrypt.CompareHashAndPassword`. A mismatch returns `401 Unauthorized`.
5. The `UserAclRepository.FindUserScopes` method collects the user's effective scopes from three sources:
   - The `admin_users.is_super_admin` flag — if true, the `super:admin` scope is added.
   - Direct assignments in `user_acl.roles` where `user_id = <id>` and `is_active = true`.
   - Inherited assignments from any groups the user belongs to via `user_group_members`, joined with `user_group_acl.roles`.
   All scopes from all sources are deduplicated into a flat list.
6. If the resulting scope list is empty, the handler returns `403 Forbidden` with a message directing the user to contact their administrator.
7. `IssueToken` is called with the user ID and the final scope list. It signs a HS256 JWT using the secret and issuer/audience values from `config.json`, with a TTL of 30 minutes.
8. The response is `200 OK` with a `TokenResponse` body.

**Success response:**
```json
{
  "access_token": "<JWT>",
  "token_type": "Bearer",
  "expires_at": 1713600000,
  "user": {
    "id": "<user-uuid>"
  }
}
```

---

## JWT Token Structure

Tokens are signed with HS256. The payload contains the following claims:

| Claim   | Type             | Description                                        |
|---------|------------------|----------------------------------------------------|
| `sub`   | string           | User ID (UUID)                                     |
| `iss`   | string           | Issuer — configured in `config.json`               |
| `aud`   | string           | Audience — configured in `config.json`             |
| `scope` | string           | Space-separated list of scopes, e.g. `"admin:users:read admin:groups:read"` |
| `exp`   | Unix timestamp   | Expiry time (30 minutes from issue time)           |

The `scope` claim is a single space-separated string following the OAuth2 convention. The frontend parses it by splitting on spaces to produce a `scopes` array (see `app/src/config/security/jtw.ts`).

---

## Request Authorization Flow (Middleware)

Every request to a protected route passes through `ScopeSecurityLayer` in `api/lift/security/scope_based_auth_layer.go` before reaching a handler.

**Steps:**

1. The security layer looks up the matched route in the flattened route map built from `routes.yaml`.
2. If the route's `security` field is `"public"`, the request is allowed through immediately.
3. If the route is `"authenticated"` but has no required scopes, a valid token is sufficient — scope content is not checked.
4. The `Authorization` header is read. If absent, `401` is returned with `{"message": "Access denied"}`.
5. `oauth.ExtractBearer` parses the header. A malformed header returns `401`.
6. `Validator.Validate` verifies the HS256 signature and expiry, and extracts `sub` and `scopes`. An invalid or expired token returns `401`.
7. If the route declares required scopes, the middleware checks whether any of the user's scopes match any of the required scopes (OR logic). If none match, `403` is returned.
8. Exception: if `admin:me` appears in the required scopes list, the request is always allowed through for authenticated users regardless of other scope checks.
9. On success, the request is dispatched to the route handler.

---

## Token Renewal Flow

Renewal is handled by `TokenRenewHandler` in `api/src/auth/oauth/renew/renew.go`.

**Endpoint:** `POST /api/v1/admin/oauth/renew`  
**Security:** `public` (no middleware enforcement — token validation happens inside the handler)

**Request body:**
```json
{
  "refresh_token": "<current JWT>"
}
```

**Steps:**

1. The handler decodes the request body. A missing `refresh_token` returns `400`.
2. The current token is validated with the same `Validator.Validate` logic used by the middleware, extracting `sub`, `scopes`, and `expiry`.
3. If the token is invalid or the subject is empty, `401` is returned.
4. The handler applies a grace window: if the token's `expiry + 15 minutes` is before the current time, the token is too old to renew and `401` is returned with the message `"refresh token expired"`.
5. A new token is issued for the same `sub` and `scopes` using `IssueToken`, producing a fresh 30-minute JWT.
6. The response is identical in structure to a login response.

**Note:** coco-iam uses a single token for both access and renewal. There is no separate refresh token. The renewal endpoint accepts the same JWT that is used for API calls, provided it has not exceeded the 15-minute grace window beyond its expiry.

---

## Frontend Auth Flow

The frontend auth lifecycle is managed by `AuthProvider` (`app/src/Components/Auth/Guard/AuthContext.tsx`), which wraps the application and exposes auth state via React Context.

### Token Storage

- On successful login, the `TokenResponse` object is serialized and stored in `localStorage` under the key `AUTH_TOKEN_KEY`.
- On application load, `AuthProvider` initializes `authenticated` to `true` if a value exists in `localStorage`, and reads the token back with `findAuthToken`.

### HTTP Client Header Injection

`app/src/api/client.ts` provides the core fetch wrapper. Before every authenticated request, `buildHeaders` is called:

1. `findAuthToken()` reads the token from `localStorage`.
2. The token's `expires_at` (Unix timestamp) is compared to the current time.
3. If the token has expired, `renewToken` is called, which posts to `/api/v1/admin/oauth/renew`. On success, `saveAuthToken` writes the new token back to `localStorage`. On failure, `logoutUser` is called and an error is thrown.
4. The valid token is injected as `Authorization: Bearer <access_token>`.

### Route Protection with AuthGuard

`AuthGuard` (`app/src/Components/Auth/Guard/AuthGuard.tsx`) wraps protected routes in the React Router configuration.

- If `authenticated` is `false` or `authToken` is absent, the user is redirected to `/login`, preserving the attempted location in router state.
- If the route declares an `accessScopes` prop, the JWT is parsed client-side with `parseJwt` and the user's scopes array is extracted.
- A user passes the scope check if any of the following are true:
  - They hold the `super:admin` scope.
  - The route's `accessScopes` includes `user:me`.
  - The user's scopes contain at least one of the route's `accessScopes`.
- If none of these conditions are met, the user is redirected to `/`.

### Logout Flow

1. The user navigates to the logout route or calls `AuthContext.logout()`.
2. `logout()` sets `authenticated = false`, clears `authToken` from state, and removes the entry from `localStorage`.
3. The `Logout` component (`app/src/Components/Auth/Logout/Logout.tsx`) calls `logout()` on mount and then navigates to `/login` with `{ replace: true }`.

---

## Auth Failure Behavior

| Condition                          | Backend response        | Frontend behavior                         |
|------------------------------------|-------------------------|-------------------------------------------|
| Missing `Authorization` header     | `401 {"message":"Access denied"}` | HTTP client throws error; session ended if renewal also fails |
| Invalid or malformed JWT           | `401`                   | Same as above                             |
| Expired JWT, renewal succeeds      | —                       | Client renews silently and retries        |
| Expired JWT, renewal fails         | `401` on renewal        | `logoutUser()` called; user redirected to `/login` |
| Valid token, insufficient scopes   | `403 {"message":"Access denied"}` | Error surfaced to calling component      |
| Not authenticated (frontend)       | —                       | `AuthGuard` redirects to `/login`         |
| Authenticated but wrong scopes (frontend) | —                | `AuthGuard` redirects to `/`              |

---

## Related Documentation

- [Architecture Overview](./overview.md)
- [Scope System](./scope-system.md)
