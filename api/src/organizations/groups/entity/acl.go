package entity

import "encoding/json"

type OrganizationGroupAcl struct {
	_         struct{}        `table:"organization_group_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	GroupID   string          `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
