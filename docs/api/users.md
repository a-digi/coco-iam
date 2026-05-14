# Users API

Base URL: `http://localhost:2026/api/v1/admin`

All endpoints on this page require a valid JWT in the `Authorization` header unless noted otherwise.

```
Authorization: Bearer {access_token}
```

---

## User Entity

Users are stored in the `admin_users` table. The following fields are returned in all user responses.

| Field          | JSON key        | Type    | Nullable | Description                                                      |
|----------------|-----------------|---------|----------|------------------------------------------------------------------|
| ID             | `id`            | string  | Yes      | UUID, generated on insert                                        |
| Username       | `username`      | string  | No       | Unique login name. Cannot be changed after creation.             |
| Email          | `email`         | string  | No       | Email address                                                    |
| CreatedAt      | `created_at`    | string  | Yes      | ISO 8601 / DATETIME string, set at insert time                   |
| IsSuperAdmin   | `is_super_admin`| bool    | No       | When `true`, the account holds the `super:admin` scope automatically |
| IsActive       | `active`        | bool    | No       | Inactive accounts cannot authenticate. Defaults to `true`.       |

Password hashes are never returned in any response. They live in the `user_auth_password` table and are managed internally.

---

## List Users

Returns a paginated collection of all user accounts.

```
GET /api/v1/admin/users
```

**Required scope:** `admin:users:read`

### Request

No request body. Optional query parameters may be used for pagination (behaviour is determined by the underlying resource framework).

### Response — 200 OK

An array of User objects.

```json
[
  {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "username": "alice",
    "email": "alice@example.com",
    "created_at": "2026-01-15 09:30:00",
    "is_super_admin": false,
    "is_active": true
  },
  {
    "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
    "username": "bob",
    "email": "bob@example.com",
    "created_at": "2026-02-03 14:00:00",
    "is_super_admin": false,
    "is_active": true
  }
]
```

### Error Responses

| Status | Condition                                          |
|--------|----------------------------------------------------|
| 401    | Missing, malformed, or expired token               |
| 403    | Token does not contain `admin:users:read` scope    |
| 500    | Database error                                     |

---

## Get User

Returns a single user by their UUID.

```
GET /api/v1/admin/users/{id}
```

**Required scope:** `admin:users:read`

### Path Parameters

| Parameter | Type   | Description      |
|-----------|--------|------------------|
| `id`      | string | UUID of the user |

### Response — 200 OK

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "username": "alice",
  "email": "alice@example.com",
  "created_at": "2026-01-15 09:30:00",
  "is_super_admin": false,
  "is_active": true
}
```

### Error Responses

| Status | Condition                                        |
|--------|--------------------------------------------------|
| 401    | Missing, malformed, or expired token             |
| 403    | Token does not contain `admin:users:read` scope  |
| 404    | No user found with the given ID                  |
| 500    | Database error                                   |

---

## Create User

Creates a new user account along with a bcrypt-hashed password record. If `is_super_admin` is set to `true` in the request body, the caller must themselves hold the `super:admin` scope; otherwise the request is rejected with `401`.

```
POST /api/v1/admin/users
```

**Required scope:** `super:admin`

### Request Body

| Field          | Type   | Required | Description                                                                       |
|----------------|--------|----------|-----------------------------------------------------------------------------------|
| `username`     | string | Yes      | Login name, must be unique                                                        |
| `email`        | string | Yes      | Email address                                                                     |
| `password`     | string | Yes      | Plaintext password, hashed with bcrypt before storage                             |
| `is_active`    | bool   | No       | Defaults to `false` if omitted; set to `true` to allow the account to authenticate |
| `is_super_admin` | bool | No      | Grants `super:admin` privilege. Caller must also hold `super:admin`.              |

```json
{
  "username": "carol",
  "email": "carol@example.com",
  "password": "initialPassword123",
  "is_active": true,
  "is_super_admin": false
}
```

### Response — 201 Created

The newly created User object.

```json
{
  "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "username": "carol",
  "email": "carol@example.com",
  "created_at": "2026-04-19 11:00:00",
  "is_super_admin": false,
  "is_active": true
}
```

### Error Responses

| Status | Condition                                                                      |
|--------|--------------------------------------------------------------------------------|
| 400    | Malformed JSON, or `username`, `email`, or `password` is missing               |
| 401    | Missing/expired token, or `is_super_admin: true` requested by non-super-admin  |
| 403    | Token does not contain `super:admin` scope                                     |
| 500    | Database error or password hashing failure                                     |

---

## Update User (Partial)

Applies a partial update to an existing user. Only the fields present in the body are modified. The `username` field is immutable and cannot be included in the payload.

If the body contains a `password` field, the current password must also be supplied as `old_password`. The old password is verified before the new hash is written.

Setting `is_super_admin` to `true` requires the caller to hold `super:admin`.

```
PATCH /api/v1/admin/users/{id}
```

**Required scope:** `super:admin`

### Path Parameters

| Parameter | Type   | Description      |
|-----------|--------|------------------|
| `id`      | string | UUID of the user |

### Request Body

All fields are optional. Omitted fields are left unchanged.

| Field          | Type   | Description                                                                         |
|----------------|--------|-------------------------------------------------------------------------------------|
| `email`        | string | New email address                                                                   |
| `password`     | string | New plaintext password. Requires `old_password` to also be present.                 |
| `old_password` | string | Current plaintext password. Required when `password` is being changed.              |
| `is_active`    | bool   | Activate or deactivate the account                                                  |
| `is_super_admin` | bool | Requires the caller to also hold `super:admin`                                     |

```json
{
  "email": "carol-new@example.com",
  "is_active": false
}
```

Changing a password:

```json
{
  "old_password": "initialPassword123",
  "password": "newStrongerPassword456"
}
```

### Response — 200 OK

The updated User object.

```json
{
  "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "username": "carol",
  "email": "carol-new@example.com",
  "created_at": "2026-04-19 11:00:00",
  "is_super_admin": false,
  "is_active": false
}
```

### Error Responses

| Status | Condition                                                                           |
|--------|-------------------------------------------------------------------------------------|
| 400    | Malformed JSON, attempt to change `username`, or `old_password` missing when changing password |
| 401    | Invalid or missing token, invalid old password, or `is_super_admin: true` set by non-super-admin |
| 403    | Token does not contain `super:admin` scope                                          |
| 404    | No user found with the given ID                                                     |
| 500    | Database error                                                                      |

---

## Replace User (Full)

Replaces a user record in full. The same rules as PATCH apply: `username` cannot be changed, password change requires `old_password`, and escalating to `is_super_admin: true` requires the caller to hold `super:admin`.

```
PUT /api/v1/admin/users/{id}
```

**Required scope:** `super:admin`

### Path Parameters

| Parameter | Type   | Description      |
|-----------|--------|------------------|
| `id`      | string | UUID of the user |

### Request Body

Supply all writable fields. The handler behaviour is identical to PATCH — the same update mechanism is used internally and fields absent from the body retain their existing values.

```json
{
  "email": "carol@example.com",
  "is_active": true,
  "is_super_admin": false
}
```

### Response — 200 OK

The updated User object. Same shape as the PATCH response.

### Error Responses

Same as PATCH.

---

## Delete User

Permanently deletes a user record. Several safety guards are enforced:

- A user cannot delete their own account.
- Deleting a super admin requires the caller to hold `super:admin`.
- If the target user is the last remaining super admin, deletion is blocked to prevent lockout.

```
DELETE /api/v1/admin/users/{id}
```

**Required scope:** `super:admin`

### Path Parameters

| Parameter | Type   | Description      |
|-----------|--------|------------------|
| `id`      | string | UUID of the user |

### Response — 200 OK

The deleted User object is returned.

```json
{
  "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "username": "carol",
  "email": "carol@example.com",
  "created_at": "2026-04-19 11:00:00",
  "is_super_admin": false,
  "is_active": false
}
```

### Error Responses

| Status | Condition                                                                       |
|--------|---------------------------------------------------------------------------------|
| 401    | Missing or invalid token                                                        |
| 403    | Token does not contain `super:admin` scope, caller is trying to delete themselves, or target is the last super admin |
| 404    | No user found with the given ID                                                 |
| 500    | Database error                                                                  |

---

## Get My ACL

Returns the effective access control list for the currently authenticated user, split into scopes assigned directly to the user and scopes inherited through group memberships.

The user is identified from the `sub` claim of the bearer token; no ID is required in the path.

```
GET /api/v1/admin/me/acl
```

**Required scope:** `admin:me`

### Response — 200 OK

```json
{
  "direct_acl": [
    "admin:users:read",
    "admin:acl:read"
  ],
  "inherited_acl": [
    "admin:groups:read"
  ]
}
```

| Field          | Type     | Description                                                                  |
|----------------|----------|------------------------------------------------------------------------------|
| `direct_acl`   | string[] | Scopes assigned via `user_acl` entries directly linked to this user          |
| `inherited_acl`| string[] | Scopes inherited from all active groups the user belongs to via `user_group_acl` |

Both arrays are always present; they are empty arrays when no scopes exist for the respective source.

### Error Responses

| Status | Condition                                      |
|--------|------------------------------------------------|
| 401    | Missing, malformed, or expired token           |
| 403    | Token does not contain `admin:me` scope        |
| 500    | Database error                                 |

---

## Get My Groups

Returns the groups the currently authenticated user belongs to, along with the ACL scopes those groups grant.

The user is identified from the `sub` claim of the bearer token.

```
GET /api/v1/admin/me/admin_groups
```

**Required scope:** `admin:me`

### Response — 200 OK

```json
{
  "groups": [
    {
      "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "title": "Editors"
    }
  ],
  "inherited_acl": [
    "admin:groups:read",
    "admin:users:read"
  ]
}
```

| Field          | Type     | Description                                                                  |
|----------------|----------|------------------------------------------------------------------------------|
| `groups`       | object[] | List of groups the user is an active member of. Each entry has `id` and `title`. |
| `inherited_acl`| string[] | Deduplicated union of all scope strings from all group ACL records associated with the user's memberships |

### Error Responses

| Status | Condition                                      |
|--------|------------------------------------------------|
| 401    | Missing, malformed, or expired token           |
| 403    | Token does not contain `admin:me` scope        |
| 500    | Database error                                 |
