package entity

import "encoding/json"

type OrganizationUserAcl struct {
	_         struct{}        `table:"organization_user_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	UserID    string          `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

// UserScopeView is the enriched response returned by the organization_user_acl
// GET endpoint when filtered by user_id. It aggregates all three inheritance
// sources so the caller can see exactly where every role comes from.
type UserScopeView struct {
	UserID         string            `json:"user_id"`
	Direct         *OrgDirectGrant   `json:"direct"`
	FromGroups     []GroupScopeGrant `json:"from_groups"`
	FromApps       []AppScopeGrant   `json:"from_apps"`
	EffectiveRoles []string          `json:"effective_roles"`
}

// OrgDirectGrant is the org-level role grant assigned directly to a user.
type OrgDirectGrant struct {
	ID        string          `json:"id"`
	Roles     json.RawMessage `json:"roles"`
	IsActive  bool            `json:"is_active"`
	CreatedAt string          `json:"created_at"`
}

// GroupScopeGrant is the role contribution from one group the user belongs to.
type GroupScopeGrant struct {
	GroupID   string          `json:"group_id"`
	GroupName string          `json:"group_name"`
	Roles     json.RawMessage `json:"roles"`
	IsActive  bool            `json:"is_active"`
}

// AppScopeGrant is the role contribution from one application the user has
// been added to.
type AppScopeGrant struct {
	ApplicationID string          `json:"application_id"`
	ClientID      string          `json:"client_id"`
	Roles         json.RawMessage `json:"roles"`
	IsActive      bool            `json:"is_active"`
}
