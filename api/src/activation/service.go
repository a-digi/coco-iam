package activation

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-iam/src/general"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/scopedsettings"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-logger/logger"
)

// Service orchestrates the agnostic user activation flow. It is safe for
// concurrent use — every method opens its own short-lived transaction
// against the shared DB handle.
type Service struct {
	db          *sql.DB // main DB — admin_users, admin_activations
	orgRegistry *dbregistry.OrgUserDBRegistry
	adminStore  *AdminStore
	orgStore    *OrgStore
	mail        iam_mail.MailService
	mailConfig  *scopedsettings.ScopedResolver
	settings    *SettingsReader
	rules       *userrules.Store
	log         logger.Logger
}

// NewService constructs a Service.
// `orgRegistry` may be nil in admin-only paths; user activation will fail
// gracefully when it is absent.
// `rules` may be nil in tests; when nil, `userrules.Defaults()` apply.
func NewService(
	db *sql.DB,
	orgRegistry *dbregistry.OrgUserDBRegistry,
	adminStore *AdminStore,
	orgStore *OrgStore,
	mail iam_mail.MailService,
	mailConfig *scopedsettings.ScopedResolver,
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

// Start generates a fresh activation token + temp password, upserts the
// password row so the user can log in with that temp immediately, flips
// must_change_password on, and enqueues the invite email.
//
// Safe to call from user-creation paths: existing pending activations
// for the same user are invalidated first.
func (s *Service) Start(ctx context.Context, args StartArgs) (StartResult, error) {
	if !args.UserType.IsValid() {
		return StartResult{}, fmt.Errorf("activation: invalid user_type %q", args.UserType)
	}
	if args.UserID == "" {
		return StartResult{}, errors.New("activation: user_id is required")
	}

	baseURL := s.generalStoreFor(args.UserID, args.UserType).BaseURL()
	if baseURL == "" && args.UserType == UserTypeUser {
		// Org has no base_url in its app_settings — fall back to the global store.
		baseURL = general.NewStoreFromDB(s.db).BaseURL()
	}
	if baseURL == "" {
		return StartResult{}, errors.New("activation: base URL not configured — set it under Admin Settings → General")
	}

	// For user activations, resolve the org DB once and reuse it.
	var orgDB *sql.DB
	if args.UserType == UserTypeUser {
		var err error
		orgDB, err = s.openOrgDBForUserWithHint(args.UserID, args.OrgID)
		if err != nil {
			return StartResult{}, err
		}
	}

	if args.UserType == UserTypeAdmin {
		if err := s.adminStore.DeletePendingForUser(args.UserID); err != nil {
			return StartResult{}, err
		}
	} else {
		if err := s.orgStore.DeletePendingForUser(args.UserID, orgDB); err != nil {
			return StartResult{}, err
		}
	}

	token, err := GenerateToken()
	if err != nil {
		return StartResult{}, err
	}
	tempPassword, err := GenerateTempPassword()
	if err != nil {
		return StartResult{}, err
	}
	tempHash, err := bcrypt.HashPassword(tempPassword, bcrypt.DefaultCost)
	if err != nil {
		return StartResult{}, fmt.Errorf("activation: hash temp password: %w", err)
	}

	expiresAt := time.Now().UTC().Add(s.settings.TTL())

	row := Row{
		UserID:           args.UserID,
		UserType:         args.UserType,
		TokenHash:        HashToken(token),
		TempPasswordHash: tempHash,
		ExpiresAt:        expiresAt,
	}
	if args.Redirect != nil {
		row.RedirectOrgSlug = args.Redirect.OrgSlug
		row.RedirectWorkspaceSlug = args.Redirect.WorkspaceSlug
		row.RedirectClientID = args.Redirect.ClientID
	}

	if args.UserType == UserTypeAdmin {
		if err := s.adminStore.Insert(row); err != nil {
			return StartResult{}, err
		}
	} else {
		if err := s.orgStore.Insert(row, orgDB); err != nil {
			return StartResult{}, err
		}
	}

	if err := s.upsertPassword(args.UserType, args.UserID, args.OrgID, tempHash); err != nil {
		return StartResult{}, err
	}
	if err := s.setMustChangePassword(args.UserType, args.UserID, args.OrgID, true); err != nil {
		return StartResult{}, err
	}

	link := baseURL + "/activate?token=" + token
	if args.UserType == UserTypeAdmin {
		link = baseURL + "/activation/a?token=" + token
	}
	if err := s.sendInvite(ctx, args, link, tempPassword); err != nil {
		// Don't fail the whole flow — the admin can resend. Log prominently.
		s.log.Error("activation: failed to enqueue invite email for %s (%s): %v",
			args.UserID, args.UserType, err)
	}

	return StartResult{
		Token:        token,
		TempPassword: tempPassword,
		ExpiresAt:    expiresAt,
	}, nil
}

// Verify checks that the supplied token + email pair points at a
// pending (non-expired, non-consumed) activation row. Returns
// ErrActivationFailed for any failure.
func (s *Service) Verify(token, email string) error {
	row, _, err := s.loadUsableRow(token)
	if err != nil {
		return ErrActivationFailed
	}
	user, err := s.lookupUser(row.UserType, row.UserID)
	if err != nil {
		return ErrActivationFailed
	}
	if !emailsMatch(user.Email, email) {
		return ErrActivationFailed
	}
	return nil
}

// ErrActivationFailed is the single error returned for any
// authentication-related failure. The HTTP layer translates this to a
// generic 400 so attackers can't distinguish which check failed.
var ErrActivationFailed = errors.New("activation: something went wrong")

// VerifyAdmin checks that the supplied token + email pair points at a
// pending admin activation row. Unlike Verify it never scans per-org
// databases. Returns ErrActivationFailed for any failure.
func (s *Service) VerifyAdmin(token, email string) error {
	hash := HashToken(token)
	row, err := s.adminStore.FindByTokenHash(hash)
	if err != nil {
		return ErrActivationFailed
	}
	if row.ConsumedAt != nil {
		return ErrActivationFailed
	}
	if time.Now().UTC().After(row.ExpiresAt) {
		return ErrActivationFailed
	}
	user, err := s.lookupUser(UserTypeAdmin, row.UserID)
	if err != nil {
		return ErrActivationFailed
	}
	if !emailsMatch(user.Email, email) {
		return ErrActivationFailed
	}
	return nil
}

// ActivateAdmin consumes an admin activation token and sets the user's
// permanent password. Unlike Activate it never scans per-org databases
// and always resolves to an admin user. Returns ErrActivationFailed for
// any authentication-related failure.
func (s *Service) ActivateAdmin(token, email, newPassword string) (*ActivateResult, error) {
	if strings.TrimSpace(newPassword) == "" {
		return nil, errors.New("activation: new password cannot be empty")
	}

	hash := HashToken(token)
	row, err := s.adminStore.FindByTokenHash(hash)
	if err != nil {
		return nil, ErrActivationFailed
	}
	if row.ConsumedAt != nil {
		return nil, ErrActivationFailed
	}
	if time.Now().UTC().After(row.ExpiresAt) {
		return nil, ErrActivationFailed
	}

	user, err := s.lookupUser(UserTypeAdmin, row.UserID)
	if err != nil {
		return nil, ErrActivationFailed
	}
	if !emailsMatch(user.Email, email) {
		return nil, ErrActivationFailed
	}

	rs := s.RulesFor(UserTypeAdmin, row.UserID)
	if violations := userrules.ValidatePassword(rs.Password, userrules.Input{
		Username: user.Username,
		Email:    user.Email,
		Password: newPassword,
	}); len(violations) > 0 {
		return nil, errors.New(strings.Join(violations, " "))
	}

	newHash, err := bcrypt.HashPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("activation: hash new password: %w", err)
	}
	if err := s.upsertPassword(UserTypeAdmin, row.UserID, "", newHash); err != nil {
		return nil, err
	}
	if err := s.setMustChangePassword(UserTypeAdmin, row.UserID, "", false); err != nil {
		return nil, err
	}
	if err := s.adminStore.ConsumeByID(row.ID); err != nil {
		return nil, err
	}

	return &ActivateResult{
		UserType:    UserTypeAdmin,
		UserID:      row.UserID,
		Username:    user.Username,
		Email:       user.Email,
		RedirectURL: composeRedirectURL(row),
	}, nil
}

// Activate consumes the token if and only if (a) it exists, (b) it
// hasn't expired, (c) it hasn't been consumed, AND (d) the supplied
// email matches the user the token was minted for.
func (s *Service) Activate(token, email, newPassword string) (*ActivateResult, error) {
	if strings.TrimSpace(newPassword) == "" {
		return nil, errors.New("activation: new password cannot be empty")
	}

	row, orgDB, err := s.loadUsableRow(token)
	if err != nil {
		return nil, ErrActivationFailed
	}
	user, err := s.lookupUser(row.UserType, row.UserID)
	if err != nil {
		return nil, ErrActivationFailed
	}

	if !emailsMatch(user.Email, email) {
		return nil, ErrActivationFailed
	}

	rs := s.RulesFor(row.UserType, row.UserID)
	if violations := userrules.ValidatePassword(rs.Password, userrules.Input{
		Username: user.Username,
		Email:    user.Email,
		Password: newPassword,
	}); len(violations) > 0 {
		return nil, errors.New(strings.Join(violations, " "))
	}

	newHash, err := bcrypt.HashPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("activation: hash new password: %w", err)
	}

	if err := s.upsertPassword(row.UserType, row.UserID, "", newHash); err != nil {
		return nil, err
	}
	if err := s.setMustChangePassword(row.UserType, row.UserID, "", false); err != nil {
		return nil, err
	}
	if row.UserType == UserTypeUser {
		if err := s.setIsActive(row.UserID, ""); err != nil {
			return nil, err
		}
	}

	if row.UserType == UserTypeAdmin {
		if err := s.adminStore.ConsumeByID(row.ID); err != nil {
			return nil, err
		}
	} else {
		if err := s.orgStore.ConsumeByID(row.ID, orgDB); err != nil {
			return nil, err
		}
	}

	return &ActivateResult{
		UserType:    row.UserType,
		UserID:      row.UserID,
		Username:    user.Username,
		Email:       user.Email,
		RedirectURL: composeRedirectURL(row),
	}, nil
}

// composeRedirectURL turns the three slug fields on an activation row
// into the per-app login path. All three must be set.
func composeRedirectURL(row *Row) string {
	if row.RedirectOrgSlug == "" || row.RedirectWorkspaceSlug == "" || row.RedirectClientID == "" {
		return ""
	}
	return "/login/a/" +
		url.PathEscape(row.RedirectOrgSlug) + "/" +
		url.PathEscape(row.RedirectWorkspaceSlug) + "/" +
		url.PathEscape(row.RedirectClientID)
}

// Resend regenerates a token + temp password for a user who already has
// one (admin-triggered). Enforces the configured cooldown.
func (s *Service) Resend(ctx context.Context, userType UserType, userID string) (StartResult, error) {
	var carriedRedirect *RedirectTarget

	if userType == UserTypeAdmin {
		latest, err := s.adminStore.LatestPendingForUser(userID)
		if err != nil {
			return StartResult{}, err
		}
		if latest != nil {
			elapsed := time.Since(latest.CreatedAt)
			if cooldown := s.settings.ResendCooldown(); elapsed < cooldown {
				return StartResult{}, fmt.Errorf(
					"%w (wait %s)", ErrCooldown, (cooldown - elapsed).Round(time.Second),
				)
			}
			if latest.RedirectOrgSlug != "" && latest.RedirectWorkspaceSlug != "" && latest.RedirectClientID != "" {
				carriedRedirect = &RedirectTarget{
					OrgSlug:       latest.RedirectOrgSlug,
					WorkspaceSlug: latest.RedirectWorkspaceSlug,
					ClientID:      latest.RedirectClientID,
				}
			}
		}
	} else {
		orgDB, err := s.openOrgDBForUser(userID)
		if err != nil {
			return StartResult{}, err
		}
		latest, err := s.orgStore.LatestPendingForUser(userID, orgDB)
		if err != nil {
			return StartResult{}, err
		}
		if latest != nil {
			elapsed := time.Since(latest.CreatedAt)
			if cooldown := s.settings.ResendCooldown(); elapsed < cooldown {
				return StartResult{}, fmt.Errorf(
					"%w (wait %s)", ErrCooldown, (cooldown - elapsed).Round(time.Second),
				)
			}
			if latest.RedirectOrgSlug != "" && latest.RedirectWorkspaceSlug != "" && latest.RedirectClientID != "" {
				carriedRedirect = &RedirectTarget{
					OrgSlug:       latest.RedirectOrgSlug,
					WorkspaceSlug: latest.RedirectWorkspaceSlug,
					ClientID:      latest.RedirectClientID,
				}
			}
		}
	}

	user, err := s.lookupUser(userType, userID)
	if err != nil {
		return StartResult{}, err
	}
	return s.Start(ctx, StartArgs{
		UserType: userType,
		UserID:   userID,
		Username: user.Username,
		Email:    user.Email,
		Redirect: carriedRedirect,
	})
}

// ChangePasswordForUser clears must_change_password for an already-
// authenticated user, replacing their password.
func (s *Service) ChangePasswordForUser(userType UserType, userID, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("activation: new password cannot be empty")
	}
	if user, err := s.lookupUser(userType, userID); err == nil {
		rs := s.RulesFor(userType, userID)
		if violations := userrules.ValidatePassword(rs.Password, userrules.Input{
			Username: user.Username,
			Email:    user.Email,
			Password: newPassword,
		}); len(violations) > 0 {
			return errors.New(strings.Join(violations, " "))
		}
	}
	hash, err := bcrypt.HashPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("activation: hash new password: %w", err)
	}
	if err := s.upsertPassword(userType, userID, "", hash); err != nil {
		return err
	}
	if err := s.setMustChangePassword(userType, userID, "", false); err != nil {
		return err
	}

	if userType == UserTypeAdmin {
		if err := s.adminStore.DeletePendingForUser(userID); err != nil {
			s.log.Warning("activation: purge pending after change-password: %v", err)
		}
	} else {
		if orgDB, err := s.openOrgDBForUser(userID); err == nil {
			if err := s.orgStore.DeletePendingForUser(userID, orgDB); err != nil {
				s.log.Warning("activation: purge pending after change-password: %v", err)
			}
		}
	}
	return nil
}

// RulesFor returns the rule set that applies to the given activation row.
func (s *Service) RulesFor(userType UserType, userID string) userrules.RuleSet {
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

// RulesForToken resolves the rule set for a pending activation by token.
func (s *Service) RulesForToken(token string) userrules.RuleSet {
	row, _, err := s.loadUsableRow(token)
	if err != nil {
		return userrules.Defaults()
	}
	return s.RulesFor(row.UserType, row.UserID)
}

// --- helpers ---

// loadUsableRow tries the admin store first, then scans org stores.
// Returns (row, orgDB, nil) where orgDB is nil for admin rows.
func (s *Service) loadUsableRow(token string) (*Row, *sql.DB, error) {
	hash := HashToken(token)

	// Admin store is O(1) — try it first.
	if row, err := s.adminStore.FindByTokenHash(hash); err == nil {
		if row.ConsumedAt != nil {
			return nil, nil, ErrAlreadyUsed
		}
		if time.Now().UTC().After(row.ExpiresAt) {
			return nil, nil, ErrExpired
		}
		return row, nil, nil
	}

	// Org store is O(N) scan across per-org DBs.
	if s.orgStore != nil {
		row, orgDB, err := s.orgStore.FindByTokenHash(hash)
		if err == nil {
			if row.ConsumedAt != nil {
				return nil, nil, ErrAlreadyUsed
			}
			if time.Now().UTC().After(row.ExpiresAt) {
				return nil, nil, ErrExpired
			}
			return row, orgDB, nil
		}
	}
	return nil, nil, ErrNotFound
}

type userRef struct {
	Username string
	Email    string
}

func (s *Service) lookupUser(t UserType, id string) (*userRef, error) {
	if t == UserTypeAdmin {
		var u userRef
		err := s.db.QueryRow(
			`SELECT username, email FROM admin_users WHERE id = ?`, id,
		).Scan(&u.Username, &u.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("activation: admin user %s not found", id)
			}
			return nil, fmt.Errorf("activation: lookup admin user: %w", err)
		}
		return &u, nil
	}
	orgDB, err := s.openOrgDBForUser(id)
	if err != nil {
		return nil, err
	}
	var u userRef
	if err := orgDB.QueryRow(
		`SELECT username, email FROM users WHERE id = ?`, id,
	).Scan(&u.Username, &u.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("activation: user %s not found", id)
		}
		return nil, fmt.Errorf("activation: lookup user: %w", err)
	}
	return &u, nil
}

func (s *Service) upsertPassword(t UserType, userID, orgID, hash string) error {
	db := s.db
	if t == UserTypeUser {
		odb, err := s.openOrgDBForUserWithHint(userID, orgID)
		if err != nil {
			return fmt.Errorf("activation: upsert password: %w", err)
		}
		db = odb
	}
	table := "user_auth_password"
	if t != UserTypeUser {
		table = "admin_auth_password"
	}
	_, err := db.Exec(
		`INSERT INTO `+table+` (user_id, password, created_at, is_active, changed_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP, TRUE, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password, is_active = TRUE, changed_at = CURRENT_TIMESTAMP`,
		userID, hash,
	)
	if err != nil {
		return fmt.Errorf("activation: upsert password: %w", err)
	}
	return nil
}

func (s *Service) setMustChangePassword(t UserType, id, orgID string, value bool) error {
	if t == UserTypeAdmin {
		_, err := s.db.Exec(
			`UPDATE admin_users SET must_change_password = ? WHERE id = ?`, value, id,
		)
		if err != nil {
			return fmt.Errorf("activation: set must_change_password (admin): %w", err)
		}
		return nil
	}
	orgDB, err := s.openOrgDBForUserWithHint(id, orgID)
	if err != nil {
		return fmt.Errorf("activation: set must_change_password: %w", err)
	}
	_, err = orgDB.Exec(
		`UPDATE users SET must_change_password = ? WHERE id = ?`, value, id,
	)
	if err != nil {
		return fmt.Errorf("activation: set must_change_password (user): %w", err)
	}
	return nil
}

func (s *Service) setIsActive(id, orgID string) error {
	orgDB, err := s.openOrgDBForUserWithHint(id, orgID)
	if err != nil {
		return fmt.Errorf("activation: set is_active (user): %w", err)
	}
	_, err = orgDB.Exec(`UPDATE users SET is_active = true WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("activation: set is_active (user): %w", err)
	}
	return nil
}

// IsActivated reports whether the org user has ever consumed an activation
// token. Only UserTypeUser is supported — any other type returns an error so
// accidental admin-path calls fail loudly.
func (s *Service) IsActivated(userType UserType, userID string) (bool, error) {
	if userType != UserTypeUser {
		return false, fmt.Errorf("activation: IsActivated only supports UserTypeUser")
	}
	orgDB, err := s.openOrgDBForUser(userID)
	if err != nil {
		return false, err
	}
	return s.orgStore.HasConsumedActivation(userID, orgDB)
}

func (s *Service) generalStoreFor(userID string, userType UserType) *general.Store {
	if userType == UserTypeUser && s.orgRegistry != nil {
		if orgDB, _, err := orgrouter.OrgDBFor(s.orgRegistry, userID); err == nil {
			return general.NewStoreFromDB(orgDB)
		}
	}
	return general.NewStoreFromDB(s.db)
}

// resolvedOrgIDFor returns the org id for a UserTypeUser send — args.OrgID
// when the caller already supplied it, otherwise resolved by scanning
// for userID (same lookup generalStoreFor already does). Returns "" for
// admin sends (no org concept) or when resolution fails, which the mail
// resolver treats identically to "no org override — use global".
func (s *Service) resolvedOrgIDFor(args StartArgs) string {
	if args.UserType != UserTypeUser {
		return ""
	}
	if args.OrgID != "" {
		return args.OrgID
	}
	if s.orgRegistry == nil {
		return ""
	}
	_, orgID, err := orgrouter.OrgDBFor(s.orgRegistry, args.UserID)
	if err != nil {
		return ""
	}
	return orgID
}

func (s *Service) openOrgDBForUser(userID string) (*sql.DB, error) {
	return s.openOrgDBForUserWithHint(userID, "")
}

func (s *Service) openOrgDBForUserWithHint(userID, orgID string) (*sql.DB, error) {
	if s.orgRegistry == nil {
		return nil, fmt.Errorf("activation: org db registry not available")
	}
	if orgID != "" {
		return orgrouter.ForOrg(s.orgRegistry, orgID)
	}
	odb, _, err := orgrouter.OrgDBFor(s.orgRegistry, userID)
	if err != nil {
		return nil, fmt.Errorf("activation: org lookup for user %s: %w", userID, err)
	}
	return odb, nil
}

func (s *Service) sendInvite(ctx context.Context, args StartArgs, link, tempPassword string) error {
	event := EventUserInvite
	if args.UserType == UserTypeAdmin {
		event = EventAdminInvite
	}
	orgID := s.resolvedOrgIDFor(args)

	template := s.mailConfig.TemplateForEvent(orgID, "", event)
	if template == "" {
		return fmt.Errorf("no template bound to event %q — configure it in Admin Settings → Email", event)
	}
	account, resolvedOrgID := s.mailConfig.AccountForEvent(orgID, "", event)
	if account == "" {
		return fmt.Errorf("no account bound to event %q — configure it in Admin Settings → Email", event)
	}

	gs := s.generalStoreFor(args.UserID, args.UserType)
	websiteTitle := gs.PageTitle()
	if websiteTitle == "" {
		websiteTitle = extractHost(gs.BaseURL())
	}
	if websiteTitle == "" && args.UserType == UserTypeUser {
		globalGs := general.NewStoreFromDB(s.db)
		websiteTitle = globalGs.PageTitle()
		if websiteTitle == "" {
			websiteTitle = extractHost(globalGs.BaseURL())
		}
	}

	data := map[string]interface{}{
		"WebsiteTitle":   websiteTitle,
		"PageTitle":      websiteTitle,
		"Username":       args.Username,
		"ActivationLink": link,
		"TempPassword":   tempPassword,
		"ExpiresIn":      s.settings.TTLHumanReadable(),
		"ResetLink":      link,
	}

	task := iam_mail.MailTask{
		Account: account,
		OrgID:   resolvedOrgID,
		To:      []iam_mail.Address{{Email: args.Email, Name: args.Username}},
	}
	// Prefer this org's own active template of the same name over the
	// global renderer — falls through untouched (task.Template +
	// task.Data set, exactly as before) when the org has none of its own.
	if subject, text, html, ok, rerr := s.mailConfig.RenderTemplate(orgID, "", template, data); rerr == nil && ok {
		task.Subject, task.TextBody, task.HTMLBody = subject, text, html
	} else {
		if rerr != nil {
			s.log.Warning("activation: org template render for %q failed, falling back to global: %v", template, rerr)
		}
		task.Template = template
		task.Data = data
	}

	_, err := s.mail.Enqueue(task)
	_ = ctx
	return err
}

func emailsMatch(storedEmail, provided string) bool {
	a := []byte(strings.ToLower(strings.TrimSpace(storedEmail)))
	b := []byte(strings.ToLower(strings.TrimSpace(provided)))
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}

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
