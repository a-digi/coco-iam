package acl

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/a-digi/coco-server/server/request"
)

// GroupRolesValidator is an event listener for application_group_acl resources.
// It runs on POST/PATCH/PUT and performs the role-vocabulary check only:
// every role name in `roles` must correspond to an active row in
// `application_scopes` for the same `application_id`.
//
// Organization-membership checks are not applicable here (groups are
// org-scoped by construction); keeping the check lighter avoids duplicating
// state checks that the user-ACL path already enforces.
type GroupRolesValidator struct{}

type groupAclPeek struct {
	ApplicationID string          `json:"application_id"`
	GroupID       string          `json:"group_id"`
	Roles         json.RawMessage `json:"roles"`
}

func (v *GroupRolesValidator) BeforeExecution(reqCtx request.RequestContext, entity interface{}) error {
	if entity == nil {
		return nil
	}

	bytes, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity for validation: %w", err)
	}

	var peek groupAclPeek
	if err := json.Unmarshal(bytes, &peek); err != nil {
		return fmt.Errorf("failed to read application group ACL payload: %w", err)
	}

	if peek.ApplicationID == "" {
		return errors.New("application_id is required")
	}
	if peek.GroupID == "" {
		return errors.New("group_id is required")
	}

	if len(peek.Roles) == 0 || string(peek.Roles) == "null" {
		return nil
	}

	var roles []string
	if err := json.Unmarshal(peek.Roles, &roles); err != nil {
		return errors.New("roles must be a JSON array of scope name strings")
	}

	_, workspaceOrgID, err := resolveAppOrgID(reqCtx, peek.ApplicationID)
	if err != nil {
		return fmt.Errorf("application %q not found in routing index", peek.ApplicationID)
	}
	orgID := workspaceOrgID

	orgDB, err := resolveOrgDB(reqCtx, orgID)
	if err != nil {
		return fmt.Errorf("open org db for scope check: %w", err)
	}

	for _, scopeID := range roles {
		var exists int
		if err := orgDB.QueryRow(
			"SELECT COUNT(1) FROM application_scopes WHERE application_id = ? AND scope_id = ? AND is_active = TRUE",
			peek.ApplicationID, scopeID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("failed to look up application scope %q: %w", scopeID, err)
		}
		if exists == 0 {
			return fmt.Errorf("scope %q is not defined for application %q", scopeID, peek.ApplicationID)
		}
	}

	return nil
}

func (v *GroupRolesValidator) AfterExecution(_ request.RequestContext, _ interface{}) {
}
