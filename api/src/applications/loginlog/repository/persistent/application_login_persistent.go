package persistent

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// attemptTimeLayout matches application_login_attempts.created_at's
// own storage format — see this package's ArchiveRecorder (its
// timeLayout constant) and the admin-login/ip-attacks recorders'
// sibling constants; each package keeps its own copy rather than
// sharing one, the established convention in this codebase.
const attemptTimeLayout = "2006-01-02 15:04:05"

// ApplicationLoginPersistentRepo writes through a *dbhandle.Handle
// rather than a raw *sql.DB, so it keeps working across the archiver
// rotating this application's <slug>_login.db out from under it
// mid-run — see plan/ip-attacks-db-archiving/plan.md (the same
// mechanism, reused — see plan/login-audit-log/plan.md Step 1). Every
// call increments the handle's entry counter on the same connection
// it just wrote to, so the two can never disagree.
type ApplicationLoginPersistentRepo struct {
	handle *dbhandle.Handle
}

func NewApplicationLoginPersistentRepo(handle *dbhandle.Handle) *ApplicationLoginPersistentRepo {
	return &ApplicationLoginPersistentRepo{handle: handle}
}

// RecordAttempt inserts one row. applicationUserID and failureReason
// are nullable — applicationUserID is empty when the typed username
// never resolved to a real end-user account; failureReason is empty
// on success. geoIPInfo is a JSON-marshaled geoip.Info snapshot, empty
// when the IP was loopback/private, GeoIP had no coverage, or GeoIP
// is disabled — see plan/login-log-geoip/plan.md. Best-effort by
// design: a login-log write must never block or fail the actual
// login it's recording, so every call site treats this method's
// error as log-and-continue, not a request failure.
func (r *ApplicationLoginPersistentRepo) RecordAttempt(applicationUserID, username string, success bool, failureReason, ip, userAgent, geoIPInfo string) error {
	db := r.handle.DB()

	var userIDArg, failureReasonArg, geoIPInfoArg interface{}
	if applicationUserID != "" {
		userIDArg = applicationUserID
	}
	if failureReason != "" {
		failureReasonArg = failureReason
	}
	if geoIPInfo != "" {
		geoIPInfoArg = geoIPInfo
	}

	successInt := 0
	if success {
		successInt = 1
	}

	_, err := db.Exec(
		`INSERT INTO application_login_attempts (id, application_user_id, username, success, failure_reason, ip, user_agent, created_at, geoip_info)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userIDArg, username, successInt, failureReasonArg, ip, userAgent,
		time.Now().UTC().Format(attemptTimeLayout), geoIPInfoArg,
	)
	if err != nil {
		return fmt.Errorf("application-login-attempt: record: %w", err)
	}
	if _, err := r.handle.IncrementEntryCount(db, 1); err != nil {
		return fmt.Errorf("application-login-attempt: record: %w", err)
	}
	return nil
}
