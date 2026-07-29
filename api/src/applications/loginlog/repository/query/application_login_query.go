// Package query is the read-only half of the per-application
// login-attempt repository — each application's own <slug>_login.db,
// application_login_attempts table.
package query

import (
	"fmt"
	"time"

	loginlog_entity "github.com/a-digi/coco-iam/src/applications/loginlog/entity"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ApplicationLoginQueryRepo reads through a *dbhandle.Handle rather
// than a raw *sql.DB, so it keeps reading the live generation even
// across the archiver rotating this application's <slug>_login.db out
// from under it — see plan/ip-attacks-db-archiving/plan.md (the same
// mechanism, reused — see plan/login-audit-log/plan.md Step 1).
type ApplicationLoginQueryRepo struct {
	handle *dbhandle.Handle
}

func NewApplicationLoginQueryRepo(handle *dbhandle.Handle) *ApplicationLoginQueryRepo {
	return &ApplicationLoginQueryRepo{handle: handle}
}

// ListFilter narrows ListAttempts/CountAttempts — every string field
// is an exact match, empty/nil means "don't filter on this". From/To
// must already be formatted as
// application_login_attempts.created_at's own storage layout (see
// parseTimeFilter in the handler package) — this repo does no parsing
// of its own, it only builds SQL.
type ListFilter struct {
	Username          string
	ApplicationUserID string
	Success           *bool
	IP                string
	From              string
	To                string
	Limit             int
	Offset            int
}

func (f ListFilter) whereClause() (string, []interface{}) {
	clause := " WHERE 1=1"
	var args []interface{}
	if f.Username != "" {
		clause += " AND username = ?"
		args = append(args, f.Username)
	}
	if f.ApplicationUserID != "" {
		clause += " AND application_user_id = ?"
		args = append(args, f.ApplicationUserID)
	}
	if f.Success != nil {
		clause += " AND success = ?"
		if *f.Success {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.IP != "" {
		clause += " AND ip = ?"
		args = append(args, f.IP)
	}
	if f.From != "" {
		clause += " AND created_at >= ?"
		args = append(args, f.From)
	}
	if f.To != "" {
		clause += " AND created_at <= ?"
		args = append(args, f.To)
	}
	return clause, args
}

// ListAttempts returns login attempts newest-first, filtered and
// paginated per filter.
func (r *ApplicationLoginQueryRepo) ListAttempts(filter ListFilter) ([]loginlog_entity.ApplicationLoginAttempt, error) {
	where, args := filter.whereClause()
	q := `SELECT id, COALESCE(application_user_id, ''), username, success, COALESCE(failure_reason, ''), ip, COALESCE(user_agent, ''), created_at
	      FROM application_login_attempts` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.handle.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("application-login-attempt: list: %w", err)
	}
	defer rows.Close()

	var out []loginlog_entity.ApplicationLoginAttempt
	for rows.Next() {
		var a loginlog_entity.ApplicationLoginAttempt
		var successInt int
		if err := rows.Scan(&a.ID, &a.ApplicationUserID, &a.Username, &successInt, &a.FailureReason, &a.IP, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("application-login-attempt: scan: %w", err)
		}
		a.Success = successInt == 1
		a.CreatedAt = normalizeTimestamp(a.CreatedAt)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("application-login-attempt: rows: %w", err)
	}
	return out, nil
}

// CountAttempts returns how many rows match filter, ignoring
// filter.Limit/Offset — for the list endpoint's pagination total.
func (r *ApplicationLoginQueryRepo) CountAttempts(filter ListFilter) (int, error) {
	where, args := filter.whereClause()
	var n int
	err := r.handle.DB().QueryRow(`SELECT COUNT(*) FROM application_login_attempts`+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("application-login-attempt: count: %w", err)
	}
	return n, nil
}

// CountRecentFailures returns how many failed attempts ip has made
// since since — the failed-login ban-rule check's own query, backed
// by application_login_attempts_ip_success_idx (ip, success,
// created_at) so this stays cheap regardless of how large the table
// grows. See plan/login-ban-rules/plan.md.
func (r *ApplicationLoginQueryRepo) CountRecentFailures(ip string, since time.Time) (int, error) {
	var n int
	err := r.handle.DB().QueryRow(
		`SELECT COUNT(*) FROM application_login_attempts WHERE ip = ? AND success = 0 AND created_at >= ?`,
		ip, since.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("application-login-attempt: count recent failures: %w", err)
	}
	return n, nil
}

// normalizeTimestamp rewrites created_at to a single consistent
// RFC3339 format — same defensive parse the admin-login/ip-attacks
// query repos use. Each package here keeps its own copy of this
// helper rather than sharing one, the established convention.
func normalizeTimestamp(s string) string {
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}
