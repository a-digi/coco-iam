package query

import (
	"database/sql"

	password_entity "github.com/a-digi/coco-iam/src/auth/database/entity"
	db "github.com/a-digi/coco-orm/orm"
)

type PasswordQueryRepository struct {
	DatabaseManager *db.DatabaseManager
	rawDB           *sql.DB // used when no DatabaseManager is available
	tableName       string
}

func NewPasswordQueryRepository(manager *db.DatabaseManager) *PasswordQueryRepository {
	return &PasswordQueryRepository{DatabaseManager: manager, tableName: "user_auth_password"}
}

// NewPasswordQueryRepositoryFromDB constructs a repository backed by a
// bare *sql.DB. Used by paths that have a per-org DB handle but no
// full DatabaseManager (e.g. login, activation for org users).
func NewPasswordQueryRepositoryFromDB(rawDB *sql.DB) *PasswordQueryRepository {
	return &PasswordQueryRepository{rawDB: rawDB, tableName: "user_auth_password"}
}

// NewAdminPasswordQueryRepository queries admin_auth_password in the main DB.
func NewAdminPasswordQueryRepository(manager *db.DatabaseManager) *PasswordQueryRepository {
	return &PasswordQueryRepository{DatabaseManager: manager, tableName: "admin_auth_password"}
}

// FindByUserID returns the most recent active password record for a user.
// The bool indicates whether a record exists.
func (repo *PasswordQueryRepository) FindByUserID(userID string) (*password_entity.Password, bool, error) {
	// Fast path: bare *sql.DB supplied (per-org DB handle). Scan manually
	// because DatabaseManager is nil in this code path.
	if repo.rawDB != nil {
		var pw password_entity.Password
		err := repo.rawDB.QueryRow(
			`SELECT user_id, password, created_at, is_active
			 FROM `+repo.tableName+`
			 WHERE user_id = ? AND is_active = 1
			 ORDER BY created_at DESC LIMIT 1`,
			userID,
		).Scan(&pw.UserId, &pw.Password, &pw.CreatedAt, &pw.IsActive)
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return &pw, true, nil
	}

	qb := repo.DatabaseManager.QueryBuilder().
		Select("user_id", "password", "created_at", "is_active").
		From(repo.tableName).
		Where("user_id = ?", userID).
		Where("is_active = ?", true).
		OrderBy("created_at DESC").
		Limit(1)
	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	return repo.hydrateFirstPassword(rows)
}

func (repo *PasswordQueryRepository) hydrateFirstPassword(rows *sql.Rows) (*password_entity.Password, bool, error) {
	var results []password_entity.Password
	err := repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)
	if err != nil {
		return nil, false, err
	}
	if len(results) == 0 {
		return nil, false, nil
	}
	return &results[0], true, nil
}
