package entity

// Workspace belongs to exactly one organization. OrganizationID is
// required on create and immutable afterwards — the validator in
// `workspaces/validator` enforces both.
//
// WorkspaceID is the human-readable, admin-chosen identifier (the
// workspace's analogue of applications.client_id). It is unique
// within an organization, so two organizations can both have a
// workspace with workspace_id "default". The primary key is `id`
// (uuid); `workspace_id` is for display and, eventually, URLs.
type Workspace struct {
	_              struct{} `table:"workspace"`
	ID             string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	WorkspaceID    string   `db:"workspace_id" dbtype:"TEXT" nullable:"false" default:"" json:"workspace_id"`
	Title          string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	Description    string   `db:"description" dbtype:"TEXT" nullable:"false" default:"" json:"description"`
	OrganizationID string   `db:"organization_id" dbtype:"TEXT" nullable:"false" json:"organization_id"`
	CreatedAt      string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive       bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
