package authentication

import (
	"database/sql"
	"encoding/json"
)

// LoadAllUserScopes aggregates ACL roles from all three inheritance sources
// for an organisation user and returns a deduplicated slice (first-seen order):
//
//  1. application_user_acl   — per-app direct grant  (app-scoped)
//  2. organization_user_acl  — org-level direct grant (org-scoped)
//  3. organization_group_acl — roles inherited via group membership (org-scoped)
//
// Any source that fails or returns no rows contributes an empty set; the
// function never blocks login on a lookup failure.
func LoadAllUserScopes(db *sql.DB, appID, userID string) []string {
	seen := make(map[string]struct{})
	var out []string

	appendRoles := func(raw []byte) {
		if len(raw) == 0 {
			return
		}
		var roles []string
		if err := json.Unmarshal(raw, &roles); err != nil {
			return
		}
		for _, r := range roles {
			if _, ok := seen[r]; !ok {
				seen[r] = struct{}{}
				out = append(out, r)
			}
		}
	}

	// Source 1: application_user_acl (per app+user).
	var raw1 []byte
	_ = db.QueryRow(
		`SELECT roles FROM application_user_acl
		 WHERE application_id = ? AND user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		appID, userID,
	).Scan(&raw1)
	appendRoles(raw1)

	// Source 2: organization_user_acl (direct org-level grant).
	var raw2 []byte
	_ = db.QueryRow(
		`SELECT roles FROM organization_user_acl
		 WHERE user_id = ? AND is_active = TRUE
		 LIMIT 1`,
		userID,
	).Scan(&raw2)
	appendRoles(raw2)

	// Source 3: group inheritance via user_group_members.
	rows, err := db.Query(
		`SELECT oga.roles
		 FROM organization_group_acl oga
		 JOIN user_group_members ugm ON ugm.group_id = oga.group_id
		 WHERE ugm.user_id = ? AND ugm.is_active = TRUE AND oga.is_active = TRUE`,
		userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var raw3 []byte
			if rows.Scan(&raw3) == nil {
				appendRoles(raw3)
			}
		}
	}

	return out
}
