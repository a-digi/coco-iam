package repository

import (
	"github.com/a-digi/coco-iam/src/xbrl/standard/entity"
	db "github.com/a-digi/coco-orm/orm"
	orm "github.com/a-digi/coco-orm/orm/orm"
)

type StandardRepository struct {
	DatabaseManager *db.DatabaseManager
}

func NewStandardRepository(manager *db.DatabaseManager) *StandardRepository {
	return &StandardRepository{
		DatabaseManager: manager,
	}
}

func (repo *StandardRepository) Insert(e *entity.Standard) error {
	builder := &orm.InsertObjectQueryBuilder{}
	query, args, err := builder.BuildFrom(e)

	if err != nil {
		return err
	}

	_, err = repo.DatabaseManager.Insert(query, args...)

	return err
}

func (repo *StandardRepository) FindByFilePath(filePath string) (*entity.Standard, error) {
	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "file_path", "created_at", "is_active").
		From("standard").
		Where("file_path = ?", filePath).
		Limit(1)
	query := qb.Build()
	row := repo.DatabaseManager.Connector.DB.QueryRow(query.SQL, query.Args...)
	var s entity.Standard
	err := row.Scan(&s.ID, &s.FilePath, &s.CreatedAt, &s.IsActive)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (repo *StandardRepository) FindByFileHash(fileHash string) (*entity.Standard, error) {
	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "file_path", "file_hash", "created_at", "is_active").
		From("standard").
		Where("file_hash = ?", fileHash).
		Limit(1)
	query := qb.Build()
	row := repo.DatabaseManager.Connector.DB.QueryRow(query.SQL, query.Args...)
	var s entity.Standard
	err := row.Scan(&s.ID, &s.FilePath, &s.FileHash, &s.CreatedAt, &s.IsActive)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (repo *StandardRepository) FindAll() ([]entity.Standard, error) {

	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "file_path", "file_hash", "created_at", "is_active").
		From("standard").
		OrderBy("created_at DESC")

	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []entity.Standard
	err = repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (repo *StandardRepository) FindByPagination(page int, limit int) ([]entity.Standard, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	qb := repo.DatabaseManager.QueryBuilder().
		Select("id", "file_path", "file_hash", "created_at", "is_active", "title", "version").
		From("standard").
		OrderBy("created_at DESC").
		Limit(limit).
		Offset(offset)

	query := qb.Build()

	rows, err := repo.DatabaseManager.Connector.DB.Query(query.SQL, query.Args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []entity.Standard
	err = repo.DatabaseManager.Hydrator.HydrateRows(rows, &results)

	if err != nil {
		return nil, err
	}

	return results, nil
}
