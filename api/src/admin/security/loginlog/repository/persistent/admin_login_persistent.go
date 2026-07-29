// Package persistent is the write half of the admin login-attempt
// repository — admin_login.db (a separate database from the main
// one), admin_login_attempts table. See plan/login-audit-log/plan.md
// Step 3.
package persistent

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/a-digi/coco-iam/src/security/dbhandle"
)

// attemptTimeLayout matches admin_login_attempts.created_at's storage
// format — see this package's own ArchiveRecorder and dbarchive's
// sibling timeLayout constants; each package keeps its own copy
// rather than sharing one, the established convention in this
// codebase.
const attemptTimeLayout = "2006-01-02 15:04:05"

// AdminLoginPersistentRepo writes through a *dbhandle.Handle rather
// than a raw *sql.DB, so it keeps working across the archiver rotating
// admin_login.db out from under it mid-run — see
// plan/ip-attacks-db-archiving/plan.md (the same mechanism, reused —
// see plan/login-audit-log/plan.md Step 1). Every call increments the
// handle's entry counter on the same connection it just wrote to, so
// the two can never disagree.
type AdminLoginPersistentRepo struct {
	handle *dbhandle.Handle
}

func NewAdminLoginPersistentRepo(handle *dbhandle.Handle) *AdminLoginPersistentRepo {
	return &AdminLoginPersistentRepo{handle: handle}
}

// RecordAttempt inserts one row. adminUserID and failureReason are
// nullable — adminUserID is empty when the typed username never
// resolved to a real admin account; failureReason is empty on
// success. Best-effort by design: a login-log write must never block
// or fail the actual login it's recording, so every call site treats
// this method's error as log-and-continue, not a request failure.
func (r *AdminLoginPersistentRepo) RecordAttempt(adminUserID, username string, success bool, failureReason, ip, userAgent string) error {
	db := r.handle.DB()

	var adminUserIDArg, failureReasonArg interface{}
	if adminUserID != "" {
		adminUserIDArg = adminUserID
	}
	if failureReason != "" {
		failureReasonArg = failureReason
	}

	successInt := 0
	if success {
		successInt = 1
	}

	_, err := db.Exec(
		`INSERT INTO admin_login_attempts (id, admin_user_id, username, success, failure_reason, ip, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), adminUserIDArg, username, successInt, failureReasonArg, ip, userAgent,
		time.Now().UTC().Format(attemptTimeLayout),
	)
	if err != nil {
		return fmt.Errorf("admin-login-attempt: record: %w", err)
	}
	if _, err := r.handle.IncrementEntryCount(db, 1); err != nil {
		return fmt.Errorf("admin-login-attempt: record: %w", err)
	}
	return nil
}
