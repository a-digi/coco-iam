package notification

// Scope builds the coconotification.SenderRef.Scope map this app's
// ScopedResolver expects: "org_id"/"app_id" keys, omitted when empty
// rather than present with an empty value, so an all-empty call
// returns nil (serializes as an absent `scope` field, and reads back
// identically to "no override" either way).
func Scope(orgID, appID string) map[string]string {
	if orgID == "" && appID == "" {
		return nil
	}
	out := make(map[string]string, 2)
	if orgID != "" {
		out["org_id"] = orgID
	}
	if appID != "" {
		out["app_id"] = appID
	}
	return out
}
