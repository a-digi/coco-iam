package acl

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/a-digi/coco-server/server/request"
)

// ScopeIDFormat is the canonical format for application scope identifiers:
// one or more colon-separated segments, each made of letters, digits,
// underscores or hyphens. Hyphens/digits are allowed so plugin scopes can
// embed a kebab-case plugin slug (coco-mda slugs match ^[a-z0-9-]+$), e.g.
// `plugin:platform-discovery:read`.
var ScopeIDFormat = regexp.MustCompile(`^[a-zA-Z0-9_-]+(:[a-zA-Z0-9_-]+)*$`)

// ScopeIDValidator is an event listener for application_scopes POST/PATCH/PUT.
// It validates that the `scope_id` field matches the canonical format. No
// content validation beyond the format; uniqueness is enforced by the DB's
// (application_id, scope_id) unique index.
type ScopeIDValidator struct{}

type scopePeek struct {
	ScopeID string `json:"scope_id"`
}

func (v *ScopeIDValidator) BeforeExecution(_ request.RequestContext, entity interface{}) error {
	if entity == nil {
		return nil
	}
	bytes, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}
	var p scopePeek
	if err := json.Unmarshal(bytes, &p); err != nil {
		return fmt.Errorf("failed to read scope payload: %w", err)
	}
	if p.ScopeID == "" {
		return errors.New("scope_id is required")
	}
	if !ScopeIDFormat.MatchString(p.ScopeID) {
		return fmt.Errorf("scope_id %q is invalid — only letters, digits, underscores, hyphens and colon separators are allowed (e.g. 'read', 'docs:write', 'plugin:platform-discovery:read')", p.ScopeID)
	}
	return nil
}

func (v *ScopeIDValidator) AfterExecution(_ request.RequestContext, _ interface{}) {
}
