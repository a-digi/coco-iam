package recovery

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	bcryptx "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-iam/src/general"
	iam_notification "github.com/a-digi/coco-iam/src/notification"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-logger/logger"
	coconotification "github.com/a-digi/coco-notification"
)

// ErrRecoveryFailed collapses every authentication-related failure
// (bad token, expired, consumed, email mismatch, unknown user). The
// HTTP layer translates this to a single generic 400 so attackers
// can't probe which check tripped.
var ErrRecoveryFailed = errors.New("recovery: something went wrong")

// Service orchestrates the email-based password-recovery flow.
// Safe for concurrent use.
type Service struct {
	db          *sql.DB // main DB — admin_users, user_org_index, user_email_org_index
	orgRegistry *dbregistry.OrgUserDBRegistry
	adminStore  *AdminStore
	orgStore    *OrgStore
	mail        coconotification.Service
	mailConfig  *iam_notification.ScopedResolver
	settings    *SettingsReader
	rules       *userrules.Store
	log         logger.Logger
}

// NewService wires every dependency needed by the recovery flow.
// orgRegistry may be nil; user-facing recovery paths will fail
// gracefully when it is absent.
func NewService(
	db *sql.DB,
	orgRegistry *dbregistry.OrgUserDBRegistry,
	adminStore *AdminStore,
	orgStore *OrgStore,
	mail coconotification.Service,
	mailConfig *iam_notification.ScopedResolver,
	settings *SettingsReader,
	rules *userrules.Store,
	log logger.Logger,
) *Service {
	return &Service{
		db:          db,
		orgRegistry: orgRegistry,
		adminStore:  adminStore,
		orgStore:    orgStore,
		mail:        mail,
		mailConfig:  mailConfig,
		settings:    settings,
		rules:       rules,
		log:         log,
	}
}

// Request mints a recovery token for the account owning `email` and
// enqueues the reset email. Unknown emails are silently accepted — the
// HTTP layer always returns 200 so the endpoint can't be used to
// enumerate accounts.
//
// Never returns an error for "user not found" — only for infrastructure
// failures, and even those are swallowed by the handler.
func (s *Service) Request(ctx context.Context, email string) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}

	ref, found := s.lookupUserByEmail(email)
	if !found {
		return
	}

	baseURL := s.generalStoreFor(ref).BaseURL()
	if baseURL == "" {
		s.log.Warning("recovery: base URL not configured — cannot build reset link")
		return
	}

	if ref.UserType == UserTypeAdmin {
		if latest, err := s.adminStore.LatestPendingForUser(ref.UserID); err == nil && latest != nil {
			if time.Since(latest.CreatedAt) < s.settings.ResendCooldown() {
				return
			}
		}
		if err := s.adminStore.DeletePendingForUser(ref.UserID); err != nil {
			s.log.Warning("recovery: purge pending (admin): %v", err)
		}
		token, err := GenerateToken()
		if err != nil {
			s.log.Error("recovery: generate token: %v", err)
			return
		}
		if err := s.adminStore.Insert(Row{
			UserID:    ref.UserID,
			UserType:  ref.UserType,
			TokenHash: HashToken(token),
			ExpiresAt: time.Now().UTC().Add(s.settings.TTL()),
		}); err != nil {
			s.log.Error("recovery: insert row (admin): %v", err)
			return
		}
		link := baseURL + "/reset-password?token=" + token
		if err := s.sendRecoveryEmail(ctx, ref, link); err != nil {
			s.log.Error("recovery: failed to enqueue email for %s: %v", ref.UserID, err)
		}
		return
	}

	// Regular user — route through per-org DB.
	if s.orgRegistry == nil {
		return
	}
	orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, ref.UserID)
	if err != nil {
		s.log.Warning("recovery: resolve org db for user %s: %v", ref.UserID, err)
		return
	}
	if latest, err := s.orgStore.LatestPendingForUser(ref.UserID, orgDB); err == nil && latest != nil {
		if time.Since(latest.CreatedAt) < s.settings.ResendCooldown() {
			return
		}
	}
	if err := s.orgStore.DeletePendingForUser(ref.UserID, orgDB); err != nil {
		s.log.Warning("recovery: purge pending (user): %v", err)
	}
	token, err := GenerateToken()
	if err != nil {
		s.log.Error("recovery: generate token: %v", err)
		return
	}
	if err := s.orgStore.Insert(Row{
		UserID:    ref.UserID,
		TokenHash: HashToken(token),
		ExpiresAt: time.Now().UTC().Add(s.settings.TTL()),
	}, orgDB); err != nil {
		s.log.Error("recovery: insert row (user): %v", err)
		return
	}
	link := baseURL + "/reset-password?token=" + token
	if err := s.sendRecoveryEmail(ctx, ref, link); err != nil {
		s.log.Error("recovery: failed to enqueue email for %s: %v", ref.UserID, err)
	}
}

// Verify returns nil + the applicable rule set if (token, email)
// matches a pending row. Any failure → ErrRecoveryFailed.
func (s *Service) Verify(token, email string) (userrules.RuleSet, error) {
	row, _, err := s.loadUsableRow(token)
	if err != nil {
		return userrules.Defaults(), ErrRecoveryFailed
	}
	user, err := s.lookupUser(row.UserType, row.UserID)
	if err != nil {
		return userrules.Defaults(), ErrRecoveryFailed
	}
	if !emailsMatch(user.Email, email) {
		return userrules.Defaults(), ErrRecoveryFailed
	}
	return s.rulesFor(row.UserType, row.UserID), nil
}

// Reset consumes the token if every check passes, validates the new
// password against user-rules, and writes the hash. Rule violations
// surface verbatim; auth failures collapse to ErrRecoveryFailed.
func (s *Service) Reset(token, email, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("recovery: new password cannot be empty")
	}

	row, orgDB, err := s.loadUsableRow(token)
	if err != nil {
		return ErrRecoveryFailed
	}
	user, err := s.lookupUser(row.UserType, row.UserID)
	if err != nil {
		return ErrRecoveryFailed
	}
	if !emailsMatch(user.Email, email) {
		return ErrRecoveryFailed
	}

	rs := s.rulesFor(row.UserType, row.UserID)
	if violations := userrules.ValidatePassword(rs.Password, userrules.Input{
		Username: user.Username,
		Email:    user.Email,
		Password: newPassword,
	}); len(violations) > 0 {
		return errors.New(strings.Join(violations, " "))
	}

	newHash, err := bcryptx.HashPassword(newPassword, bcryptx.DefaultCost)
	if err != nil {
		return fmt.Errorf("recovery: hash new password: %w", err)
	}
	if err := s.upsertPassword(row.UserType, row.UserID, newHash); err != nil {
		return err
	}
	if err := s.setMustChangePassword(row.UserType, row.UserID, false); err != nil {
		return err
	}
	if row.UserType == UserTypeAdmin {
		return s.adminStore.ConsumeByID(row.ID)
	}
	return s.orgStore.ConsumeByID(row.ID, orgDB)
}

// RulesForToken returns the rule set applicable to a pending row,
// without requiring email. Used by the public Verify endpoint so the
// UI can pre-validate. Falls back to defaults when anything's off —
// the server still enforces on Reset.
func (s *Service) RulesForToken(token string) userrules.RuleSet {
	row, _, err := s.loadUsableRow(token)
	if err != nil {
		return userrules.Defaults()
	}
	return s.rulesFor(row.UserType, row.UserID)
}

// --- helpers ---

// generalStoreFor returns a general.Store backed by the per-org DB for
// regular users, or the main DB for admin users. Callers use it to read
// BaseURL and PageTitle at request time rather than at boot time.
func (s *Service) generalStoreFor(ref userRef) *general.Store {
	if ref.UserType == UserTypeUser && s.orgRegistry != nil {
		if orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, ref.UserID); err == nil {
			return general.NewStoreFromDB(orgDB)
		}
	}
	return general.NewStoreFromDB(s.db)
}

// resolvedOrgIDFor returns the org id for a UserTypeUser recovery send
// (resolved by scanning for ref.UserID, same lookup generalStoreFor
// already does), or "" for admin sends or a failed lookup — which the
// mail resolver treats identically to "no org override — use global".
func (s *Service) resolvedOrgIDFor(ref userRef) string {
	if ref.UserType != UserTypeUser || s.orgRegistry == nil {
		return ""
	}
	_, orgID, err := orgrouter.OrgDBFor(s.orgRegistry, ref.UserID)
	if err != nil {
		return ""
	}
	return orgID
}

// loadUsableRow looks up a token hash across both stores.
// Returns the row and the per-org DB (nil for admin rows).
func (s *Service) loadUsableRow(token string) (*Row, *sql.DB, error) {
	hash := HashToken(token)

	// Try admin store first.
	if row, err := s.adminStore.FindByTokenHash(hash); err == nil {
		if row.ConsumedAt != nil {
			return nil, nil, ErrAlreadyUsed
		}
		if time.Now().UTC().After(row.ExpiresAt) {
			return nil, nil, ErrExpired
		}
		return row, nil, nil
	}

	// Try org store.
	row, orgDB, err := s.orgStore.FindByTokenHash(hash)
	if err != nil {
		return nil, nil, err
	}
	if row.ConsumedAt != nil {
		return nil, nil, ErrAlreadyUsed
	}
	if time.Now().UTC().After(row.ExpiresAt) {
		return nil, nil, ErrExpired
	}
	return row, orgDB, nil
}

type userRef struct {
	UserID   string
	UserType UserType
	Username string
	Email    string
}

// lookupUserByEmail searches admin_users first (main DB), then uses
// user_email_org_index to locate a regular user in their per-org DB.
func (s *Service) lookupUserByEmail(email string) (userRef, bool) {
	// Admin users stay in the main DB.
	var adminRef userRef
	adminRef.UserType = UserTypeAdmin
	if err := s.db.QueryRow(
		`SELECT id, username, email FROM admin_users WHERE LOWER(email) = LOWER(?) LIMIT 1`, email,
	).Scan(&adminRef.UserID, &adminRef.Username, &adminRef.Email); err == nil {
		return adminRef, true
	}

	// Regular users — scan per-org DBs for the email.
	if s.orgRegistry == nil {
		return userRef{}, false
	}
	orgDB, _, userID, err := orgrouter.OrgDBForEmail(s.orgRegistry, email)
	if err != nil {
		return userRef{}, false
	}
	var u userRef
	u.UserType = UserTypeUser
	if err := orgDB.QueryRow(
		`SELECT id, username, email FROM users WHERE id = ? LIMIT 1`, userID,
	).Scan(&u.UserID, &u.Username, &u.Email); err != nil {
		return userRef{}, false
	}
	return u, true
}

func (s *Service) lookupUser(t UserType, id string) (*userRef, error) {
	u := &userRef{UserType: t, UserID: id}
	if t == UserTypeAdmin {
		if err := s.db.QueryRow(
			`SELECT username, email FROM admin_users WHERE id = ?`, id,
		).Scan(&u.Username, &u.Email); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("recovery: admin user %s not found", id)
			}
			return nil, fmt.Errorf("recovery: lookup admin user: %w", err)
		}
		return u, nil
	}
	// Regular user — open the per-org DB.
	if s.orgRegistry == nil {
		return nil, fmt.Errorf("recovery: org db registry not available")
	}
	orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, id)
	if err != nil {
		return nil, fmt.Errorf("recovery: resolve org db for user %s: %w", id, err)
	}
	if err := orgDB.QueryRow(
		`SELECT username, email FROM users WHERE id = ?`, id,
	).Scan(&u.Username, &u.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("recovery: user %s not found", id)
		}
		return nil, fmt.Errorf("recovery: lookup user: %w", err)
	}
	return u, nil
}

func (s *Service) upsertPassword(t UserType, userID, hash string) error {
	db := s.db
	if t == UserTypeUser {
		if s.orgRegistry == nil {
			return fmt.Errorf("recovery: org db registry not available")
		}
		orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, userID)
		if err != nil {
			return fmt.Errorf("recovery: upsert password: %w", err)
		}
		db = orgDB
	}
	table := "user_auth_password"
	if t == UserTypeAdmin {
		table = "admin_auth_password"
	}
	_, err := db.Exec(
		`INSERT INTO `+table+` (user_id, password, created_at, is_active, changed_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, TRUE, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password, is_active = TRUE, changed_at = CURRENT_TIMESTAMP`,
		userID, hash,
	)
	if err != nil {
		return fmt.Errorf("recovery: upsert password: %w", err)
	}
	return nil
}

func (s *Service) setMustChangePassword(t UserType, id string, value bool) error {
	if t == UserTypeAdmin {
		_, err := s.db.Exec(
			`UPDATE admin_users SET must_change_password = ? WHERE id = ?`, value, id,
		)
		if err != nil {
			return fmt.Errorf("recovery: set must_change_password (admin): %w", err)
		}
		return nil
	}
	if s.orgRegistry == nil {
		return fmt.Errorf("recovery: org db registry not available")
	}
	orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, id)
	if err != nil {
		return fmt.Errorf("recovery: set must_change_password: %w", err)
	}
	_, err = orgDB.Exec(
		`UPDATE users SET must_change_password = ? WHERE id = ?`, value, id,
	)
	if err != nil {
		return fmt.Errorf("recovery: set must_change_password (user): %w", err)
	}
	return nil
}

// rulesFor resolves the rule set for an identified user — admin rules
// for admins, the user's org rules otherwise.
func (s *Service) rulesFor(userType UserType, userID string) userrules.RuleSet {
	if s.rules == nil {
		return userrules.Defaults()
	}
	if userType == UserTypeAdmin {
		rs, err := s.rules.GetAdmin()
		if err != nil {
			return userrules.Defaults()
		}
		return rs
	}
	rs, err := s.rules.GetForUser(userID)
	if err != nil {
		return userrules.Defaults()
	}
	return rs
}

// sendRecoveryEmail resolves the template + account bound to
// `password_recovery` and enqueues the mail task.
func (s *Service) sendRecoveryEmail(ctx context.Context, ref userRef, link string) error {
	orgID := s.resolvedOrgIDFor(ref)

	template := s.mailConfig.TemplateForEvent(orgID, "", EventPasswordRecovery)
	if template == "" {
		return fmt.Errorf("no template bound to event %q — configure it in Admin Settings → Email", EventPasswordRecovery)
	}
	account, resolvedOrgID, _ := s.mailConfig.AccountForEvent(orgID, "", EventPasswordRecovery)
	if account == "" {
		return fmt.Errorf("no account bound to event %q — configure it in Admin Settings → Email", EventPasswordRecovery)
	}

	gs := s.generalStoreFor(ref)
	websiteTitle := gs.PageTitle()
	if websiteTitle == "" {
		websiteTitle = extractHost(gs.BaseURL())
	}

	data := map[string]interface{}{
		"WebsiteTitle": websiteTitle,
		"PageTitle":    websiteTitle,
		"Username":     ref.Username,
		"ResetLink":    link,
		"ExpiresIn":    s.settings.TTLHumanReadable(),
	}

	task := coconotification.Task{
		Ref: coconotification.SenderRef{Name: account, Scope: iam_notification.Scope(resolvedOrgID, "")},
		To:  []coconotification.Address{{Email: ref.Email, Name: ref.Username}},
	}
	// Prefer this org's own active template of the same name over the
	// global renderer — falls through untouched when the org has none.
	if subject, text, html, ok, rerr := s.mailConfig.RenderTemplate(orgID, "", template, data); rerr == nil && ok {
		task.Subject, task.TextBody, task.HTMLBody = subject, text, html
	} else {
		if rerr != nil {
			s.log.Warning("recovery: org template render for %q failed, falling back to global: %v", template, rerr)
		}
		task.Template = template
		task.Data = data
	}

	_, err := s.mail.Enqueue(task)
	_ = ctx
	return err
}

// emailsMatch is a case-insensitive, trim-tolerant constant-time
// compare. Same helper activation uses.
func emailsMatch(stored, provided string) bool {
	a := []byte(strings.ToLower(strings.TrimSpace(stored)))
	b := []byte(strings.ToLower(strings.TrimSpace(provided)))
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

// extractHost returns a user-friendly name derived from the base URL.
// Same fallback activation uses — scheme + path are stripped.
func extractHost(u string) string {
	if u == "" {
		return "coco-iam"
	}
	if idx := strings.Index(u, "://"); idx >= 0 {
		u = u[idx+3:]
	}
	if idx := strings.IndexAny(u, "/?"); idx >= 0 {
		u = u[:idx]
	}
	if u == "" {
		return "coco-iam"
	}
	return u
}
