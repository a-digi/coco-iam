package query

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/a-digi/coco-iam/src/admin/users/entity"
	db "github.com/a-digi/coco-orm/orm"
)

type AdminUserQueryRepository struct {
	DatabaseManager *db.DatabaseManager
}

func NewAdminUserQueryRepository(manager *db.DatabaseManager) *AdminUserQueryRepository {
	return &AdminUserQueryRepository{
		DatabaseManager: manager,
	}
}

func (repo *AdminUserQueryRepository) FindByUsername(username string) (*entity.User, bool, error) {
	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "username", "email", "created_at", "is_active").
		From("admin_users").
		Where("username = ?", username).
		Where("is_active = ?", true).
		Limit(1)
	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	return repo.hydrateFirstUser(rows)
}

func (repo *AdminUserQueryRepository) FindSuperAdmins() (*entity.User, bool, error) {
	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "username", "email", "is_active", "is_super_admin").
		From("admin_users").
		Where("is_super_admin = ?", true).
		Where("is_active = ?", true).
		Limit(1)
	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var results []entity.User
	err = repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)
	if err != nil {
		return nil, false, err
	}

	if len(results) == 0 {
		return nil, true, nil
	}

	return &results[0], false, nil
}

func (repo *AdminUserQueryRepository) FindByEmail(email string) (*entity.User, bool, error) {
	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "username", "email", "created_at", "is_active").
		From("admin_users").
		Where("email = ?", email).
		Where("is_active = ?", true).
		Limit(1)
	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)

	if err != nil {
		return nil, false, err
	}

	defer rows.Close()

	return repo.hydrateFirstUser(rows)
}

func (repo *AdminUserQueryRepository) FindByPagination(page int, limit int) ([]entity.User, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "username", "email", "created_at", "is_active").
		From("admin_users").
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset)

	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []entity.User
	err = repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)

	if err != nil {
		return nil, err
	}

	return results, nil
}

// ExistsByUsername reports whether any admin user (active or not) already
// holds the given username. Comparison is case-insensitive.
func (repo *AdminUserQueryRepository) ExistsByUsername(username string) (bool, error) {
	var found int
	err := repo.DatabaseManager.Connector.DB.QueryRow(
		`SELECT 1 FROM admin_users WHERE LOWER(username) = LOWER(?) LIMIT 1`, username,
	).Scan(&found)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ExistsByEmailExcludingID reports whether any admin user other than
// excludeID already holds the given email address. Pass excludeID = ""
// to check without exclusion (create path). Comparison is case-insensitive.
func (repo *AdminUserQueryRepository) ExistsByEmailExcludingID(email, excludeID string) (bool, error) {
	var found int
	var err error
	if excludeID == "" {
		err = repo.DatabaseManager.Connector.DB.QueryRow(
			`SELECT 1 FROM admin_users WHERE LOWER(email) = LOWER(?) LIMIT 1`, email,
		).Scan(&found)
	} else {
		err = repo.DatabaseManager.Connector.DB.QueryRow(
			`SELECT 1 FROM admin_users WHERE LOWER(email) = LOWER(?) AND id != ? LIMIT 1`, email, excludeID,
		).Scan(&found)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (repo *AdminUserQueryRepository) FindByID(id string) (*entity.User, error) {
	var u entity.User
	var createdAt sql.NullString
	err := repo.DatabaseManager.Connector.DB.QueryRow(
		`SELECT id, username, email, created_at, is_active, is_super_admin FROM admin_users WHERE id = ? LIMIT 1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &createdAt, &u.IsActive, &u.IsSuperAdmin)
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		u.CreatedAt = createdAt.String
	}
	return &u, nil
}

func (repo *AdminUserQueryRepository) hydrateFirstUser(rows *sql.Rows) (*entity.User, bool, error) {

	var results []entity.User
	err := repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)

	if err != nil {
		return nil, false, err
	}
	if len(results) == 0 {
		return nil, false, nil
	}

	return &results[0], true, nil
}

type MeGroupsResult struct {
	Groups       []map[string]interface{} `json:"groups"`
	InheritedAcl []string                 `json:"inherited_acl"`
}

func (repo *AdminUserQueryRepository) GetMeGroups(userId string) (*MeGroupsResult, error) {
	result := &MeGroupsResult{
		Groups:       make([]map[string]interface{}, 0),
		InheritedAcl: make([]string, 0),
	}

	// 1. Fetch current groups
	groupQb := repo.DatabaseManager.QueryBuilder().
		Select("admin_groups.id", "admin_groups.title").
		From("admin_groups").
		Join("JOIN admin_group_members m ON admin_groups.id = m.group_id").
		Where("m.user_id = ?", userId).
		Where("m.is_active = ?", true).
		Where("admin_groups.is_active = ?", true)

	groupQuery := groupQb.Build()
	rows, err := repo.DatabaseManager.Connector.DB.Query(groupQuery.SQL, groupQuery.Args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, title string
			if err := rows.Scan(&id, &title); err == nil {
				result.Groups = append(result.Groups, map[string]interface{}{"id": id, "title": title})
			}
		}
	}

	// 2. Fetch inherited ACL by groups
	inhQb := repo.DatabaseManager.QueryBuilder().
		Select("admin_group_acl.roles").
		From("admin_group_acl").
		Join("JOIN admin_group_members m ON m.group_id = admin_group_acl.group_id").
		Where("m.user_id = ?", userId).
		Where("m.is_active = ?", true).
		Where("admin_group_acl.is_active = ?", true)

	inhQuery := inhQb.Build()
	inhRows, err := repo.DatabaseManager.Connector.DB.Query(inhQuery.SQL, inhQuery.Args...)
	if err == nil {
		defer inhRows.Close()
		for inhRows.Next() {
			var rolesJSON string
			if err := inhRows.Scan(&rolesJSON); err == nil && rolesJSON != "" {
				var roles []string
				if err := json.Unmarshal([]byte(rolesJSON), &roles); err == nil {
					result.InheritedAcl = append(result.InheritedAcl, roles...)
				}
			}
		}
	}

	return result, nil
}

type MeAclResult struct {
	DirectAcl    []string `json:"direct_acl"`
	InheritedAcl []string `json:"inherited_acl"`
}

func (repo *AdminUserQueryRepository) GetMeAcl(userId string) (*MeAclResult, error) {
	result := &MeAclResult{
		DirectAcl:    make([]string, 0),
		InheritedAcl: make([]string, 0),
	}

	// 1. Fetch directly assigned user ACL
	aclQb := repo.DatabaseManager.QueryBuilder().
		Select("roles").
		From("admin_acl").
		Where("user_id = ?", userId).
		Where("is_active = ?", true)

	aclQuery := aclQb.Build()
	aclRows, err := repo.DatabaseManager.Connector.DB.Query(aclQuery.SQL, aclQuery.Args...)
	if err == nil {
		defer aclRows.Close()
		for aclRows.Next() {
			var rolesJSON string
			if err := aclRows.Scan(&rolesJSON); err == nil && rolesJSON != "" {
				var roles []string
				if err := json.Unmarshal([]byte(rolesJSON), &roles); err == nil {
					result.DirectAcl = append(result.DirectAcl, roles...)
				}
			}
		}
	}

	// 2. Fetch inherited ACL by groups
	inhQb := repo.DatabaseManager.QueryBuilder().
		Select("admin_group_acl.roles").
		From("admin_group_acl").
		Join("JOIN admin_group_members m ON m.group_id = admin_group_acl.group_id").
		Where("m.user_id = ?", userId).
		Where("m.is_active = ?", true).
		Where("admin_group_acl.is_active = ?", true)

	inhQuery := inhQb.Build()
	inhRows, err := repo.DatabaseManager.Connector.DB.Query(inhQuery.SQL, inhQuery.Args...)
	if err == nil {
		defer inhRows.Close()
		for inhRows.Next() {
			var rolesJSON string
			if err := inhRows.Scan(&rolesJSON); err == nil && rolesJSON != "" {
				var roles []string
				if err := json.Unmarshal([]byte(rolesJSON), &roles); err == nil {
					result.InheritedAcl = append(result.InheritedAcl, roles...)
				}
			}
		}
	}

	return result, nil
}
