package persistent

import (
	"database/sql"
	"testing"
	"time"

	attacks_query "github.com/a-digi/coco-iam/src/admin/security/attacks/repository/query"
	_ "github.com/mattn/go-sqlite3"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// freshDB opens an in-memory SQLite DB with the ip_attacks/
// ip_attack_targets/db_meta schema — mirrors
// api/config/db/ip_attacks_migrations/001_initial.sql, 002_db_meta.sql,
// and 004_target_body_capture.sql.
func freshDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE ip_attacks (
		    id           TEXT NOT NULL CONSTRAINT ip_attacks_pk PRIMARY KEY,
		    ip           TEXT NOT NULL,
		    tier         TEXT NOT NULL,
		    started_at   DATETIME NOT NULL,
		    last_seen_at DATETIME NOT NULL,
		    ended_at     DATETIME,
		    hit_count    INTEGER NOT NULL DEFAULT 0,
		    ban_count    INTEGER NOT NULL DEFAULT 1,
		    origin_hint  TEXT,
		    geoip_info   TEXT
		);
		CREATE TABLE ip_attack_targets (
		    id          TEXT NOT NULL CONSTRAINT ip_attack_targets_pk PRIMARY KEY,
		    attack_id   TEXT NOT NULL CONSTRAINT ip_attack_targets_attack_fk REFERENCES ip_attacks (id),
		    path        TEXT NOT NULL,
		    method      TEXT NOT NULL,
		    hit_count   INTEGER NOT NULL DEFAULT 0,
		    body_sample TEXT
		);
		CREATE UNIQUE INDEX ip_attack_targets_unique_idx ON ip_attack_targets (attack_id, path, method);
		CREATE TABLE db_meta (key TEXT NOT NULL PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO db_meta (key, value) VALUES ('entry_count', '0');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

// mustHandle wraps db in a *dbhandle.Handle, the type
// AttackPersistentRepo actually takes — see
// plan/ip-attacks-db-archiving/plan.md.
func mustHandle(t *testing.T, db *sql.DB) *dbhandle.Handle {
	t.Helper()
	h, err := dbhandle.New(db)
	if err != nil {
		t.Fatalf("dbhandle.New() error = %v", err)
	}
	return h
}

func TestCreateAttack_ThenFindable(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateAttack("attack-1", "203.0.113.7", "sensitive", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}

	a, err := query.FindAttack("attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if a.IP != "203.0.113.7" || a.Tier != "sensitive" {
		t.Fatalf("attack = %+v, unexpected", a)
	}
	if a.HitCount != 0 || a.BanCount != 1 {
		t.Fatalf("attack initial counts = hit=%d ban=%d, want hit=0 ban=1", a.HitCount, a.BanCount)
	}
	if a.EndedAt != "" {
		t.Fatalf("EndedAt = %q, want empty for a fresh episode", a.EndedAt)
	}
	if a.OriginHint != "" {
		t.Fatalf("OriginHint = %q, want empty when nil was passed to CreateAttack", a.OriginHint)
	}
}

func TestCreateAttack_StoresOriginHintWhenProvided(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	hint := `{"x_forwarded_for":"198.51.100.9","host":"coco-iam.example.com"}`
	if err := persist.CreateAttack("attack-1", "127.0.0.1", "unmatched", time.Now(), &hint, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}

	a, err := query.FindAttack("attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if a.OriginHint != hint {
		t.Fatalf("OriginHint = %q, want %q", a.OriginHint, hint)
	}
}

func TestCreateAttack_StoresGeoIPInfoWhenProvided(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	geoInfo := `{"country_code":"DE","country":"Germany","asn":3320,"as_org":"Deutsche Telekom AG"}`
	if err := persist.CreateAttack("attack-1", "203.0.113.7", "global", time.Now(), nil, &geoInfo); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}

	a, err := query.FindAttack("attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if a.GeoIPInfo != geoInfo {
		t.Fatalf("GeoIPInfo = %q, want %q", a.GeoIPInfo, geoInfo)
	}

	// Confirm ListAttacks also carries geoip_info — unlike origin_hint,
	// this field is populated on the list view too.
	all, err := query.ListAttacks(attacks_query.ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListAttacks() error = %v", err)
	}
	if len(all) != 1 || all[0].GeoIPInfo != geoInfo {
		t.Fatalf("ListAttacks() = %+v, want a single entry with GeoIPInfo = %q", all, geoInfo)
	}
}

func TestUpdateAttackCounts_FlushesLatestTotals(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateAttack("attack-1", "203.0.113.7", "global", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.UpdateAttackCounts("attack-1", 42, 3, now.Add(time.Minute)); err != nil {
		t.Fatalf("UpdateAttackCounts() error = %v", err)
	}

	a, err := query.FindAttack("attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if a.HitCount != 42 || a.BanCount != 3 {
		t.Fatalf("attack counts = hit=%d ban=%d, want hit=42 ban=3", a.HitCount, a.BanCount)
	}
}

func TestCloseAttack_SetsEndedAt(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateAttack("attack-1", "203.0.113.7", "global", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CloseAttack("attack-1", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("CloseAttack() error = %v", err)
	}

	a, err := query.FindAttack("attack-1")
	if err != nil {
		t.Fatalf("FindAttack() error = %v", err)
	}
	if a.EndedAt == "" {
		t.Fatal("EndedAt should be set after CloseAttack")
	}
}

func TestCloseAllOpen_ClosesOnlyOpenRows(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	now := time.Now()
	if err := persist.CreateAttack("open-1", "1.1.1.1", "global", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CreateAttack("open-2", "2.2.2.2", "sensitive", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CreateAttack("already-closed", "3.3.3.3", "global", now, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CloseAttack("already-closed", now); err != nil {
		t.Fatalf("CloseAttack() error = %v", err)
	}

	n, err := persist.CloseAllOpen()
	if err != nil {
		t.Fatalf("CloseAllOpen() error = %v", err)
	}
	if n != 2 {
		t.Fatalf("CloseAllOpen() closed %d rows, want 2", n)
	}

	for _, id := range []string{"open-1", "open-2", "already-closed"} {
		a, err := query.FindAttack(id)
		if err != nil {
			t.Fatalf("FindAttack(%s) error = %v", id, err)
		}
		if a.EndedAt == "" {
			t.Fatalf("attack %s should be closed", id)
		}
	}
}

func TestSetTargetCount_CreatesThenUpdatesInPlace(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	if err := persist.CreateAttack("attack-1", "203.0.113.7", "sensitive", time.Now(), nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if got := persist.handle.EntryCount(); got != 1 {
		t.Fatalf("EntryCount() after CreateAttack = %d, want 1", got)
	}

	if err := persist.SetTargetCount("attack-1", "/admin/oauth/authenticate", "POST", 5, nil); err != nil {
		t.Fatalf("SetTargetCount() error = %v", err)
	}
	if got := persist.handle.EntryCount(); got != 2 {
		t.Fatalf("EntryCount() after the first SetTargetCount (a new row) = %d, want 2", got)
	}

	targets, err := query.ListTargets("attack-1")
	if err != nil {
		t.Fatalf("ListTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0].HitCount != 5 {
		t.Fatalf("targets = %+v, want single entry with hit_count=5", targets)
	}

	// Setting again for the same (attack, path, method) must update in
	// place, not create a second row — and must not move the entry
	// counter, which only tracks rows, not writes.
	if err := persist.SetTargetCount("attack-1", "/admin/oauth/authenticate", "POST", 12, nil); err != nil {
		t.Fatalf("SetTargetCount() second call error = %v", err)
	}
	if got := persist.handle.EntryCount(); got != 2 {
		t.Fatalf("EntryCount() after updating the same target in place = %d, want still 2", got)
	}
	targets, err = query.ListTargets("attack-1")
	if err != nil {
		t.Fatalf("ListTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0].HitCount != 12 {
		t.Fatalf("targets after update = %+v, want single entry with hit_count=12", targets)
	}
}

func TestSetTargetCount_BodySampleWrittenOnceAndNeverOverwritten(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	if err := persist.CreateAttack("attack-1", "203.0.113.7", "sensitive", time.Now(), nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}

	first := `{"email":"a@x.com","password":"[REDACTED]"}`
	if err := persist.SetTargetCount("attack-1", "/admin/oauth/authenticate", "POST", 1, &first); err != nil {
		t.Fatalf("SetTargetCount() error = %v", err)
	}
	targets, err := query.ListTargets("attack-1")
	if err != nil {
		t.Fatalf("ListTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0].BodySample == nil || *targets[0].BodySample != first {
		t.Fatalf("targets = %+v, want body_sample = %q", targets, first)
	}

	// A later flush for the same target must not overwrite the
	// already-stored sample with a different one, or with nil.
	second := `{"email":"b@x.com","password":"[REDACTED]"}`
	if err := persist.SetTargetCount("attack-1", "/admin/oauth/authenticate", "POST", 2, &second); err != nil {
		t.Fatalf("SetTargetCount() second call error = %v", err)
	}
	targets, err = query.ListTargets("attack-1")
	if err != nil {
		t.Fatalf("ListTargets() error = %v", err)
	}
	if len(targets) != 1 || targets[0].BodySample == nil || *targets[0].BodySample != first {
		t.Fatalf("targets after second flush = %+v, want body_sample still = %q", targets, first)
	}
	if targets[0].HitCount != 2 {
		t.Fatalf("targets[0].HitCount = %d, want 2 (hit_count must still update)", targets[0].HitCount)
	}
}

func TestSetTargetCount_DifferentEndpointsAreSeparateRows(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	if err := persist.CreateAttack("attack-1", "203.0.113.7", "global", time.Now(), nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.SetTargetCount("attack-1", "/a", "GET", 3, nil); err != nil {
		t.Fatalf("SetTargetCount() error = %v", err)
	}
	if err := persist.SetTargetCount("attack-1", "/b", "POST", 7, nil); err != nil {
		t.Fatalf("SetTargetCount() error = %v", err)
	}

	targets, err := query.ListTargets("attack-1")
	if err != nil {
		t.Fatalf("ListTargets() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want 2 distinct endpoints", targets)
	}
}

func TestListAttacks_NewestFirstAndPaginated(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	base := time.Now()
	if err := persist.CreateAttack("older", "1.1.1.1", "global", base, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CreateAttack("newer", "2.2.2.2", "global", base.Add(time.Minute), nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}

	all, err := query.ListAttacks(attacks_query.ListFilter{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("ListAttacks() error = %v", err)
	}
	if len(all) != 2 || all[0].ID != "newer" || all[1].ID != "older" {
		t.Fatalf("ListAttacks() = %+v, want [newer, older]", all)
	}

	page, err := query.ListAttacks(attacks_query.ListFilter{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListAttacks(limit=1,offset=1) error = %v", err)
	}
	if len(page) != 1 || page[0].ID != "older" {
		t.Fatalf("ListAttacks(limit=1,offset=1) = %+v, want [older]", page)
	}

	total, err := query.CountAttacks(attacks_query.ListFilter{})
	if err != nil {
		t.Fatalf("CountAttacks() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("CountAttacks() = %d, want 2", total)
	}
}

func TestListAttacks_FiltersByIPTierAndActiveOnly(t *testing.T) {
	db := freshDB(t)
	handle := mustHandle(t, db)
	persist := NewAttackPersistentRepo(handle)
	query := attacks_query.NewAttackQueryRepo(handle)

	base := time.Now()
	if err := persist.CreateAttack("a", "1.1.1.1", "global", base, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CreateAttack("b", "1.1.1.1", "sensitive", base, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CreateAttack("c", "2.2.2.2", "global", base, nil, nil); err != nil {
		t.Fatalf("CreateAttack() error = %v", err)
	}
	if err := persist.CloseAttack("a", base); err != nil {
		t.Fatalf("CloseAttack() error = %v", err)
	}

	byIP, err := query.ListAttacks(attacks_query.ListFilter{IP: "1.1.1.1", Limit: 50})
	if err != nil {
		t.Fatalf("ListAttacks(ip filter) error = %v", err)
	}
	if len(byIP) != 2 {
		t.Fatalf("ListAttacks(ip=1.1.1.1) = %+v, want 2 rows", byIP)
	}

	byTier, err := query.ListAttacks(attacks_query.ListFilter{Tier: "sensitive", Limit: 50})
	if err != nil {
		t.Fatalf("ListAttacks(tier filter) error = %v", err)
	}
	if len(byTier) != 1 || byTier[0].ID != "b" {
		t.Fatalf("ListAttacks(tier=sensitive) = %+v, want [b]", byTier)
	}

	active, err := query.ListAttacks(attacks_query.ListFilter{ActiveOnly: true, Limit: 50})
	if err != nil {
		t.Fatalf("ListAttacks(active filter) error = %v", err)
	}
	if len(active) != 2 { // b and c are still open; a was closed
		t.Fatalf("ListAttacks(active=true) = %+v, want 2 open rows", active)
	}

	count, err := query.CountAttacks(attacks_query.ListFilter{IP: "1.1.1.1"})
	if err != nil {
		t.Fatalf("CountAttacks(ip filter) error = %v", err)
	}
	if count != 2 {
		t.Fatalf("CountAttacks(ip=1.1.1.1) = %d, want 2", count)
	}
}

func TestFindAttack_NotFound(t *testing.T) {
	db := freshDB(t)
	query := attacks_query.NewAttackQueryRepo(mustHandle(t, db))
	if _, err := query.FindAttack("nope"); err != attacks_query.ErrNotFound {
		t.Fatalf("FindAttack() error = %v, want ErrNotFound", err)
	}
}
