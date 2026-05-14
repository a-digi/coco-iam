package orgpwnotify

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-logger/logger"
)

// publisher is the narrow interface OrgDetector needs from queue.Manager.
// queue.Manager satisfies this interface automatically.
type publisher interface {
	Publish(queueName string, payload interface{}) error
}

type OrgDetector struct {
	reg      *dbregistry.OrgUserDBRegistry
	orgStore *userrules.OrgStore
	queueMgr publisher
	log      logger.Logger
	interval time.Duration
}

func NewOrgDetector(reg *dbregistry.OrgUserDBRegistry, orgStore *userrules.OrgStore, mgr publisher, log logger.Logger) *OrgDetector {
	return &OrgDetector{
		reg:      reg,
		orgStore: orgStore,
		queueMgr: mgr,
		log:      log,
		interval: 24 * time.Hour,
	}
}

func (d *OrgDetector) Run(ctx context.Context) {
	d.scan(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.scan(ctx)
		}
	}
}

func (d *OrgDetector) scan(ctx context.Context) {
	orgIDs := d.reg.KnownOrgIDs()
	for _, orgID := range orgIDs {
		mgr, err := d.reg.For(orgID)
		if err != nil || mgr == nil || mgr.Connector == nil || mgr.Connector.DB == nil {
			d.log.Warning("orgpwnotify: cannot open db for org %s: %v", orgID, err)
			continue
		}
		d.scanOrg(ctx, orgID, mgr.Connector.DB)
	}
}

type orgUserRow struct {
	userID       string
	email        string
	username     string
	changedAtStr string
}

func (d *OrgDetector) scanOrg(ctx context.Context, orgID string, orgDB *sql.DB) {
	rs, err := d.orgStore.GetForOrg(orgID)
	if err != nil || rs.Password.ExpiryDays <= 0 || len(rs.Password.NotifyDays) == 0 {
		return
	}
	expiryDays := rs.Password.ExpiryDays
	notifyDays := rs.Password.NotifyDays

	rows, err := orgDB.QueryContext(ctx, `
		SELECT u.id, u.email, u.username, p.changed_at
		FROM users u
		JOIN user_auth_password p ON p.user_id = u.id AND p.is_active = TRUE
		WHERE u.is_active = TRUE AND p.changed_at IS NOT NULL`)
	if err != nil {
		d.log.Warning("orgpwnotify: scan query org %s: %v", orgID, err)
		return
	}
	var users []orgUserRow
	for rows.Next() {
		var r orgUserRow
		if err := rows.Scan(&r.userID, &r.email, &r.username, &r.changedAtStr); err != nil {
			continue
		}
		users = append(users, r)
	}
	rows.Close()

	now := time.Now().UTC()
	for _, r := range users {
		changedAt, err := parseTime(r.changedAtStr)
		if err != nil {
			continue
		}
		expiryDate := changedAt.Add(time.Duration(expiryDays) * 24 * time.Hour)

		for _, n := range notifyDays {
			if n <= 0 {
				continue
			}
			notifyAt := expiryDate.Add(-time.Duration(n) * 24 * time.Hour)
			if now.Before(notifyAt) {
				continue
			}
			if d.alreadySent(orgDB, r.userID, r.changedAtStr, n) {
				continue
			}
			p := Payload{
				OrgID:           orgID,
				UserID:          r.userID,
				Email:           r.email,
				Username:        r.username,
				DaysUntilExpiry: n,
				ExpiryDate:      expiryDate.Format("02 Jan 2006"),
			}
			if err := d.queueMgr.Publish(QueueName, p); err != nil {
				d.log.Warning("orgpwnotify: publish for user %s org %s days %d: %v", r.userID, orgID, n, err)
				continue
			}
			d.recordSent(orgDB, r.userID, r.changedAtStr, n)
		}
	}
}

func (d *OrgDetector) alreadySent(orgDB *sql.DB, userID, changedAt string, daysBefore int) bool {
	var found int
	err := orgDB.QueryRow(
		`SELECT 1 FROM user_password_notify_log
		 WHERE user_id = ? AND password_changed_at = ? AND days_before = ? LIMIT 1`,
		userID, changedAt, daysBefore,
	).Scan(&found)
	return err == nil
}

func (d *OrgDetector) recordSent(orgDB *sql.DB, userID, changedAt string, daysBefore int) {
	id := newID()
	_, _ = orgDB.Exec(
		`INSERT OR IGNORE INTO user_password_notify_log (id, user_id, password_changed_at, days_before)
		 VALUES (?, ?, ?, ?)`,
		id, userID, changedAt, daysBefore,
	)
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	hx := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:])
}
