package entity

type ApplicationUserAcl struct {
	_              struct{} `table:"application_user_acl"`
	ID             string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	ApplicationID  string   `db:"application_id" dbtype:"TEXT" nullable:"false" json:"application_id"`
	UserID         string   `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	Roles          JSONText `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	GrantableRoles JSONText `db:"grantable_roles" dbtype:"JSON" nullable:"false" default:"[]" json:"grantable_roles"`
	ResourceIDs    JSONText `db:"resource_ids" dbtype:"JSON" nullable:"false" default:"{}" json:"resource_ids"`
	CreatedAt      string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive       bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
