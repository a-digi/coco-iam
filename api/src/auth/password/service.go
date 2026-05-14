// Package password implements the authenticated, self-service
// password-change flow. Distinct from `activation` (which handles new
// account onboarding via email token) — here the user proves identity
// with the *current* password and no email is involved.
package password

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	bcryptx "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	auth_db "github.com/a-digi/coco-iam/src/auth/database"
	auth_query "github.com/a-digi/coco-iam/src/auth/database/repository/query"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-orm/orm"
)

// ErrChangeFailed is the single error returned for any
// authentication-related failure (user not found, wrong current
// password). The HTTP layer translates this into a generic 400 so the
// endpoint can't be used to enumerate users or test passwords with a
// distinguishable response.
var ErrChangeFailed = errors.New("password: change failed")

// Service orchestrates the self-service password-change flow. Safe for
// concurrent use — every call opens its own short-lived queries.
type Service struct {
	dbm         *orm.DatabaseManager
	db          *sql.DB // main DB
	orgRegistry *dbregistry.OrgUserDBRegistry
	rules       *userrules.Store
}

// NewService binds a Service to the main DB manager, the per-org DB
// registry, and the user-rules store. Both orgRegistry and rules may be
// nil; the service degrades gracefully when they are absent.
func NewService(dbm *orm.DatabaseManager, orgRegistry *dbregistry.OrgUserDBRegistry, rules *userrules.Store) *Service {
	return &Service{dbm: dbm, db: dbm.Connector.DB, orgRegistry: orgRegistry, rules: rules}
}

// Verify checks that `currentPassword` matches the stored hash for
// `userID`. Returns ErrChangeFailed for any mismatch — missing user,
// inactive password row, or wrong password all collapse to the same
// sentinel so the HTTP layer leaks nothing.
func (s *Service) Verify(userID, currentPassword string) error {
	if strings.TrimSpace(userID) == "" || currentPassword == "" {
		return ErrChangeFailed
	}
	ok, err := s.authenticate(userID, currentPassword)
	if err != nil {
		return ErrChangeFailed
	}
	if !ok {
		return ErrChangeFailed
	}
	return nil
}

// Change verifies the old password and writes a new one. On success,
// clears `must_change_password` on whichever user table owns the ID.
// Rule violations surface verbatim (joined) so the UI can show them;
// auth errors (wrong current, unknown user) collapse to ErrChangeFailed.
func (s *Service) Change(userID, currentPassword, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("password: new password cannot be empty")
	}
	if newPassword == currentPassword {
		return errors.New("password: new password must differ from the current one")
	}

	// Run the configured rule set. Username/email context lets the
	// DisallowUsername / DisallowEmail rules do their job.
	if violations := s.validate(userID, newPassword); len(violations) > 0 {
		return errors.New(strings.Join(violations, " "))
	}

	if err := s.Verify(userID, currentPassword); err != nil {
		return err
	}

	newHash, err := bcryptx.HashPassword(newPassword, bcryptx.DefaultCost)
	if err != nil {
		return fmt.Errorf("password: hash new password: %w", err)
	}

	if err := s.upsertPassword(userID, newHash); err != nil {
		return err
	}
	if err := s.clearMustChangeFlag(userID); err != nil {
		// Not fatal — the password itself is changed. Surface to caller
		// so ops/logging captures it.
		return err
	}
	return nil
}

// --- helpers ---

// isAdminUser returns true when userID is found in admin_users.
func (s *Service) isAdminUser(userID string) bool {
	var exists int
	_ = s.db.QueryRow(
		`SELECT 1 FROM admin_users WHERE id = ? LIMIT 1`, userID,
	).Scan(&exists)
	return exists == 1
}

// orgDBForUser resolves the per-org DB for a regular user.
func (s *Service) orgDBForUser(userID string) (*sql.DB, error) {
	if s.orgRegistry == nil {
		return nil, fmt.Errorf("password: org db registry not available")
	}
	orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, userID)
	return orgDB, err
}

func (s *Service) authenticate(userID, plaintext string) (bool, error) {
	if s.isAdminUser(userID) {
		// Admin passwords live in admin_auth_password in the main DB.
		pwrepo := auth_query.NewAdminPasswordQueryRepository(s.dbm)
		authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
		return authenticator.Verify(userID, plaintext)
	}
	// Regular user — look up the per-org DB.
	orgDB, err := s.orgDBForUser(userID)
	if err != nil {
		return false, err
	}
	pwrepo := auth_query.NewPasswordQueryRepositoryFromDB(orgDB)
	authenticator := auth_db.NewPasswordAuthenticator(pwrepo)
	return authenticator.Verify(userID, plaintext)
}

// validate loads the rule set that applies to `userID` and returns any
// violations for the proposed new password. Username + email are
// looked up so the DisallowUsername / DisallowEmail rules can work.
func (s *Service) validate(userID, newPassword string) []string {
	rs := userrules.Defaults()
	if s.rules != nil {
		if loaded, err := s.rules.GetForUser(userID); err == nil {
			rs = loaded
		}
	}
	username, email := s.lookupIdentity(userID)
	return userrules.ValidatePassword(rs.Password, userrules.Input{
		Username: username,
		Email:    email,
		Password: newPassword,
	})
}

// lookupIdentity returns (username, email) for a user id, checking
// admin_users in the main DB first, then the per-org DB for regular users.
func (s *Service) lookupIdentity(userID string) (string, string) {
	var u, e string
	if err := s.db.QueryRow(
		`SELECT username, email FROM admin_users WHERE id = ?`, userID,
	).Scan(&u, &e); err == nil {
		return u, e
	}
	orgDB, err := s.orgDBForUser(userID)
	if err != nil {
		return "", ""
	}
	if err := orgDB.QueryRow(
		`SELECT username, email FROM users WHERE id = ?`, userID,
	).Scan(&u, &e); err == nil {
		return u, e
	}
	return "", ""
}

// upsertPassword replaces (or inserts) the user_auth_password row for
// `userID`. Admin passwords stay in the main DB; regular users go to
// the per-org DB.
func (s *Service) upsertPassword(userID, hash string) error {
	db := s.db
	if !s.isAdminUser(userID) {
		orgDB, err := s.orgDBForUser(userID)
		if err != nil {
			return fmt.Errorf("password: upsert password: %w", err)
		}
		db = orgDB
	}
	table := "user_auth_password"
	if s.isAdminUser(userID) {
		table = "admin_auth_password"
	}
	_, err := db.Exec(
		`INSERT INTO `+table+` (user_id, password, created_at, is_active, changed_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, TRUE, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password, is_active = TRUE, changed_at = CURRENT_TIMESTAMP`,
		userID, hash,
	)
	if err != nil {
		return fmt.Errorf("password: upsert password: %w", err)
	}
	return nil
}

// clearMustChangeFlag updates the must_change_password column on the
// appropriate table. Admins are in admin_users (main DB); regular users
// are in users (per-org DB).
func (s *Service) clearMustChangeFlag(userID string) error {
	if s.isAdminUser(userID) {
		if _, err := s.db.Exec(
			`UPDATE admin_users SET must_change_password = FALSE WHERE id = ?`, userID,
		); err != nil {
			return fmt.Errorf("password: clear must_change_password (admin): %w", err)
		}
		return nil
	}
	orgDB, err := s.orgDBForUser(userID)
	if err != nil {
		return fmt.Errorf("password: clear must_change_password: %w", err)
	}
	if _, err := orgDB.Exec(
		`UPDATE users SET must_change_password = FALSE WHERE id = ?`, userID,
	); err != nil {
		return fmt.Errorf("password: clear must_change_password (user): %w", err)
	}
	return nil
}
