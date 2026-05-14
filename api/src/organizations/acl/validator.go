package acl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/a-digi/coco-server/server/request"
)

const scopePrefix = "organizations:"

// OrganizationScopeValidator is an event listener that rejects role assignments
// containing any scope that is not under the `organizations:` namespace.
//
// It satisfies the PostEventListener, PatchEventListener, and PutEventListener
// interfaces so a single instance can be registered on all three method hooks
// of an ACL resource.
type OrganizationScopeValidator struct{}

// BeforeExecution inspects the bound entity, extracts the `roles` field via
// reflection-like JSON round-trip, and returns an error if any role string
// does not start with `organizations:`.
func (v *OrganizationScopeValidator) BeforeExecution(_ request.RequestContext, entity interface{}) error {
	if entity == nil {
		return nil
	}

	bytes, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity for scope validation: %w", err)
	}

	var peek struct {
		Roles json.RawMessage `json:"roles"`
	}
	if err := json.Unmarshal(bytes, &peek); err != nil {
		return fmt.Errorf("failed to read roles field: %w", err)
	}

	if len(peek.Roles) == 0 || string(peek.Roles) == "null" {
		return nil
	}

	var roles []string
	if err := json.Unmarshal(peek.Roles, &roles); err != nil {
		return errors.New("roles must be a JSON array of scope strings")
	}

	for _, role := range roles {
		if !strings.HasPrefix(role, scopePrefix) {
			return fmt.Errorf("scope %q is not allowed; only scopes under %q may be assigned", role, scopePrefix)
		}
	}

	return nil
}

// AfterExecution is a no-op.
func (v *OrganizationScopeValidator) AfterExecution(_ request.RequestContext, _ interface{}) {
}
