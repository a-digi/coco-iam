package entity

import (
	admin_users "github.com/a-digi/coco-iam/src/admin/users/entity"
)

type AdminGroup struct {
	_                struct{} `table:"admin_groups"`
	ID               string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	Title            string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	GroupDescription string   `db:"group_description" dbtype:"TEXT" nullable:"true" json:"group_description"`
	CreatedAt        string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive         bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
}

type AdminGroupMember struct {
	_         struct{}          `table:"admin_group_members"`
	ID        string            `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	GroupID   string            `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	UserID    string            `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	CreatedAt string            `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool              `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
	User      *admin_users.User `relation:"m2o" join_fk:"user_id" join_ass_fk:"id" join_table:"admin_users" json:"user,omitempty"`
	Group     *AdminGroup       `relation:"m2o" join_fk:"group_id" join_ass_fk:"id" join_table:"admin_groups" json:"group,omitempty"`
}
