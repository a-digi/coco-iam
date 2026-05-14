package entity

import "encoding/json"

type UserAcl struct {
	_         struct{}        `table:"user_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	UserID    string          `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

type AdminAcl struct {
	_         struct{}        `table:"admin_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	UserID    string          `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

type UserGroupAcl struct {
	_         struct{}        `table:"user_group_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	GroupID   string          `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

type AdminGroupAcl struct {
	_         struct{}        `table:"admin_group_acl"`
	ID        string          `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	GroupID   string          `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	CreatedAt string          `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	Roles     json.RawMessage `db:"roles" dbtype:"JSON" nullable:"false" json:"roles"`
	IsActive  bool            `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}
