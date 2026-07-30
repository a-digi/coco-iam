// Package query is the read-only half of the admin login-attempt
// repository — admin_login.db (a separate database from the main
// one), admin_login_attempts table.
package query

import (
	"fmt"
	"time"

	loginlog_entity "github.com/a-digi/coco-iam/src/admin/security/loginlog/entity"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// AdminLoginQueryRepo reads through a *dbhandle.Handle rather than a
// raw *sql.DB, so the admin login-log page keeps reading the live
// generation even across the archiver rotating admin_login.db out
// from under it — see plan/ip-attacks-db-archiving/plan.md (the same
// mechanism, reused — see plan/login-audit-log/plan.md Step 1).
type AdminLoginQueryRepo struct {
	handle *dbhandle.Handle
}

func NewAdminLoginQueryRepo(handle *dbhandle.Handle) *AdminLoginQueryRepo {
	return &AdminLoginQueryRepo{handle: handle}
}

// ListFilter narrows ListAttempts/CountAttempts — every string field
// is an exact match, empty/nil means "don't filter on this". From/To
// must already be formatted as admin_login_attempts.created_at's own
// storage layout (see parseTimeFilter in the handler package) — this
// repo does no parsing of its own, it only builds SQL.
type ListFilter struct {
	Username    string
	AdminUserID string
	Success     *bool
	IP          string
	From        string
	To          string
	Limit       int
	Offset      int
}

func (f ListFilter) whereClause() (string, []interface{}) {
	clause := " WHERE 1=1"
	var args []interface{}
	if f.Username != "" {
		clause += " AND username = ?"
		args = append(args, f.Username)
	}
	if f.AdminUserID != "" {
		clause += " AND admin_user_id = ?"
		args = append(args, f.AdminUserID)
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
func (r *AdminLoginQueryRepo) ListAttempts(filter ListFilter) ([]loginlog_entity.AdminLoginAttempt, error) {
	where, args := filter.whereClause()
	q := `SELECT id, COALESCE(admin_user_id, ''), username, success, COALESCE(failure_reason, ''), ip, COALESCE(user_agent, ''), created_at, COALESCE(geoip_info, '')
	      FROM admin_login_attempts` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.handle.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("admin-login-attempt: list: %w", err)
	}
	defer rows.Close()

	var out []loginlog_entity.AdminLoginAttempt
	for rows.Next() {
		var a loginlog_entity.AdminLoginAttempt
		var successInt int
		if err := rows.Scan(&a.ID, &a.AdminUserID, &a.Username, &successInt, &a.FailureReason, &a.IP, &a.UserAgent, &a.CreatedAt, &a.GeoIPInfo); err != nil {
			return nil, fmt.Errorf("admin-login-attempt: scan: %w", err)
		}
		a.Success = successInt == 1
		a.CreatedAt = normalizeTimestamp(a.CreatedAt)
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin-login-attempt: rows: %w", err)
	}
	return out, nil
}

// CountAttempts returns how many rows match filter, ignoring
// filter.Limit/Offset — for the list endpoint's pagination total.
func (r *AdminLoginQueryRepo) CountAttempts(filter ListFilter) (int, error) {
	where, args := filter.whereClause()
	var n int
	err := r.handle.DB().QueryRow(`SELECT COUNT(*) FROM admin_login_attempts`+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("admin-login-attempt: count: %w", err)
	}
	return n, nil
}

// CountRecentFailures returns how many failed attempts ip has made
// since since — the failed-login ban-rule check's own query, backed
// by admin_login_attempts_ip_success_idx (ip, success, created_at) so
// this stays cheap regardless of how large the table grows. See
// plan/login-ban-rules/plan.md.
func (r *AdminLoginQueryRepo) CountRecentFailures(ip string, since time.Time) (int, error) {
	var n int
	err := r.handle.DB().QueryRow(
		`SELECT COUNT(*) FROM admin_login_attempts WHERE ip = ? AND success = 0 AND created_at >= ?`,
		ip, since.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("admin-login-attempt: count recent failures: %w", err)
	}
	return n, nil
}

// UsernameFailureSummary is one username a given IP has failed to log
// in as, aggregated across every recorded failure — see
// ListFailedUsernamesForIP.
type UsernameFailureSummary struct {
	Username      string
	AdminUserID   string
	Attempts      int
	LastAttemptAt string
}

// ListFailedUsernamesForIP returns every distinct username ip has
// ever failed a login attempt as, newest-first, with a per-username
// attempt count — used by the IP-bans page's "which accounts did
// this IP try" view. Not scoped to any particular time window
// (unlike CountRecentFailures) since a ban's own triggering window
// may since have changed, or the ban may be manual with no window at
// all — this shows the full history. See plan/ip-ban-accounts/plan.md.
func (r *AdminLoginQueryRepo) ListFailedUsernamesForIP(ip string) ([]UsernameFailureSummary, error) {
	rows, err := r.handle.DB().Query(
		`SELECT username, COALESCE(admin_user_id, ''), COUNT(*), MAX(created_at)
		 FROM admin_login_attempts WHERE ip = ? AND success = 0
		 GROUP BY username ORDER BY MAX(created_at) DESC`,
		ip,
	)
	if err != nil {
		return nil, fmt.Errorf("admin-login-attempt: list failed usernames: %w", err)
	}
	defer rows.Close()

	var out []UsernameFailureSummary
	for rows.Next() {
		var s UsernameFailureSummary
		if err := rows.Scan(&s.Username, &s.AdminUserID, &s.Attempts, &s.LastAttemptAt); err != nil {
			return nil, fmt.Errorf("admin-login-attempt: scan failed usernames: %w", err)
		}
		s.LastAttemptAt = normalizeTimestamp(s.LastAttemptAt)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin-login-attempt: rows failed usernames: %w", err)
	}
	return out, nil
}

// normalizeTimestamp rewrites created_at to a single consistent
// RFC3339 format — same defensive parse attacks_query.go's own
// normalizeTimestamp uses, in case a COALESCE-wrapped or raw scan ever
// disagrees on shape. Each package here keeps its own copy of this
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
