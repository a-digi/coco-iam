package cleanup

// Payload is the body of a single `application-user-cleanup` task. It identifies
// the user whose application ACLs should be re-evaluated.
// OrgID is set when the task is enqueued by the custom org-user delete handler;
// the consumer uses it to open the per-org DB directly without needing the
// user_org_index routing table (which is already deleted by that point).
type Payload struct {
	UserID string `json:"user_id"`
	OrgID  string `json:"org_id,omitempty"`
}
