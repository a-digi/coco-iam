package persistent

import (
	"database/sql"
	"encoding/json"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
)

type AdminAclPersistentRepo struct {
	db *sql.DB
}

func NewAdminAclPersistentRepo(db *sql.DB) *AdminAclPersistentRepo {
	return &AdminAclPersistentRepo{db: db}
}

func (r *AdminAclPersistentRepo) Insert(entry *acl_entity.AdminAcl) error {
	rolesJSON, err := json.Marshal(entry.Roles)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`INSERT INTO admin_acl (id, user_id, roles, is_active) VALUES (?, ?, ?, ?)`,
		entry.ID, entry.UserID, string(rolesJSON), entry.IsActive,
	)
	return err
}

func (r *AdminAclPersistentRepo) UpdateRoles(id string, roles []string, isActive bool) error {
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		`UPDATE admin_acl SET roles = ?, is_active = ? WHERE id = ?`,
		string(rolesJSON), isActive, id,
	)
	return err
}

func (r *AdminAclPersistentRepo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM admin_acl WHERE id = ?`, id)
	return err
}
