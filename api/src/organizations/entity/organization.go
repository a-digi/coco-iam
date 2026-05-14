package entity

// Organization is the top-level tenant. The human-readable
// OrganizationID is globally unique across all organizations and
// used as the first segment of slug-based URLs
// (e.g. /login/a/<org_id>/<workspace_id>/<client_id>).
type Organization struct {
	_              struct{} `table:"organization"`
	ID             string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	OrganizationID string   `db:"organization_id" dbtype:"TEXT" nullable:"false" default:"" json:"organization_id"`
	Title          string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	Description    string   `db:"description" dbtype:"TEXT" nullable:"false" default:"" json:"description"`
	CreatedAt      string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive       bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
