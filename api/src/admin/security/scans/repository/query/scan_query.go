// Package query is the read-only half of the port-scan-history
// repository — the scan_episodes table in ip-attacks.db.
package query

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	scans_entity "github.com/a-digi/coco-iam/src/admin/security/scans/entity"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// ErrNotFound signals no scan_episodes row exists for the given id.
var ErrNotFound = errors.New("scan-episode: not found")

// ScanQueryRepo reads through a *dbhandle.Handle rather than a raw
// *sql.DB, so the admin Scans page keeps reading the live generation
// across the archiver rotating ip-attacks.db out from under it — see
// plan/ip-attacks-db-archiving/plan.md.
type ScanQueryRepo struct {
	handle *dbhandle.Handle
}

func NewScanQueryRepo(handle *dbhandle.Handle) *ScanQueryRepo {
	return &ScanQueryRepo{handle: handle}
}

// ListFilter narrows ListScans/CountScans — IP is an exact match,
// empty means "don't filter on it".
type ListFilter struct {
	IP         string
	ActiveOnly bool // ended_at IS NULL
	Limit      int
	Offset     int
}

func (f ListFilter) whereClause() (string, []interface{}) {
	clause := " WHERE 1=1"
	var args []interface{}
	if f.IP != "" {
		clause += " AND ip = ?"
		args = append(args, f.IP)
	}
	if f.ActiveOnly {
		clause += " AND ended_at IS NULL"
	}
	return clause, args
}

// ListScans returns scan episodes newest-first, filtered and
// paginated per filter.
func (r *ScanQueryRepo) ListScans(filter ListFilter) ([]scans_entity.Scan, error) {
	where, args := filter.whereClause()
	q := `SELECT id, ip, started_at, last_seen_at, COALESCE(ended_at, ''), distinct_ports, hit_count, sample_ports
	      FROM scan_episodes` + where + ` ORDER BY started_at DESC LIMIT ? OFFSET ?`
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.handle.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("scan-episode: list: %w", err)
	}
	defer rows.Close()

	var out []scans_entity.Scan
	for rows.Next() {
		var s scans_entity.Scan
		if err := rows.Scan(&s.ID, &s.IP, &s.StartedAt, &s.LastSeenAt, &s.EndedAt, &s.DistinctPorts, &s.HitCount, &s.SamplePorts); err != nil {
			return nil, fmt.Errorf("scan-episode: scan: %w", err)
		}
		normalizeTimestamps(&s)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan-episode: rows: %w", err)
	}
	return out, nil
}

// CountScans returns how many rows match filter, ignoring
// filter.Limit/Offset — for the list endpoint's pagination total.
func (r *ScanQueryRepo) CountScans(filter ListFilter) (int, error) {
	where, args := filter.whereClause()
	var n int
	err := r.handle.DB().QueryRow(`SELECT COUNT(*) FROM scan_episodes`+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("scan-episode: count: %w", err)
	}
	return n, nil
}

// FindScan returns a single episode by id, or ErrNotFound.
func (r *ScanQueryRepo) FindScan(id string) (*scans_entity.Scan, error) {
	row := r.handle.DB().QueryRow(
		`SELECT id, ip, started_at, last_seen_at, COALESCE(ended_at, ''), distinct_ports, hit_count, sample_ports
		 FROM scan_episodes WHERE id = ?`,
		id,
	)
	var s scans_entity.Scan
	if err := row.Scan(&s.ID, &s.IP, &s.StartedAt, &s.LastSeenAt, &s.EndedAt, &s.DistinctPorts, &s.HitCount, &s.SamplePorts); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan-episode: find: %w", err)
	}
	normalizeTimestamps(&s)
	return &s, nil
}

// normalizeTimestamps rewrites StartedAt/LastSeenAt/EndedAt to a
// single consistent RFC3339 format — the same COALESCE-strips-driver-
// auto-conversion issue documented on attack_query.go's
// normalizeTimestamp.
func normalizeTimestamps(s *scans_entity.Scan) {
	s.StartedAt = normalizeTimestamp(s.StartedAt)
	s.LastSeenAt = normalizeTimestamp(s.LastSeenAt)
	s.EndedAt = normalizeTimestamp(s.EndedAt)
}

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
