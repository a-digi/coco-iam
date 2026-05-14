package query

import (
	"database/sql"
	"encoding/json"
	"errors"

	acl_entity "github.com/a-digi/coco-iam/src/admin/acl/entity"
)

type AdminAclQueryRepo struct {
	db *sql.DB
}

func NewAdminAclQueryRepo(db *sql.DB) *AdminAclQueryRepo {
	return &AdminAclQueryRepo{db: db}
}

func scanAcl(scan func(dest ...any) error) (*acl_entity.AdminAcl, error) {
	var a acl_entity.AdminAcl
	// roles is a TEXT/JSON column. mattn/go-sqlite3 returns TEXT as string;
	// database/sql's convertAssign handles *[]byte via type assertion but not
	// *json.RawMessage (a named alias), so we scan into string then convert.
	var rolesStr string
	var createdAt sql.NullString
	if err := scan(&a.ID, &a.UserID, &rolesStr, &createdAt, &a.IsActive); err != nil {
		return nil, err
	}
	a.Roles = json.RawMessage(rolesStr)
	if createdAt.Valid {
		a.CreatedAt = createdAt.String
	}
	return &a, nil
}

func (r *AdminAclQueryRepo) FindByID(id string) (*acl_entity.AdminAcl, error) {
	row := r.db.QueryRow(
		`SELECT id, user_id, roles, created_at, is_active FROM admin_acl WHERE id = ? LIMIT 1`, id,
	)
	a, err := scanAcl(row.Scan)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AdminAclQueryRepo) FindByUserID(userID string) ([]*acl_entity.AdminAcl, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, roles, created_at, is_active FROM admin_acl WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*acl_entity.AdminAcl
	for rows.Next() {
		a, err := scanAcl(rows.Scan)
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

func (r *AdminAclQueryRepo) List() ([]*acl_entity.AdminAcl, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, roles, created_at, is_active FROM admin_acl ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*acl_entity.AdminAcl
	for rows.Next() {
		a, err := scanAcl(rows.Scan)
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

func (r *AdminAclQueryRepo) AdminUserExists(userID string) (bool, error) {
	var found int
	err := r.db.QueryRow(`SELECT 1 FROM admin_users WHERE id = ? LIMIT 1`, userID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
