// Package deleted implements the "organization-deleted" async cascade.
//
// When the admin hard-deletes an organization, the HTTP DELETE handler
// removes only the `organization` row from the main DB. A delete-event
// listener on the organizations resource then:
//
//  1. Captures a snapshot of the org + its workspaces + applications
//     in BeforeExecution, BEFORE the delete runs.
//  2. Publishes the snapshot to QueueOrganizationDeleted in
//     AfterExecution, AFTER the delete has succeeded.
//
// The consumer in this package picks up the job and:
//
//  1. Writes a row to `deleted_organizations` keyed by organization id
//     with the JSON snapshot — idempotent via INSERT OR IGNORE.
//  2. Moves ./data/db/organization/<orgID>/ into
//     ./data/db/deleted/organization/<orgID>/ (folder created on
//     first use). Archive layout mirrors the live tree on purpose —
//     admins can navigate both symmetrically. Org UUIDs are unique
//     so the base destination never collides in practice; a __stamp
//     suffix only appears on a manually-replayed identical delete.
//  3. Deletes every downstream row the org owned: workspaces,
//     applications + their login-settings/assets/keys/scopes/ACLs,
//     user_groups + members + ACL scoped to the org, users whose
//     organization_id matched + their password/ACL/group-member rows,
//     organization_user_acl / organization_group_acl rows are removed
//     implicitly when the per-org folder is archived, plus the
//     org-scoped user_rule_sets entry.
//  4. Walks ./data/uploads/media/<appID>/ and
//     ./data/uploads/login-assets/<appID>/ and removes the directory
//     trees for every deleted application.
//
// Every step is idempotent so retries are safe.
package deleted

// Payload is the message shape on QueueOrganizationDeleted. The
// listener captures the snapshot synchronously in BeforeExecution
// before the rows it references get deleted; the consumer trusts the
// snapshot as its source of truth for what to cascade-delete.
type Payload struct {
	OrganizationID string   `json:"organization_id"`
	Snapshot       Snapshot `json:"snapshot"`
}

// Snapshot is the admin-visible record of what the organization
// contained at the moment of deletion. Captured synchronously in the
// delete listener's BeforeExecution so the consumer never has to
// re-read rows that no longer exist.
type Snapshot struct {
	Organization OrganizationRow `json:"organization"`
	Workspaces   []WorkspaceRow  `json:"workspaces"`
	Applications []ApplicationRow `json:"applications"`
}

// OrganizationRow mirrors the subset of the `organization` row we
// retain for audit. No admin secrets; just identity + config.
type OrganizationRow struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CreatedAt      string `json:"created_at"`
	IsActive       bool   `json:"is_active"`
}

// WorkspaceRow mirrors the core identity of a workspace owned by the
// deleted org.
type WorkspaceRow struct {
	ID             string `json:"id"`
	WorkspaceID    string `json:"workspace_id"`
	OrganizationID string `json:"organization_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	CreatedAt      string `json:"created_at"`
	IsActive       bool   `json:"is_active"`
}

// ApplicationRow mirrors the core identity of an application owned by
// a workspace in the deleted org.
type ApplicationRow struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	IsActive    bool   `json:"is_active"`
}
