package persistent

import (
	"github.com/a-digi/coco-iam/src/auth/database/entity"
	db "github.com/a-digi/coco-orm/orm"
	orm "github.com/a-digi/coco-orm/orm/orm"
)

type PasswordPersistentRepository struct {
	DatabaseManager *db.DatabaseManager
}

func NewPasswordPersistentRepository(manager *db.DatabaseManager) *PasswordPersistentRepository {
	return &PasswordPersistentRepository{
		DatabaseManager: manager,
	}
}

func (repo *PasswordPersistentRepository) Insert(e *entity.Password) error {
	builder := &orm.InsertObjectQueryBuilder{}
	query, args, err := builder.BuildFrom(e)
	if err != nil {
		return err
	}
	_, err = repo.DatabaseManager.Insert(query, args...)
	return err
}

func (repo *PasswordPersistentRepository) InsertAdmin(e *entity.AdminPassword) error {
	builder := &orm.InsertObjectQueryBuilder{}
	query, args, err := builder.BuildFrom(e)
	if err != nil {
		return err
	}
	_, err = repo.DatabaseManager.Insert(query, args...)
	return err
}
