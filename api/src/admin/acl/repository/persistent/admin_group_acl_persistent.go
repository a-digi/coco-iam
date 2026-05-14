package persistent

import (
	"database/sql"
	"encoding/json"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
)

type AdminGroupAclPersistentRepo struct {
	db *sql.DB
}

func NewAdminGroupAclPersistentRepo(db *sql.DB) *AdminGroupAclPersistentRepo {
	return &AdminGroupAclPersistentRepo{db: db}
}

func (r *AdminGroupAclPersistentRepo) Insert(entry *acl_entity.AdminGroupAcl) error {
	rolesJSON, err := json.Marshal(entry.Roles)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`INSERT INTO admin_group_acl (id, group_id, roles, is_active) VALUES (?, ?, ?, ?)`,
		entry.ID, entry.GroupID, string(rolesJSON), entry.IsActive,
	)
	return err
}

func (r *AdminGroupAclPersistentRepo) UpdateRoles(id string, roles []string, isActive bool) error {
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE admin_group_acl SET roles = ?, is_active = ? WHERE id = ?`,
		string(rolesJSON), isActive, id,
	)
	return err
}

func (r *AdminGroupAclPersistentRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM admin_group_acl WHERE id = ?`, id)
	return err
}
