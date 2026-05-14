package db

import (
	"fmt"
	"strings"

	admin_creator "github.com/a-digi/coco-iam/src/admin/users"
	users_repo "github.com/a-digi/coco-iam/src/admin/users/repository/query"
	db "github.com/a-digi/coco-orm/orm"
)

func Install(manager *db.DatabaseManager) error {
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		return fmt.Errorf("DatabaseManager or Connector is nil")
	}

	return manager.SyncMigrations()
}

func EnsureHasSuperadmin(manager *db.DatabaseManager) error {
	if manager == nil {
		fmt.Printf("[EnsureHasSuperadmin] Manager is nil\n")
		return adminDoesNotExistError()
	}

	repo := users_repo.NewAdminUserQueryRepository(manager)
	user, noResults, err := repo.FindSuperAdmins()

	if err != nil {
		fmt.Printf("[EnsureHasSuperadmin] Query error: %v\n", err)
		return adminDoesNotExistError()
	}

	if noResults {
		fmt.Printf("[EnsureHasSuperadmin] No results returned by query.\n")
		return adminDoesNotExistError()
	}

	if user == nil {
		fmt.Printf("[EnsureHasSuperadmin] User pointer is nil despite noResults=false.\n")
		return adminDoesNotExistError()
	}

	return nil
}

func adminDoesNotExistError() error {
	return fmt.Errorf("No superadmin user found, use the admin-create argument to create one")
}

func AddSuperadminInteractive(manager *db.DatabaseManager) error {
	return fmt.Errorf("interactive mode disabled; provide <username> <email> <password> as CLI arguments")
}

func AddSuperadminWithArgs(manager *db.DatabaseManager, username, email, password string) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" {
		return fmt.Errorf("username, email and password must not be empty")
	}

	creator := admin_creator.NewAdminUserCreator(manager)
	_, err := creator.Create(username, email, password, true, true)
	if err != nil {
		return err
	}

	return nil
}
