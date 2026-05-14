package query

import (
	"database/sql"
	"encoding/json"
	"errors"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
)

type AdminGroupAclQueryRepo struct {
	db *sql.DB
}

func NewAdminGroupAclQueryRepo(db *sql.DB) *AdminGroupAclQueryRepo {
	return &AdminGroupAclQueryRepo{db: db}
}

func scanGroupAcl(scan func(dest ...any) error) (*acl_entity.AdminGroupAcl, error) {
	var a acl_entity.AdminGroupAcl
	var rolesStr string
	var createdAt sql.NullString
	if err := scan(&a.ID, &a.GroupID, &rolesStr, &createdAt, &a.IsActive); err != nil {
		return nil, err
	}
	a.Roles = json.RawMessage(rolesStr)
	if createdAt.Valid {
		a.CreatedAt = createdAt.String
	}
	return &a, nil
}

func (r *AdminGroupAclQueryRepo) FindByID(id string) (*acl_entity.AdminGroupAcl, error) {
	row := r.db.QueryRow(
		`SELECT id, group_id, roles, created_at, is_active FROM admin_group_acl WHERE id = ? LIMIT 1`, id,
	)
	return scanGroupAcl(row.Scan)
}

func (r *AdminGroupAclQueryRepo) FindByGroupID(groupID string) ([]*acl_entity.AdminGroupAcl, error) {
	rows, err := r.db.Query(
		`SELECT id, group_id, roles, created_at, is_active FROM admin_group_acl WHERE group_id = ?`, groupID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*acl_entity.AdminGroupAcl
	for rows.Next() {
		a, err := scanGroupAcl(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AdminGroupAclQueryRepo) List() ([]*acl_entity.AdminGroupAcl, error) {
	rows, err := r.db.Query(
		`SELECT id, group_id, roles, created_at, is_active FROM admin_group_acl ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*acl_entity.AdminGroupAcl
	for rows.Next() {
		a, err := scanGroupAcl(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AdminGroupAclQueryRepo) AdminGroupExists(groupID string) (bool, error) {
	var found int
	err := r.db.QueryRow(`SELECT 1 FROM admin_groups WHERE id = ? LIMIT 1`, groupID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
