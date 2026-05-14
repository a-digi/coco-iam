package entity

// ApplicationGroupAcl maps an organization group to an application with a set
// of scope names. A user gets effective app access if they have a direct ACL
// or if they're a member of a group with an ACL on the same app.
type ApplicationGroupAcl struct {
	_              struct{} `table:"application_group_acl"`
	ID             string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	ApplicationID  string   `db:"application_id" dbtype:"TEXT" nullable:"false" json:"application_id"`
	GroupID        string   `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	Roles          JSONText `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	GrantableRoles JSONText `db:"grantable_roles" dbtype:"JSON" nullable:"false" default:"[]" json:"grantable_roles"`
	ResourceIDs    JSONText `db:"resource_ids" dbtype:"JSON" nullable:"false" default:"{}" json:"resource_ids"`
	CreatedAt      string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive       bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
