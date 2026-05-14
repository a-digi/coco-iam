package repository

import (
	"encoding/json"

	db "github.com/a-digi/coco-orm/orm"
)

type UserAclRepository struct {
	DatabaseManager *db.DatabaseManager
}

func NewUserAclRepository(manager *db.DatabaseManager) *UserAclRepository {
	return &UserAclRepository{
		DatabaseManager: manager,
	}
}

func (repo *UserAclRepository) FindUserScopes(userID string) ([]string, error) {
	scopesMap := make(map[string]bool)

	// 0. Check if user is super_admin
	var isSuperAdmin bool
	qbUser := repo.DatabaseManager.QueryBuilder().
		Select("is_super_admin").
		From("admin_users").
		Where("id = ?", userID).
		Where("is_active = ?", true)

	queryUser := qbUser.Build()
	err := repo.DatabaseManager.Connector.DB.QueryRow(queryUser.SQL, queryUser.Args...).Scan(&isSuperAdmin)
	if err == nil && isSuperAdmin {
		scopesMap["super:admin"] = true
	}

	// 1. Get direct user scopes from admin_acl
	qbDirect := repo.DatabaseManager.QueryBuilder().
		Select("roles").
		From("admin_acl").
		Where("user_id = ?", userID).
		Where("is_active = ?", true)

	queryDirect := qbDirect.Build()
	rowsDirect, err := repo.DatabaseManager.Connector.DB.Query(queryDirect.SQL, queryDirect.Args...)
	if err != nil {
		return nil, err
	}
	defer rowsDirect.Close()

	for rowsDirect.Next() {
		var rolesJSON []byte
		if err := rowsDirect.Scan(&rolesJSON); err != nil {
			return nil, err
		}
		var roles []string
		if err := json.Unmarshal(rolesJSON, &roles); err == nil {
			for _, role := range roles {
				scopesMap[role] = true
			}
		}
	}

	// 2. Get inherited scopes via admin groups
	qbGroup := repo.DatabaseManager.QueryBuilder().
		Select("admin_group_acl.roles").
		From("admin_group_members, admin_group_acl").
		Where("admin_group_members.group_id = admin_group_acl.group_id").
		Where("admin_group_members.user_id = ?", userID).
		Where("admin_group_members.is_active = ?", true).
		Where("admin_group_acl.is_active = ?", true)

	queryGroup := qbGroup.Build()
	rowsGroup, err := repo.DatabaseManager.Connector.DB.Query(queryGroup.SQL, queryGroup.Args...)
	if err != nil {
		return nil, err
	}
	defer rowsGroup.Close()

	for rowsGroup.Next() {
		var rolesJSON []byte
		if err := rowsGroup.Scan(&rolesJSON); err != nil {
			return nil, err
		}
		var roles []string
		if err := json.Unmarshal(rolesJSON, &roles); err == nil {
			for _, role := range roles {
				scopesMap[role] = true
			}
		}
	}

	var finalScopes []string
	for scope := range scopesMap {
		finalScopes = append(finalScopes, scope)
	}

	return finalScopes, nil
}
