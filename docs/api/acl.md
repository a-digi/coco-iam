# ACL API

Base URL: `http://localhost:2026/api/v1/admin`

All endpoints on this page require a valid JWT in the `Authorization` header.

```
Authorization: Bearer {access_token}
```

This document covers two ACL resource collections:

- **User ACL** (`user_acl`) — scopes assigned directly to individual users.
- **Group ACL** (`user_group_acl`) — scopes assigned to groups and inherited by all active members.

For the full catalog of scope identifiers, call the [`GET /api/v1/admin/acl/scopes`](./authentication.md#list-available-scopes) endpoint. For information on how these scopes are merged into a token at authentication time, see the [scope inheritance section in groups.md](./groups.md#scope-inheritance).

---

## The `roles` Field

Both ACL entities store their permissions as a JSON array of scope strings in a column named `roles`. The array may contain any combination of scope identifiers from the system catalog.

```json
["admin:users:read", "admin:groups:read", "admin:acl:read"]
```

A single ACL record can hold multiple scopes. Multiple active ACL records for the same user or group are all merged at authentication time; there is no precedence — all scopes from all active records are unioned together.

Passing an empty array (`[]`) is valid and effectively grants no permissions from that record.

---

## User ACL

### List User ACL Records

Returns all user ACL records in the system.

```
GET /api/v1/admin/user_acl
```

**Required scope:** `admin:acl:read` or `admin:acl`

#### Response — 200 OK

```json
[
  {
    "id": "b1c2d3e4-f5a6-7890-bcde-f12345678901",
    "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "created_at": "2026-01-16 10:00:00",
    "roles": ["admin:users:read", "admin:groups:read"],
    "is_active": true
  }
]
```

| Field       | JSON key     | Type     | Description                                          |
|-------------|--------------|----------|------------------------------------------------------|
| ID          | `id`         | string   | UUID of the ACL record                               |
| UserID      | `user_id`    | string   | UUID of the user this record is assigned to          |
| CreatedAt   | `created_at` | string   | ISO 8601 / DATETIME string                           |
| Roles       | `roles`      | string[] | JSON array of scope strings                          |
| IsActive    | `is_active`  | bool     | Inactive records are excluded from token computation |

#### Error Responses

| Status | Condition                                              |
|--------|--------------------------------------------------------|
| 401    | Missing, malformed, or expired token                   |
| 403    | Token does not contain `admin:acl:read` or `admin:acl` |
| 500    | Database error                                         |

---

### Get User ACL Record

Returns a single user ACL record by its UUID.

```
GET /api/v1/admin/user_acl/{id}
```

**Required scope:** `admin:users:acl:read` or `admin:users:acl`

#### Path Parameters

| Parameter | Type   | Description                |
|-----------|--------|----------------------------|
| `id`      | string | UUID of the ACL record     |

#### Response — 200 OK

```json
{
  "id": "b1c2d3e4-f5a6-7890-bcde-f12345678901",
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "created_at": "2026-01-16 10:00:00",
  "roles": ["admin:users:read", "admin:groups:read"],
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                            |
|--------|----------------------------------------------------------------------|
| 401    | Missing, malformed, or expired token                                 |
| 403    | Token does not contain `admin:users:acl:read` or `admin:users:acl`  |
| 404    | No ACL record found with the given ID                                |
| 500    | Database error                                                       |

---

### Create User ACL Record

Assigns a set of scopes to a user by creating a new ACL record. A user may have multiple active ACL records; all of them are merged at authentication time.

```
POST /api/v1/admin/user_acl
```

**Required scope:** `admin:users:acl:write` or `admin:users:acl`

#### Request Body

| Field     | Type     | Required | Description                                                      |
|-----------|----------|----------|------------------------------------------------------------------|
| `user_id` | string   | Yes      | UUID of the user to assign the scopes to                         |
| `roles`   | string[] | Yes      | JSON array of scope strings. Use an empty array to create a placeholder with no scopes. |

```json
{
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "roles": ["admin:users:read", "admin:acl:read"]
}
```

#### Response — 201 Created

```json
{
  "id": "c2d3e4f5-a6b7-8901-cdef-234567890123",
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "created_at": "2026-04-19 14:00:00",
  "roles": ["admin:users:read", "admin:acl:read"],
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                            |
|--------|----------------------------------------------------------------------|
| 400    | Malformed JSON or required fields missing                            |
| 401    | Missing or invalid token                                             |
| 403    | Token does not contain `admin:users:acl:write` or `admin:users:acl` |
| 500    | Database error                                                       |

---

### Update User ACL Record (Partial)

Applies a partial update to an existing user ACL record. Use this to add or replace scopes on a record, or to deactivate a record without deleting it.

```
PATCH /api/v1/admin/user_acl/{id}
```

**Required scope:** `admin:users:acl:write` or `admin:users:acl`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the ACL record  |

#### Request Body

All fields are optional. Omitted fields are left unchanged.

| Field       | Type     | Description                                           |
|-------------|----------|-------------------------------------------------------|
| `roles`     | string[] | Replacement roles array. The entire array is replaced, not appended to. |
| `is_active` | bool     | Set to `false` to disable the record without deleting it |

```json
{
  "roles": ["admin:users:read", "admin:groups:read", "admin:acl:read"]
}
```

#### Response — 200 OK

The updated UserAcl object.

#### Error Responses

| Status | Condition                                                            |
|--------|----------------------------------------------------------------------|
| 400    | Malformed JSON                                                       |
| 401    | Missing or invalid token                                             |
| 403    | Token does not contain `admin:users:acl:write` or `admin:users:acl` |
| 404    | No ACL record found with the given ID                                |
| 500    | Database error                                                       |

---

### Replace User ACL Record (Full)

Replaces a user ACL record in full. Internally uses the same update mechanism as PATCH.

```
PUT /api/v1/admin/user_acl/{id}
```

**Required scope:** `admin:users:acl:write` or `admin:users:acl`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the ACL record  |

#### Request Body

```json
{
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "roles": ["admin:users:read", "admin:groups:read"],
  "is_active": true
}
```

#### Response — 200 OK

The updated UserAcl object.

#### Error Responses

Same as PATCH.

---

### Delete User ACL Record

Permanently deletes a user ACL record.

```
DELETE /api/v1/admin/user_acl/{id}
```

**Required scope:** `admin:users:acl:delete` or `admin:users:acl`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the ACL record  |

#### Response — 200 OK

The deleted UserAcl object.

#### Error Responses

| Status | Condition                                                              |
|--------|------------------------------------------------------------------------|
| 401    | Missing or invalid token                                               |
| 403    | Token does not contain `admin:users:acl:delete` or `admin:users:acl`  |
| 404    | No ACL record found with the given ID                                  |
| 500    | Database error                                                         |

---

## Group ACL

Group ACL records assign scopes to entire groups. Any user who is an active member of a group inherits the group's scopes at authentication time. This is the primary mechanism for role-based access at scale.

### List Group ACL Records

Returns all group ACL records in the system.

```
GET /api/v1/admin/user_group_acl
```

**Required scope:** `admin:acl:read` or `admin:acl`

#### Response — 200 OK

```json
[
  {
    "id": "d3e4f5a6-b7c8-9012-defa-345678901234",
    "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
    "created_at": "2026-01-21 11:00:00",
    "roles": ["admin:users:read", "admin:groups:read"],
    "is_active": true
  }
]
```

| Field       | JSON key     | Type     | Description                                              |
|-------------|--------------|----------|----------------------------------------------------------|
| ID          | `id`         | string   | UUID of the group ACL record                             |
| GroupID     | `group_id`   | string   | UUID of the group this record is assigned to             |
| CreatedAt   | `created_at` | string   | ISO 8601 / DATETIME string                               |
| Roles       | `roles`      | string[] | JSON array of scope strings                              |
| IsActive    | `is_active`  | bool     | Inactive records are excluded from scope inheritance     |

#### Error Responses

| Status | Condition                                              |
|--------|--------------------------------------------------------|
| 401    | Missing, malformed, or expired token                   |
| 403    | Token does not contain `admin:acl:read` or `admin:acl` |
| 500    | Database error                                         |

---

### Get Group ACL Record

Returns a single group ACL record by its UUID.

```
GET /api/v1/admin/user_group_acl/{id}
```

**Required scope:** `admin:groups:acl:read` or `admin:groups:acl`

#### Path Parameters

| Parameter | Type   | Description                   |
|-----------|--------|-------------------------------|
| `id`      | string | UUID of the group ACL record  |

#### Response — 200 OK

```json
{
  "id": "d3e4f5a6-b7c8-9012-defa-345678901234",
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "created_at": "2026-01-21 11:00:00",
  "roles": ["admin:users:read", "admin:groups:read"],
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                              |
|--------|------------------------------------------------------------------------|
| 401    | Missing, malformed, or expired token                                   |
| 403    | Token does not contain `admin:groups:acl:read` or `admin:groups:acl`  |
| 404    | No group ACL record found with the given ID                            |
| 500    | Database error                                                         |

---

### Create Group ACL Record

Assigns a set of scopes to a group. All active members of the group will inherit these scopes at their next authentication.

```
POST /api/v1/admin/user_group_acl
```

**Required scope:** `admin:groups:acl:write` or `admin:groups:acl`

#### Request Body

| Field      | Type     | Required | Description                                                      |
|------------|----------|----------|------------------------------------------------------------------|
| `group_id` | string   | Yes      | UUID of the group to assign the scopes to                        |
| `roles`    | string[] | Yes      | JSON array of scope strings. Use an empty array to create a placeholder with no scopes. |

```json
{
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "roles": ["admin:users:read", "admin:groups:read", "admin:acl:read"]
}
```

#### Response — 201 Created

```json
{
  "id": "e4f5a6b7-c8d9-0123-efab-456789012345",
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "created_at": "2026-04-19 15:00:00",
  "roles": ["admin:users:read", "admin:groups:read", "admin:acl:read"],
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                              |
|--------|------------------------------------------------------------------------|
| 400    | Malformed JSON or required fields missing                              |
| 401    | Missing or invalid token                                               |
| 403    | Token does not contain `admin:groups:acl:write` or `admin:groups:acl` |
| 500    | Database error                                                         |

---

### Update Group ACL Record (Partial)

Applies a partial update to an existing group ACL record.

```
PATCH /api/v1/admin/user_group_acl/{id}
```

**Required scope:** `admin:groups:acl:write` or `admin:groups:acl`

#### Path Parameters

| Parameter | Type   | Description                  |
|-----------|--------|------------------------------|
| `id`      | string | UUID of the group ACL record |

#### Request Body

All fields are optional.

| Field       | Type     | Description                                                    |
|-------------|----------|----------------------------------------------------------------|
| `roles`     | string[] | Replacement roles array. The entire array is replaced.         |
| `is_active` | bool     | Set to `false` to disable without deleting                     |

```json
{
  "roles": ["admin:users:read", "admin:groups:read", "admin:acl:read", "admin:users:acl:read"]
}
```

#### Response — 200 OK

The updated UserGroupAcl object.

#### Error Responses

| Status | Condition                                                              |
|--------|------------------------------------------------------------------------|
| 400    | Malformed JSON                                                         |
| 401    | Missing or invalid token                                               |
| 403    | Token does not contain `admin:groups:acl:write` or `admin:groups:acl` |
| 404    | No group ACL record found with the given ID                            |
| 500    | Database error                                                         |

---

### Replace Group ACL Record (Full)

Replaces a group ACL record in full. Internally uses the same update mechanism as PATCH.

```
PUT /api/v1/admin/user_group_acl/{id}
```

**Required scope:** `admin:groups:acl:write` or `admin:groups:acl`

#### Path Parameters

| Parameter | Type   | Description                  |
|-----------|--------|------------------------------|
| `id`      | string | UUID of the group ACL record |

#### Request Body

```json
{
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "roles": ["admin:users:read", "admin:groups:read"],
  "is_active": true
}
```

#### Response — 200 OK

The updated UserGroupAcl object.

#### Error Responses

Same as PATCH.

---

### Delete Group ACL Record

Permanently deletes a group ACL record. Members of the affected group will lose the scopes it provided at their next authentication (existing tokens are not revoked).

```
DELETE /api/v1/admin/user_group_acl/{id}
```

**Required scope:** `admin:groups:acl:delete` or `admin:groups:acl`

#### Path Parameters

| Parameter | Type   | Description                  |
|-----------|--------|------------------------------|
| `id`      | string | UUID of the group ACL record |

#### Response — 200 OK

The deleted UserGroupAcl object.

#### Error Responses

| Status | Condition                                                               |
|--------|-------------------------------------------------------------------------|
| 401    | Missing or invalid token                                                |
| 403    | Token does not contain `admin:groups:acl:delete` or `admin:groups:acl` |
| 404    | No group ACL record found with the given ID                             |
| 500    | Database error                                                          |

---

## Scope Reference

The following top-level scope groups are defined in the system. Call `GET /api/v1/admin/acl/scopes` (see [authentication.md](./authentication.md#list-available-scopes)) for the complete nested tree with descriptions.

| Scope group            | Covers                                                  |
|------------------------|---------------------------------------------------------|
| `super:admin`          | Unrestricted access; required for user creation and deletion |
| `admin:users`          | User read/write/delete/manage operations                |
| `admin:groups`         | Group and group membership read/write/delete operations |
| `admin:acl`            | Reading any ACL record (users or groups)                |
| `admin:users:acl`      | Reading, writing, and deleting user ACL records         |
| `admin:groups:acl`     | Reading, writing, and deleting group ACL records        |
| `admin:me`             | Reading one's own ACL and group memberships             |

Scope strings follow the pattern `{domain}:{resource}:{action}`. Each endpoint requires a specific action-level scope (e.g. `admin:users:acl:write`) or its parent namespace scope (e.g. `admin:users:acl`). Holding a parent namespace scope satisfies any child scope requirement on that endpoint.
