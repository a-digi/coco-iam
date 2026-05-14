package recoverypage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	bcryptx "github.com/a-digi/coco-iam/src/auth/crypto/bcrypt"
	"github.com/a-digi/coco-iam/src/auth/recovery"
	"github.com/a-digi/coco-iam/src/general"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-logger/logger"
)

// Service coordinates the per-application password-recovery flow. It
// reuses recovery.OrgStore for token storage + consumption and the
// mail service for delivery, but adds the per-application ACL gate:
// only users with an active `application_user_acl` row for the calling
// app can trigger a recovery. Unknown or non-ACL emails still get a
// 200 OK to avoid enumeration.
type Service struct {
	db            *sql.DB // main DB — applications, workspaces, organizations
	orgRegistry   *dbregistry.OrgUserDBRegistry
	store         *Store              // our template store
	recoveryStore *recovery.OrgStore  // shared per-org token store
	mail          iam_mail.MailService
	mailConfig    *mailsettings.Resolver
	settings      *recovery.SettingsReader
	rules         *userrules.Store
	log           logger.Logger
}

func NewService(
	db *sql.DB,
	orgRegistry *dbregistry.OrgUserDBRegistry,
	store *Store,
	recoveryStore *recovery.OrgStore,
	mail iam_mail.MailService,
	mailConfig *mailsettings.Resolver,
	settings *recovery.SettingsReader,
	rules *userrules.Store,
	log logger.Logger,
) *Service {
	return &Service{
		db:            db,
		orgRegistry:   orgRegistry,
		store:         store,
		recoveryStore: recoveryStore,
		mail:          mail,
		mailConfig:    mailConfig,
		settings:      settings,
		rules:         rules,
		log:           log,
	}
}

// ErrRecoveryFailed collapses every auth-related failure into one
// opaque error so the HTTP layer can return a single generic 400.
// Same pattern the global recovery flow uses.
var ErrRecoveryFailed = errors.New("recovery: something went wrong")

// -- templates ----------------------------------------------------------

type appInfo struct {
	ID               string
	WorkspaceID      string
	WorkspaceName    string
	ApplicationName  string
	OrganizationID   string // UUID — used to open the per-org DB
	OrganizationSlug string
	WorkspaceSlug    string
	ClientID         string
}

// findAppBySlugs resolves the (organization_id, workspace_id,
// client_id) slug trio to the application row. Also returns the
// organization UUID so callers can open the per-org DB.
// Applications and workspaces now live in the per-org DB; the org
// UUID is resolved from the main DB organization table first.
func (s *Service) findAppBySlugs(orgSlug, wsSlug, clientID string) (appInfo, error) {
	// Resolve org UUID from main DB.
	var orgID string
	if err := s.db.QueryRow(
		`SELECT id FROM organization WHERE organization_id = ? LIMIT 1`,
		orgSlug,
	).Scan(&orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appInfo{}, ErrNotFound
		}
		return appInfo{}, fmt.Errorf("recoverypage: resolve org: %w", err)
	}

	// Open per-org DB.
	orgDB, err := orgrouter.ForOrg(s.orgRegistry, orgID)
	if err != nil {
		return appInfo{}, fmt.Errorf("recoverypage: open org db: %w", err)
	}

	// Application + workspace from per-org DB.
	var info appInfo
	if err := orgDB.QueryRow(
		`SELECT a.id, a.client_id, a.workspace_id, a.title,
		        w.workspace_id, w.title
		 FROM applications a
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE w.workspace_id = ? AND a.client_id = ?
		 LIMIT 1`,
		wsSlug, clientID,
	).Scan(
		&info.ID, &info.ClientID, &info.WorkspaceID, &info.ApplicationName,
		&info.WorkspaceSlug, &info.WorkspaceName,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appInfo{}, ErrNotFound
		}
		return appInfo{}, fmt.Errorf("recoverypage: find app: %w", err)
	}
	info.OrganizationID = orgID
	info.OrganizationSlug = orgSlug
	return info, nil
}

// GetPublicTemplate returns the sanitised template shape the public
// /recover/a/:ws/:app pages serve. Unknown kind → ErrNotFound.
// -- recovery flow ------------------------------------------------------

type appUserRef struct {
	UserID   string
	Username string
	Email    string
}

// findAppUserByEmail looks up a user on an application's ACL by email.
// Missing → false, nil. Returns only users with an active ACL row for
// the calling application; users in the org but not on the app are
// invisible (no email enumeration). orgID is the organization UUID.
func (s *Service) findAppUserByEmail(appID, orgID, email string) (appUserRef, bool) {
	if s.orgRegistry == nil {
		return appUserRef{}, false
	}
	orgDB, err := orgrouter.ForOrg(s.orgRegistry, orgID)
	if err != nil {
		return appUserRef{}, false
	}
	var ref appUserRef
	err = orgDB.QueryRow(
		`SELECT u.id, u.username, u.email
		 FROM users u
		 JOIN application_user_acl acl ON acl.user_id = u.id AND acl.is_active = TRUE
		 WHERE acl.application_id = ? AND u.is_active = TRUE
		   AND LOWER(u.email) = LOWER(?)
		 LIMIT 1`,
		appID, email,
	).Scan(&ref.UserID, &ref.Username, &ref.Email)
	if err != nil {
		return appUserRef{}, false
	}
	return ref, true
}

// StartRecovery handles the POST /recover/request flow. Silent on
// every branch — the HTTP layer should return 200 regardless so the
// endpoint can't be used to probe membership.
func (s *Service) StartRecovery(ctx context.Context, orgSlug, wsSlug, clientID, email string) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}
	info, err := s.findAppBySlugs(orgSlug, wsSlug, clientID)
	if err != nil {
		return
	}
	// Recovery gate (plan Q5). Silent return if the admin has
	// disabled recovery for this app — no error exposed to the
	// caller so the endpoint can't be used to probe the flag.
	if !s.isRecoveryAllowed(info.ID) {
		return
	}
	ref, found := s.findAppUserByEmail(info.ID, info.OrganizationID, email)
	if !found {
		return
	}

	// Open the per-org DB for token operations.
	if s.orgRegistry == nil {
		return
	}
	orgDB, err := orgrouter.ForOrg(s.orgRegistry, info.OrganizationID)
	if err != nil {
		s.log.Warning("app-recovery: open org db: %v", err)
		return
	}

	// Cooldown: silently skip if a pending token already exists within
	// the configured resend window. Same policy the admin flow uses.
	if latest, err := s.recoveryStore.LatestPendingForUser(ref.UserID, orgDB); err == nil && latest != nil {
		if time.Since(latest.CreatedAt) < s.settings.ResendCooldown() {
			return
		}
	}
	if err := s.recoveryStore.DeletePendingForUser(ref.UserID, orgDB); err != nil {
		s.log.Warning("app-recovery: purge pending: %v", err)
	}

	token, err := recovery.GenerateToken()
	if err != nil {
		s.log.Error("app-recovery: generate token: %v", err)
		return
	}
	expiresAt := time.Now().UTC().Add(s.settings.TTL())
	if err := s.recoveryStore.Insert(recovery.Row{
		UserID:    ref.UserID,
		TokenHash: recovery.HashToken(token),
		ExpiresAt: expiresAt,
	}, orgDB); err != nil {
		s.log.Error("app-recovery: insert row: %v", err)
		return
	}

	// The link goes to the per-app public reset page so the branded
	// HTML template handles the final step. Slug trio keeps UUIDs
	// out of the email body.
	baseURL := s.baseURLForOrg(info.OrganizationID)
	if baseURL == "" {
		s.log.Warning("app-recovery: base URL not configured — cannot build reset link")
		return
	}
	link := fmt.Sprintf("%s/recover/a/%s/%s/%s/reset?token=%s&email=%s",
		strings.TrimRight(baseURL, "/"),
		info.OrganizationSlug, info.WorkspaceSlug, info.ClientID,
		token, email)

	if err := s.sendMail(ctx, ref, link); err != nil {
		s.log.Error("app-recovery: failed to enqueue email for %s: %v", ref.UserID, err)
	}
}

// CompleteRecovery handles the POST /recover/reset flow. Returns:
//   nil              → password was reset
//   ErrRecoveryFailed → generic auth failure (token, email, ACL, …)
//   other error      → rule violation, surfaced verbatim
func (s *Service) CompleteRecovery(orgSlug, wsSlug, clientID, token, email, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("recovery: new password cannot be empty")
	}
	info, err := s.findAppBySlugs(orgSlug, wsSlug, clientID)
	if err != nil {
		return ErrRecoveryFailed
	}
	// Recovery gate (plan Q5). Consuming an existing token also
	// respects the current flag — stale links in inboxes become
	// no-ops after an admin flips the switch off.
	if !s.isRecoveryAllowed(info.ID) {
		return ErrRecoveryFailed
	}
	row, tokenOrgDB, err := s.recoveryStore.FindByTokenHash(recovery.HashToken(token))
	if err != nil {
		return ErrRecoveryFailed
	}
	if row.ConsumedAt != nil {
		return ErrRecoveryFailed
	}
	if row.ExpiresAt.Before(time.Now().UTC()) {
		return ErrRecoveryFailed
	}

	// Open the per-org DB for user/ACL and password operations.
	if s.orgRegistry == nil {
		return ErrRecoveryFailed
	}
	orgDB, err := orgrouter.ForOrg(s.orgRegistry, info.OrganizationID)
	if err != nil {
		return ErrRecoveryFailed
	}
	// tokenOrgDB must match the app's org DB (guards cross-org token reuse).
	if tokenOrgDB != orgDB {
		return ErrRecoveryFailed
	}

	// Refresh ACL + user state at reset time so a revocation between
	// request and reset takes effect immediately.
	var (
		username, storedEmail string
		isActive              bool
	)
	err = orgDB.QueryRow(
		`SELECT u.username, u.email, u.is_active
		 FROM users u
		 JOIN application_user_acl acl ON acl.user_id = u.id AND acl.is_active = TRUE
		 WHERE acl.application_id = ? AND u.id = ?
		 LIMIT 1`,
		info.ID, row.UserID,
	).Scan(&username, &storedEmail, &isActive)
	if err != nil || !isActive {
		return ErrRecoveryFailed
	}
	if !strings.EqualFold(strings.TrimSpace(storedEmail), strings.TrimSpace(email)) {
		return ErrRecoveryFailed
	}

	// Password rules for the user's organisation, via the shared
	// userrules store.
	var rs userrules.RuleSet
	if s.rules != nil {
		if got, rerr := s.rules.GetForUser(row.UserID); rerr == nil {
			rs = got
		} else {
			rs = userrules.Defaults()
		}
	} else {
		rs = userrules.Defaults()
	}
	if v := userrules.ValidatePassword(rs.Password, userrules.Input{
		Username: username,
		Email:    storedEmail,
		Password: newPassword,
	}); len(v) > 0 {
		return errors.New(strings.Join(v, " "))
	}

	hash, err := bcryptx.HashPassword(newPassword, bcryptx.DefaultCost)
	if err != nil {
		return fmt.Errorf("recovery: hash new password: %w", err)
	}
	if _, err := orgDB.Exec(
		`INSERT INTO user_auth_password (user_id, password, created_at, is_active)
		 VALUES (?, ?, CURRENT_TIMESTAMP, TRUE)
		 ON CONFLICT(user_id) DO UPDATE SET password = excluded.password, is_active = TRUE`,
		row.UserID, hash,
	); err != nil {
		return fmt.Errorf("recovery: upsert password: %w", err)
	}
	if _, err := orgDB.Exec(
		`UPDATE users SET must_change_password = FALSE WHERE id = ?`, row.UserID,
	); err != nil {
		return fmt.Errorf("recovery: clear must_change_password: %w", err)
	}
	if err := s.recoveryStore.ConsumeByID(row.ID, orgDB); err != nil {
		return err
	}
	return nil
}

// isRecoveryAllowed reads the per-app allow_recovery flag from the
// applications table in the per-org DB (allow_recovery moved with the
// applications entity). Defaults to true on any error.
func (s *Service) isRecoveryAllowed(appID string) bool {
	if s.orgRegistry == nil {
		return true
	}
	orgDB, _, err := orgrouter.OrgDBForApp(s.orgRegistry, appID)
	if err != nil {
		return true
	}
	var allow bool
	err = orgDB.QueryRow(
		`SELECT allow_recovery FROM applications WHERE id = ?`, appID,
	).Scan(&allow)
	if err != nil {
		return true
	}
	return allow
}

// -- email --------------------------------------------------------------

// baseURLForOrg opens the per-org DB for orgID and reads base_url from
// its app_settings table. Returns "" when the org DB cannot be resolved
// or the setting has not been configured.
func (s *Service) baseURLForOrg(orgID string) string {
	orgDB, err := orgrouter.ForOrg(s.orgRegistry, orgID)
	if err != nil {
		return ""
	}
	return general.NewStoreFromDB(orgDB).BaseURL()
}

func (s *Service) sendMail(_ context.Context, ref appUserRef, link string) error {
	template := s.mailConfig.TemplateForEvent(recovery.EventPasswordRecovery)
	if template == "" {
		return fmt.Errorf("no template bound to event %q — configure it in Admin Settings → Email",
			recovery.EventPasswordRecovery)
	}
	account := s.mailConfig.AccountForEvent(recovery.EventPasswordRecovery)
	if account == "" {
		return fmt.Errorf("no account bound to event %q — configure it in Admin Settings → Email",
			recovery.EventPasswordRecovery)
	}
	_, err := s.mail.Enqueue(iam_mail.MailTask{
		Template: template,
		Account:  account,
		To:       []iam_mail.Address{{Email: ref.Email, Name: ref.Username}},
		Data: map[string]interface{}{
			"Username":  ref.Username,
			"ResetLink": link,
			"ExpiresIn": s.settings.TTLHumanReadable(),
		},
	})
	return err
}
