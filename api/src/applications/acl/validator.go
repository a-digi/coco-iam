package acl

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-server/server/request"
)

// RolesValidator is an event listener for application_user_acl resources.
// It runs on POST/PATCH/PUT and performs two checks:
//
//  1. Role vocabulary — every role name in `roles` must correspond to a row in
//     `application_scopes` for the same `application_id`.
//  2. Organization membership — the user referenced by `user_id` must belong
//     to the organization that owns the application's workspace (each workspace
//     has exactly one `organization_id`).
//
// A failure in either check results in a 400 response with a message that
// identifies the reason.
type RolesValidator struct{}

type aclPeek struct {
	ApplicationID string          `json:"application_id"`
	UserID        string          `json:"user_id"`
	Roles         json.RawMessage `json:"roles"`
}

// BeforeExecution inspects the bound entity and rejects the request if either
// check fails.
func (v *RolesValidator) BeforeExecution(reqCtx request.RequestContext, entity interface{}) error {
	if entity == nil {
		return nil
	}

	bytes, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity for validation: %w", err)
	}

	var peek aclPeek
	if err := json.Unmarshal(bytes, &peek); err != nil {
		return fmt.Errorf("failed to read application ACL payload: %w", err)
	}

	if peek.ApplicationID == "" {
		return errors.New("application_id is required")
	}
	if peek.UserID == "" {
		return errors.New("user_id is required")
	}

	ctx := reqCtx.GetDI()

	// Resolve the application's organization via per-org routing index.
	bag, ok := ctx.(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return errors.New("DI context not keyed")
	}
	rawReg, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return errors.New("org user db registry not available")
	}
	reg, ok := rawReg.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		return errors.New("org user db registry type mismatch")
	}
	_, workspaceOrgID, err := orgrouter.OrgDBForApp(reg, peek.ApplicationID)
	if err != nil {
		return fmt.Errorf("application %q not found in routing index", peek.ApplicationID)
	}

	// (1) Role vocabulary check — scopes now live in the per-org DB.
	if len(peek.Roles) > 0 && string(peek.Roles) != "null" {
		var roles []string
		if err := json.Unmarshal(peek.Roles, &roles); err != nil {
			return errors.New("roles must be a JSON array of scope name strings")
		}
		orgDB, err := resolveOrgDB(reqCtx, workspaceOrgID)
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
	}

	// (2) Organization membership check via per-org DB.
	{
		memberDB, err := resolveOrgDB(reqCtx, workspaceOrgID)
		if err != nil {
			return fmt.Errorf("open org db for membership check: %w", err)
		}
		var exists int
		if err := memberDB.QueryRow(
			"SELECT COUNT(1) FROM users WHERE id = ? LIMIT 1",
			peek.UserID,
		).Scan(&exists); err != nil || exists == 0 {
			return fmt.Errorf("user %q is not a member of the application workspace's organization", peek.UserID)
		}
	}

	return nil
}

// resolveAppOrgID scans per-org DBs to find the org that owns appID.
func resolveAppOrgID(reqCtx request.RequestContext, appID string) (*sql.DB, string, error) {
	bag, ok := reqCtx.GetDI().(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return nil, "", errors.New("DI context not keyed")
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil, "", errors.New("org user db registry not available")
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		return nil, "", errors.New("org user db registry type mismatch")
	}
	return orgrouter.OrgDBForApp(reg, appID)
}

// resolveOrgDB opens the per-org users DB for the given org ID using
// the OrgUserDBRegistry stored in the DI bag.
func resolveOrgDB(reqCtx request.RequestContext, orgID string) (*sql.DB, error) {
	bag, ok := reqCtx.GetDI().(interface{ Get(string) (interface{}, bool) })
	if !ok {
		return nil, errors.New("DI context not keyed")
	}
	raw, ok := bag.Get(dbregistry.ContextBagKey)
	if !ok {
		return nil, errors.New("org user db registry not available")
	}
	reg, ok := raw.(*dbregistry.OrgUserDBRegistry)
	if !ok {
		return nil, errors.New("org user db registry type mismatch")
	}
	return orgrouter.ForOrg(reg, orgID)
}

// AfterExecution is a no-op.
func (v *RolesValidator) AfterExecution(_ request.RequestContext, _ interface{}) {
}
