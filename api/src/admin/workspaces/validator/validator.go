// Package validator hosts BeforeExecution listeners for the generic
// workspace resource. Runs on POST to enforce the one-org-per-
// workspace contract (required + existing organization).
//
// PATCH/PUT immutability is a frontend convention for v1: the edit
// form simply never sends `organization_id`, so the existing value
// is preserved by the generic updater. A future coco-lift release
// that invokes listeners on patch/put can tighten this at the
// backend too.
package validator

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/a-digi/coco-server/server/request"
)

type CreateValidator struct{}

type workspacePeek struct {
	OrganizationID string `json:"organization_id"`
}

// BeforeExecution fires on POST /workspaces. Rejects the request if
// organization_id is missing or points at a non-existent org.
func (v *CreateValidator) BeforeExecution(reqCtx request.RequestContext, entity interface{}) error {
	if entity == nil {
		return errors.New("workspace payload is empty")
	}
	raw, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("workspace validator: marshal entity: %w", err)
	}
	var peek workspacePeek
	if err := json.Unmarshal(raw, &peek); err != nil {
		return fmt.Errorf("workspace validator: parse payload: %w", err)
	}
	if peek.OrganizationID == "" {
		return errors.New("organization_id is required")
	}

	manager := reqCtx.GetDI().GetDatabaseManager()
	if manager == nil || manager.Connector == nil || manager.Connector.DB == nil {
		return errors.New("workspace validator: database unavailable")
	}
	var exists int
	err = manager.Connector.DB.QueryRow(
		`SELECT 1 FROM organization WHERE id = ? LIMIT 1`, peek.OrganizationID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("organization_id %q does not exist", peek.OrganizationID)
	}
	return nil
}

// AfterExecution is unused but required by the listener interface.
func (v *CreateValidator) AfterExecution(reqCtx request.RequestContext, entity interface{}) {}
