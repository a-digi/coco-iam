# Authentication API

Base URL: `http://localhost:2026/api/v1/admin`

This document covers the three endpoints that handle authentication and token lifecycle. Two of them (`/oauth/authenticate` and `/oauth/renew`) are public and do not require a prior token. The scopes listing endpoint requires a valid token with the appropriate scope.

---

## Authenticate

Validates a username/password pair against the database, collects the user's effective ACL scopes (direct assignments plus any scopes inherited through group memberships), and issues a signed JWT.

Authentication is refused if the user account is inactive, if no password record exists, or if the account has no ACL scopes assigned. Accounts with no scopes are considered unprovisionned — the system returns `403` rather than `401` in that case to make the distinction clear.

```
POST /api/v1/admin/oauth/authenticate
```

**Auth required:** No

### Request

| Field      | Type   | Required | Description                            |
|------------|--------|----------|----------------------------------------|
| `username` | string | Yes      | The account's username                 |
| `email`    | string | No       | Accepted in the body but not used for lookup; lookup is by username |
| `password` | string | Yes      | Plaintext password (compared via bcrypt) |

```json
{
  "username": "alice",
  "password": "s3cr3tpassw0rd"
}
```

### Response — 200 OK

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 1745123456,
  "scope": "admin:users:read admin:groups:read"
}
```

| Field          | Type   | Description                                                       |
|----------------|--------|-------------------------------------------------------------------|
| `access_token` | string | Signed HS256 JWT                                                  |
| `token_type`   | string | Always `"Bearer"`                                                 |
| `expires_in`   | number | Unix timestamp of expiry (default TTL is 30 minutes from issue)   |
| `scope`        | string | Space-separated list of all effective scopes encoded in the token |

### Error Responses

| Status | Condition                                                                 |
|--------|---------------------------------------------------------------------------|
| 400    | Missing or malformed JSON body, or `username`/`password` field is empty   |
| 401    | User not found, account is inactive, or password does not match           |
| 403    | Credentials are valid but no ACL scopes have been assigned to the account |
| 500    | Database error, token signing failure, or configuration error             |

---

## Renew Token

Issues a fresh JWT by validating an existing token supplied in the request body. The token does not need to be passed in the `Authorization` header for this endpoint.

The endpoint accepts any non-expired token. A 15-minute grace window is applied: a token whose recorded expiry falls within the past 15 minutes is still accepted. Tokens expired beyond that window are rejected.

```
POST /api/v1/admin/oauth/renew
```

**Auth required:** No

### Request

| Field           | Type   | Required | Description                                            |
|-----------------|--------|----------|--------------------------------------------------------|
| `refresh_token` | string | Yes      | An existing JWT previously issued by this service      |

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Response — 200 OK

The response shape is identical to the authenticate endpoint.

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 1745127056,
  "scope": "admin:users:read admin:groups:read"
}
```

The new token carries the same `sub` (user ID) and `scopes` as the original token. Scopes are not re-fetched from the database at renewal time; the scopes embedded in the existing token are reused.

### Error Responses

| Status | Condition                                                              |
|--------|------------------------------------------------------------------------|
| 400    | Missing or malformed JSON body, or `refresh_token` field is empty      |
| 401    | Token signature is invalid, token is not parseable, or token is expired beyond the 15-minute grace window |
| 500    | Configuration error or token signing failure                           |

---

## List Available Scopes

Returns the full catalog of scope strings recognised by the system. This endpoint reads the server-side `scopes/admin.json` configuration file and returns it verbatim. Use this to discover which scope identifiers are valid when constructing ACL entries.

```
GET /api/v1/admin/acl/scopes
```

**Auth required:** Yes
**Required scope:** `admin:acl:read` or `admin:acl`

### Request

No request body. Pass a valid JWT in the `Authorization` header.

```
Authorization: Bearer {access_token}
```

### Response — 200 OK

An array of scope group objects. Each top-level object has an `id`, a `description`, and a nested `scopes` array that may itself contain further nested scope objects.

```json
[
  {
    "id": "admin",
    "description": "Administrator scope",
    "scopes": [
      {
        "id": "admin:users",
        "description": "Allows creating or modifying users.",
        "scopes": [
          {
            "id": "admin:users:read",
            "description": "Allows reading users."
          },
          {
            "id": "admin:users:write",
            "description": "Allows creating or modifying users."
          },
          {
            "id": "admin:users:delete",
            "description": "Allows deleting users."
          },
          {
            "id": "admin:users:manage",
            "description": "Allows managing users."
          }
        ]
      }
    ]
  }
]
```

The response is the raw contents of the scope configuration file; the depth and shape of nesting can vary. Callers should traverse the tree recursively to collect all leaf `id` values.

### Error Responses

| Status | Condition                                                        |
|--------|------------------------------------------------------------------|
| 401    | Missing, malformed, or expired `Authorization` token            |
| 403    | Token does not contain `admin:acl:read` or `admin:acl` scope    |
| 500    | Scope configuration file cannot be read or parsed               |

---

## JWT Token Structure

Tokens issued by this service are HS256-signed JWTs. The payload carries the following claims.

| Claim   | Type   | Description                                                      |
|---------|--------|------------------------------------------------------------------|
| `sub`   | string | UUID of the authenticated user (maps to `admin_users.id`)        |
| `iss`   | string | Issuer identifier, configured server-side                        |
| `aud`   | string | Audience identifier, configured server-side                      |
| `exp`   | int64  | Unix timestamp of token expiry                                   |
| `scope` | string | Space-separated string of all effective scopes                   |

The `scope` claim is split on spaces at parse time to produce a `[]string` slice used for scope checks. There is no separate `refresh_token` concept; the access token itself is passed to `/oauth/renew`.
