package persistent

import (
	"github.com/a-digi/coco-iam/src/admin/users/entity"
	db "github.com/a-digi/coco-orm/orm"
	orm "github.com/a-digi/coco-orm/orm/orm"
)

type AdminUserPersistentRepository struct {
	DatabaseManager *db.DatabaseManager
}

func NewAdminUserPersistentRepository(manager *db.DatabaseManager) *AdminUserPersistentRepository {
	return &AdminUserPersistentRepository{
		DatabaseManager: manager,
	}
}

func (repo *AdminUserPersistentRepository) Insert(e *entity.User) error {
	builder := &orm.InsertObjectQueryBuilder{}
	query, args, err := builder.BuildFrom(e)

	if err != nil {
		return err
	}

	_, err = repo.DatabaseManager.Insert(query, args...)
	if err != nil {
		return err
	}

	return nil
}
