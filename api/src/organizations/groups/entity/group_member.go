package entity

import (
	users_entity "github.com/a-digi/coco-iam/src/organizations/users/entity"
)

type OrganizationGroupMember struct {
	_         struct{}            `table:"user_group_members"`
	ID        string              `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	GroupID   string              `db:"group_id" dbtype:"TEXT" nullable:"false" json:"group_id"`
	UserID    string              `db:"user_id" dbtype:"TEXT" nullable:"false" json:"user_id"`
	CreatedAt string              `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive  bool                `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
	User      *users_entity.User  `relation:"m2o" join_fk:"user_id" join_ass_fk:"id" join_table:"users" json:"user,omitempty"`
	Group     *OrganizationGroup  `relation:"m2o" join_fk:"group_id" join_ass_fk:"id" join_table:"user_groups" json:"group,omitempty"`
}
