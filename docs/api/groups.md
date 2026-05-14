# Groups API

Base URL: `http://localhost:2026/api/v1/admin`

All endpoints on this page require a valid JWT in the `Authorization` header.

```
Authorization: Bearer {access_token}
```

This document covers two resource collections: **admin groups** (`admin_groups`) and **group memberships** (`admin_group_members`). Group-level ACL assignment is covered in [acl.md](./acl.md).

---

## Entities

### AdminGroup

Stored in the `user_groups` table with a discriminatory filter on `group_type = "admin"`.

| Field              | JSON key            | Type   | Nullable | Description                                      |
|--------------------|---------------------|--------|----------|--------------------------------------------------|
| ID                 | `id`                | string | No       | UUID, generated on insert                        |
| GroupType          | `group_type`        | string | No       | Always `"admin"` for this resource               |
| Title              | `title`             | string | No       | Human-readable group name                        |
| GroupDescription   | `group_description` | string | Yes      | Optional description of the group's purpose      |
| CreatedAt          | `created_at`        | string | Yes      | ISO 8601 / DATETIME string, set at insert time   |
| IsActive           | `is_active`         | bool   | No       | Inactive groups do not contribute ACL scopes to their members. Defaults to `true`. |

### AdminGroupMember

Stored in the `admin_group_members` table. Represents the membership link between a user and a group.

| Field     | JSON key    | Type   | Nullable | Description                                                       |
|-----------|-------------|--------|----------|-------------------------------------------------------------------|
| ID        | `id`        | string | No       | UUID, generated on insert                                         |
| GroupID   | `group_id`  | string | No       | UUID of the parent `AdminGroup`                                   |
| UserID    | `user_id`   | string | No       | UUID of the member `User`                                         |
| CreatedAt | `created_at`| string | Yes      | ISO 8601 / DATETIME string, set at insert time                    |
| IsActive  | `is_active` | bool   | No       | Inactive memberships do not contribute ACL scopes. Defaults to `true`. |
| User      | `user`      | object | Yes      | Hydrated `User` object when the relation is loaded (omitted when `null`) |
| Group     | `group`     | object | Yes      | Hydrated `AdminGroup` object when the relation is loaded (omitted when `null`) |

---

## Admin Groups

### List Admin Groups

Returns a paginated collection of all admin groups.

```
GET /api/v1/admin/admin_groups
```

**Required scope:** `admin:groups:read` or `admin:groups`

#### Response — 200 OK

```json
[
  {
    "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
    "group_type": "admin",
    "title": "Editors",
    "group_description": "Can read and edit content",
    "created_at": "2026-01-20 10:00:00",
    "is_active": true
  },
  {
    "id": "e5f6a7b8-c9d0-1234-efab-345678901234",
    "group_type": "admin",
    "title": "Auditors",
    "group_description": "Read-only access across all resources",
    "created_at": "2026-02-05 08:45:00",
    "is_active": true
  }
]
```

#### Error Responses

| Status | Condition                                               |
|--------|---------------------------------------------------------|
| 401    | Missing, malformed, or expired token                    |
| 403    | Token does not contain `admin:groups:read` or `admin:groups` scope |
| 500    | Database error                                          |

---

### Get Admin Group

Returns a single admin group by its UUID.

```
GET /api/v1/admin/admin_groups/{id}
```

**Required scope:** `admin:groups:read` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the admin group |

#### Response — 200 OK

```json
{
  "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "group_type": "admin",
  "title": "Editors",
  "group_description": "Can read and edit content",
  "created_at": "2026-01-20 10:00:00",
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                          |
|--------|--------------------------------------------------------------------|
| 401    | Missing, malformed, or expired token                               |
| 403    | Token does not contain `admin:groups:read` or `admin:groups` scope |
| 404    | No group found with the given ID                                   |
| 500    | Database error                                                     |

---

### Create Admin Group

Creates a new admin group.

```
POST /api/v1/admin/admin_groups
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Request Body

| Field               | Type   | Required | Description                                          |
|---------------------|--------|----------|------------------------------------------------------|
| `title`             | string | Yes      | Human-readable group name                            |
| `group_description` | string | No       | Optional description of the group's purpose          |
| `group_type`        | string | Yes      | Must be `"admin"`                                    |

```json
{
  "title": "Content Managers",
  "group_description": "Manages published content and metadata",
  "group_type": "admin"
}
```

#### Response — 201 Created

```json
{
  "id": "f6a7b8c9-d0e1-2345-fabc-456789012345",
  "group_type": "admin",
  "title": "Content Managers",
  "group_description": "Manages published content and metadata",
  "created_at": "2026-04-19 12:00:00",
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                           |
|--------|---------------------------------------------------------------------|
| 400    | Malformed JSON or required fields missing                           |
| 401    | Missing or invalid token                                            |
| 403    | Token does not contain `admin:groups:write` or `admin:groups` scope |
| 500    | Database error                                                      |

---

### Update Admin Group (Partial)

Applies a partial update to an existing admin group. Only the fields present in the body are modified.

```
PATCH /api/v1/admin/admin_groups/{id}
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the admin group |

#### Request Body

All fields are optional. Omitted fields are left unchanged.

| Field               | Type   | Description                             |
|---------------------|--------|-----------------------------------------|
| `title`             | string | New group name                          |
| `group_description` | string | New description                         |
| `is_active`         | bool   | Activate or deactivate the group        |

```json
{
  "group_description": "Manages all published and draft content"
}
```

#### Response — 200 OK

The updated AdminGroup object.

#### Error Responses

| Status | Condition                                                           |
|--------|---------------------------------------------------------------------|
| 400    | Malformed JSON                                                      |
| 401    | Missing or invalid token                                            |
| 403    | Token does not contain `admin:groups:write` or `admin:groups` scope |
| 404    | No group found with the given ID                                    |
| 500    | Database error                                                      |

---

### Replace Admin Group (Full)

Replaces an admin group record in full. Internally uses the same update mechanism as PATCH; fields absent from the body retain their existing values.

```
PUT /api/v1/admin/admin_groups/{id}
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the admin group |

#### Request Body

Supply all writable fields.

```json
{
  "title": "Content Managers",
  "group_description": "Manages all published and draft content",
  "group_type": "admin",
  "is_active": true
}
```

#### Response — 200 OK

The updated AdminGroup object.

#### Error Responses

Same as PATCH.

---

### Delete Admin Group

Permanently deletes an admin group.

```
DELETE /api/v1/admin/admin_groups/{id}
```

**Required scope:** `admin:groups:delete` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description             |
|-----------|--------|-------------------------|
| `id`      | string | UUID of the admin group |

#### Response — 200 OK

The deleted AdminGroup object.

#### Error Responses

| Status | Condition                                                            |
|--------|----------------------------------------------------------------------|
| 401    | Missing or invalid token                                             |
| 403    | Token does not contain `admin:groups:delete` or `admin:groups` scope |
| 404    | No group found with the given ID                                     |
| 500    | Database error                                                       |

---

## Group Members

### List Group Members

Returns a paginated collection of all group membership records. Each record includes the linked `user` and `group` relations when they are available.

```
GET /api/v1/admin/admin_group_members
```

**Required scope:** `admin:groups:read` or `admin:groups`

#### Response — 200 OK

```json
[
  {
    "id": "a7b8c9d0-e1f2-3456-abcd-567890123456",
    "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
    "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "created_at": "2026-03-01 09:00:00",
    "is_active": true,
    "user": {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "username": "alice",
      "email": "alice@example.com",
      "created_at": "2026-01-15 09:30:00",
      "is_super_admin": false,
      "is_active": true
    },
    "group": {
      "id": "d4e5f6a7-b8c9-0123-defa-234567890123",
      "group_type": "admin",
      "title": "Editors",
      "group_description": "Can read and edit content",
      "created_at": "2026-01-20 10:00:00",
      "is_active": true
    }
  }
]
```

#### Error Responses

| Status | Condition                                               |
|--------|---------------------------------------------------------|
| 401    | Missing, malformed, or expired token                    |
| 403    | Token does not contain `admin:groups:read` or `admin:groups` scope |
| 500    | Database error                                          |

---

### Get Group Member

Returns a single group membership record by its UUID.

```
GET /api/v1/admin/admin_group_members/{id}
```

**Required scope:** `admin:groups:read` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description                   |
|-----------|--------|-------------------------------|
| `id`      | string | UUID of the membership record |

#### Response — 200 OK

A single AdminGroupMember object with `user` and `group` relations included where available. See the list response for the full shape.

#### Error Responses

| Status | Condition                                                          |
|--------|--------------------------------------------------------------------|
| 401    | Missing, malformed, or expired token                               |
| 403    | Token does not contain `admin:groups:read` or `admin:groups` scope |
| 404    | No membership record found with the given ID                       |
| 500    | Database error                                                     |

---

### Add Group Member

Creates a new membership record linking a user to a group.

```
POST /api/v1/admin/admin_group_members
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Request Body

| Field      | Type   | Required | Description                        |
|------------|--------|----------|------------------------------------|
| `group_id` | string | Yes      | UUID of the target admin group     |
| `user_id`  | string | Yes      | UUID of the user to add            |

```json
{
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

#### Response — 201 Created

```json
{
  "id": "a7b8c9d0-e1f2-3456-abcd-567890123456",
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "created_at": "2026-04-19 13:00:00",
  "is_active": true
}
```

#### Error Responses

| Status | Condition                                                           |
|--------|---------------------------------------------------------------------|
| 400    | Malformed JSON or required fields missing                           |
| 401    | Missing or invalid token                                            |
| 403    | Token does not contain `admin:groups:write` or `admin:groups` scope |
| 500    | Database error                                                      |

---

### Update Group Member (Partial)

Applies a partial update to an existing membership record. Typically used to toggle the `is_active` flag.

```
PATCH /api/v1/admin/admin_group_members/{id}
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description                   |
|-----------|--------|-------------------------------|
| `id`      | string | UUID of the membership record |

#### Request Body

| Field       | Type   | Description                             |
|-------------|--------|-----------------------------------------|
| `is_active` | bool   | Activate or deactivate the membership   |
| `group_id`  | string | Reassign to a different group           |
| `user_id`   | string | Reassign to a different user            |

```json
{
  "is_active": false
}
```

#### Response — 200 OK

The updated AdminGroupMember object.

#### Error Responses

| Status | Condition                                                           |
|--------|---------------------------------------------------------------------|
| 400    | Malformed JSON                                                      |
| 401    | Missing or invalid token                                            |
| 403    | Token does not contain `admin:groups:write` or `admin:groups` scope |
| 404    | No membership record found with the given ID                        |
| 500    | Database error                                                      |

---

### Replace Group Member (Full)

Replaces a membership record in full. Internally identical to PATCH.

```
PUT /api/v1/admin/admin_group_members/{id}
```

**Required scope:** `admin:groups:write` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description                   |
|-----------|--------|-------------------------------|
| `id`      | string | UUID of the membership record |

#### Request Body

```json
{
  "group_id": "d4e5f6a7-b8c9-0123-defa-234567890123",
  "user_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "is_active": true
}
```

#### Response — 200 OK

The updated AdminGroupMember object.

#### Error Responses

Same as PATCH.

---

### Remove Group Member

Permanently deletes a membership record.

```
DELETE /api/v1/admin/admin_group_members/{id}
```

**Required scope:** `admin:groups:delete` or `admin:groups`

#### Path Parameters

| Parameter | Type   | Description                   |
|-----------|--------|-------------------------------|
| `id`      | string | UUID of the membership record |

#### Response — 200 OK

The deleted AdminGroupMember object.

#### Error Responses

| Status | Condition                                                            |
|--------|----------------------------------------------------------------------|
| 401    | Missing or invalid token                                             |
| 403    | Token does not contain `admin:groups:delete` or `admin:groups` scope |
| 404    | No membership record found with the given ID                         |
| 500    | Database error                                                       |

---

## Scope Inheritance

When a user authenticates, their effective scopes are computed as follows:

1. If the user's `is_super_admin` flag is `true`, the `super:admin` scope is added unconditionally.
2. All active `user_acl` records for the user are collected and their `roles` arrays are merged.
3. All groups the user belongs to (via active `admin_group_members` records) are resolved. For each group, all active `user_group_acl` records are collected and their `roles` arrays are merged.
4. Duplicates are removed. The resulting set is encoded as a space-separated string in the `scope` JWT claim.

Deactivating a membership (`is_active: false`) or a group (`is_active: false`) prevents those group scopes from contributing to the token at next login. Existing tokens are not revoked; they remain valid until expiry.
