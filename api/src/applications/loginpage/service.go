package loginpage

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Service is the single handler-facing facade that wires Store + FileStore.
type Service struct {
	Store *Store
	Files *FileStore
}

func NewService(store *Store, files *FileStore) *Service {
	return &Service{Store: store, Files: files}
}

var hexColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateSettings is the shared guard for the settings PATCH and for
// the authenticate-time "is this app actually configured" gate. Missing
// values that represent an intentional "not configured yet" state are
// still allowed, but malformed values are rejected hard.
//
// The two toggle fields (AllowRecovery, AllowRegistration) are NOT
// validated here — they are intentionally ignored by the full settings
// write path and only mutated via the dedicated toggle endpoints.
func validateSettings(s *Settings) error {
	// Redirect bits (existing rules)
	s.RedirectURL = strings.TrimSpace(s.RedirectURL)
	s.RedirectMethod = strings.ToUpper(strings.TrimSpace(s.RedirectMethod))
	if s.RedirectMethod == "" {
		s.RedirectMethod = "POST"
	}
	if s.RedirectMethod != "POST" && s.RedirectMethod != "GET" {
		return errors.New("redirect_method must be POST or GET")
	}
	if s.RedirectURL != "" {
		u, err := url.Parse(s.RedirectURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("redirect_url must be an absolute http(s) URL")
		}
	}
	if s.CustomHeaders == nil {
		s.CustomHeaders = map[string]string{}
	}
	reserved := map[string]struct{}{
		"authorization":  {},
		"x-login-secret": {},
		"x-renew-token":  {},
	}
	for k := range s.CustomHeaders {
		if _, bad := reserved[strings.ToLower(strings.TrimSpace(k))]; bad {
			return errors.New("custom headers may not override Authorization, X-Login-Secret, or X-Renew-Token")
		}
	}

	// Template / layout
	if s.TemplateKind == "" {
		s.TemplateKind = TemplateCentered1Col
	}
	if !IsValidTemplateKind(s.TemplateKind) {
		return fmt.Errorf("template_kind must be one of centered_1col, split_login_left, split_login_right")
	}
	s.BackgroundColor = strings.TrimSpace(s.BackgroundColor)
	if s.BackgroundColor == "" {
		s.BackgroundColor = "#f9fafb"
	}
	if !hexColorRegex.MatchString(s.BackgroundColor) {
		return errors.New("background_color must be a #rrggbb hex string")
	}

	// Gradient: either both stops are set (gradient enabled) or both
	// are empty (disabled). Half-configured gradients would render as
	// an opaque black wash, which is never what the admin wants.
	s.BackgroundGradientFrom = strings.TrimSpace(s.BackgroundGradientFrom)
	s.BackgroundGradientTo = strings.TrimSpace(s.BackgroundGradientTo)
	hasFrom := s.BackgroundGradientFrom != ""
	hasTo := s.BackgroundGradientTo != ""
	if hasFrom != hasTo {
		return errors.New("background_gradient_from and background_gradient_to must both be set or both be empty")
	}
	if hasFrom {
		if !hexColorRegex.MatchString(s.BackgroundGradientFrom) {
			return errors.New("background_gradient_from must be a #rrggbb hex string")
		}
		if !hexColorRegex.MatchString(s.BackgroundGradientTo) {
			return errors.New("background_gradient_to must be a #rrggbb hex string")
		}
	}
	if s.BackgroundGradientAngle < 0 || s.BackgroundGradientAngle > 360 {
		return errors.New("background_gradient_angle must be between 0 and 360")
	}
	if !hasFrom {
		// Normalise: keep the angle at its default when gradient is
		// disabled, so the JSON surface doesn't leak a stale rotation
		// an admin typed then cleared.
		s.BackgroundGradientAngle = 135
	}

	// Per-column overrides — same validation rules as the wrapper,
	// applied independently to each column. Nil pointers mean
	// "inherit the wrapper", so only non-nil fields are validated.
	for i := range s.Columns {
		if err := validateColumn(&s.Columns[i]); err != nil {
			return fmt.Errorf("column %d: %w", s.Columns[i].ColumnIndex, err)
		}
	}
	return nil
}

// validateColumn applies the same hex / gradient / angle rules the
// wrapper validateSettings uses, but to a nullable ColumnConfig.
func validateColumn(c *ColumnConfig) error {
	if c.BackgroundColor != nil {
		v := strings.TrimSpace(*c.BackgroundColor)
		if v != "" && !hexColorRegex.MatchString(v) {
			return errors.New("background_color must be a #rrggbb hex string")
		}
		if v == "" {
			c.BackgroundColor = nil
		} else {
			c.BackgroundColor = &v
		}
	}
	if c.TextColorOverride != nil {
		v := strings.TrimSpace(*c.TextColorOverride)
		if v != "" && !hexColorRegex.MatchString(v) {
			return errors.New("text_color_override must be a #rrggbb hex string")
		}
		if v == "" {
			c.TextColorOverride = nil
		} else {
			c.TextColorOverride = &v
		}
	}

	// Gradient: either both stops or neither (same rule as wrapper).
	hasFrom := c.BackgroundGradientFrom != nil && strings.TrimSpace(*c.BackgroundGradientFrom) != ""
	hasTo := c.BackgroundGradientTo != nil && strings.TrimSpace(*c.BackgroundGradientTo) != ""
	if hasFrom != hasTo {
		return errors.New("background_gradient_from and background_gradient_to must both be set or both be empty")
	}
	if hasFrom {
		if !hexColorRegex.MatchString(strings.TrimSpace(*c.BackgroundGradientFrom)) {
			return errors.New("background_gradient_from must be a #rrggbb hex string")
		}
		if !hexColorRegex.MatchString(strings.TrimSpace(*c.BackgroundGradientTo)) {
			return errors.New("background_gradient_to must be a #rrggbb hex string")
		}
	} else {
		// Clear stale fields so nothing leaks back on re-fetch.
		c.BackgroundGradientFrom = nil
		c.BackgroundGradientTo = nil
	}
	if c.BackgroundGradientAngle != nil {
		a := *c.BackgroundGradientAngle
		if a < 0 || a > 360 {
			return errors.New("background_gradient_angle must be between 0 and 360")
		}
	}

	// Title: trim; empty-after-trim → nil so the column cleanly
	// reports "no title".
	if c.TextBlockTitle != nil {
		v := strings.TrimSpace(*c.TextBlockTitle)
		if v == "" {
			c.TextBlockTitle = nil
		} else {
			c.TextBlockTitle = &v
		}
	}
	// Contents: trim each entry. An all-empty entry is preserved —
	// the admin may have just added it and be typing; drops happen
	// via the explicit remove button in the UI.
	for i := range c.TextContents {
		c.TextContents[i].Content = strings.TrimSpace(c.TextContents[i].Content)
	}
	return nil
}

// LoadSettings reads the settings for an application. Missing row → zero
// value with sensible defaults.
func (s *Service) LoadSettings(appID string) (Settings, error) {
	return s.Store.GetSettings(appID)
}

// SaveSettings validates then upserts the template/layout row.
// Feature toggles (allow_recovery, allow_registration) live on the
// `applications` entity and are edited through the standard
// application write path — not here.
func (s *Service) SaveSettings(in Settings) (Settings, error) {
	if err := validateSettings(&in); err != nil {
		return Settings{}, err
	}
	if in.BackgroundAssetID != nil {
		ok, err := s.Store.AssetIsForApp(*in.BackgroundAssetID, in.ApplicationID)
		if err != nil || !ok {
			return Settings{}, errors.New("background_asset_id does not belong to this application")
		}
	}
	// When an OAuth client is pinned as the dispatch target, verify it
	// exists for this application and is active. The redirect URI and
	// secret are resolved from the client at login time — no manual
	// URL/secret is required when a client is selected.
	if in.OAuthClientID != nil {
		_, active, err := s.Store.FindOAuthClientForDispatch(in.ApplicationID, *in.OAuthClientID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return Settings{}, errors.New("oauth_client_id does not exist for this application")
			}
			return Settings{}, fmt.Errorf("validate oauth client: %w", err)
		}
		if !active {
			return Settings{}, errors.New("the selected OAuth client is not active")
		}
	}
	if err := s.Store.UpsertSettings(in); err != nil {
		return Settings{}, err
	}
	return s.Store.GetSettings(in.ApplicationID)
}

// IsRecoveryAllowed is the canonical backend gate for the recovery
// service. Call this at both the request step (new token creation)
// and the reset step (token consumption) — see plan Q5.
func (s *Service) IsRecoveryAllowed(appID string) (bool, error) {
	return s.Store.IsRecoveryAllowed(appID)
}

// IsRegistrationAllowed is the canonical backend gate for any future
// self-registration endpoint (plan Q1).
func (s *Service) IsRegistrationAllowed(appID string) (bool, error) {
	return s.Store.IsRegistrationAllowed(appID)
}

// GetPublicConfig returns the render-only payload served to the
// /login/a/<org>/<ws>/<app> page. No secrets, no redirect URL, no
// custom headers. Lookup is slug-based so nothing in the public
// response ever leaks a UUID. Background comes from the login-assets
// subsystem via `assetURL`; the logo comes from the media subsystem
// via `mediaURL(orgSlug, wsSlug, clientID, filename)`. Feature flags
// (allow_recovery, allow_registration) are read from the central
// `applications` row.
func (s *Service) GetPublicConfig(
	orgSlug, wsSlug, clientID string,
	assetURL func(assetID string) string,
	mediaURL func(orgSlug, wsSlug, clientID, filename string) string,
) (PublicLoginConfig, error) {
	info, err := s.Store.FindBySlugs(orgSlug, wsSlug, clientID)
	if err != nil {
		return PublicLoginConfig{}, err
	}
	settings, err := s.Store.GetSettings(info.ID)
	if err != nil {
		return PublicLoginConfig{}, err
	}
	allowRecovery, _ := s.Store.IsRecoveryAllowed(info.ID)
	allowRegistration, _ := s.Store.IsRegistrationAllowed(info.ID)
	out := PublicLoginConfig{
		WorkspaceID:       info.WorkspaceID,
		ApplicationID:     info.ID,
		WorkspaceName:     info.WorkspaceName,
		ApplicationName:   info.ApplicationName,
		Configured:        settings.IsConfigured(),
		TemplateKind:      settings.TemplateKind,
		BackgroundColor:   settings.BackgroundColor,
		ShowLogo:          settings.ShowLogo,
		PageTitle:         settings.PageTitle,
		BrandText:         settings.BrandText,
		AllowRecovery:     allowRecovery,
		AllowRegistration: allowRegistration,
	}
	if settings.BackgroundAssetID != nil && assetURL != nil {
		out.BackgroundURL = assetURL(*settings.BackgroundAssetID)
	}
	if settings.BackgroundGradientFrom != "" && settings.BackgroundGradientTo != "" {
		out.BackgroundGradient = fmt.Sprintf(
			"linear-gradient(%ddeg, %s 0%%, %s 100%%)",
			settings.BackgroundGradientAngle,
			settings.BackgroundGradientFrom,
			settings.BackgroundGradientTo,
		)
	}
	if mediaURL != nil {
		if filename, _ := s.Store.FindLogoFilename(info.ID); filename != "" {
			out.LogoURL = mediaURL(info.OrganizationSlug, info.WorkspaceSlug, info.ClientID, filename)
		}
	}

	// Per-column overrides. Only fields the admin actually set land
	// on the public shape; unset fields surface as empty strings so
	// the FE inherits the wrapper background for that column.
	for _, c := range settings.Columns {
		pc := PublicColumnConfig{ColumnIndex: c.ColumnIndex}
		if c.BackgroundColor != nil {
			pc.BackgroundColor = *c.BackgroundColor
		}
		if c.BackgroundAssetID != nil && assetURL != nil {
			pc.BackgroundURL = assetURL(*c.BackgroundAssetID)
		}
		if c.BackgroundGradientFrom != nil && c.BackgroundGradientTo != nil {
			angle := 135
			if c.BackgroundGradientAngle != nil {
				angle = *c.BackgroundGradientAngle
			}
			pc.BackgroundGradient = fmt.Sprintf(
				"linear-gradient(%ddeg, %s 0%%, %s 100%%)",
				angle, *c.BackgroundGradientFrom, *c.BackgroundGradientTo,
			)
		}
		if c.TextColorOverride != nil {
			pc.TextColor = *c.TextColorOverride
		}
		if c.TextBlockTitle != nil {
			pc.TextBlockTitle = *c.TextBlockTitle
		}
		// Contents: render-only strings in order. Client id is
		// dropped — the public payload doesn't need stable keys.
		for _, e := range c.TextContents {
			pc.TextContents = append(pc.TextContents, e.Content)
		}
		out.Columns = append(out.Columns, pc)
	}
	return out, nil
}

// StoreAsset validates, persists to disk, and records the row. `kind`
// tags the asset for the admin UI — unknown or missing values default
// to AssetKindOther.
func (s *Service) StoreAsset(appID string, data []byte, claimedMime string, kind AssetKind) (Asset, error) {
	if int64(len(data)) > AssetCapBytes {
		return Asset{}, ErrTooLarge
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	mime, err := DetectAndValidateMime(head, claimedMime)
	if err != nil {
		return Asset{}, err
	}
	if !IsValidAssetKind(kind) {
		kind = AssetKindOther
	}
	ext := ExtForMime(mime)
	rel, err := s.Files.Write(appID, ext, data)
	if err != nil {
		return Asset{}, err
	}
	a := Asset{
		ID:            newID(),
		ApplicationID: appID,
		FilePath:      rel,
		MimeType:      mime,
		SizeBytes:     int64(len(data)),
		Kind:          kind,
	}
	if err := s.Store.InsertAsset(a); err != nil {
		_ = s.Files.Delete(rel)
		return Asset{}, err
	}
	return a, nil
}

func (s *Service) DeleteAsset(assetID string) error {
	rel, err := s.Store.DeleteAsset(assetID)
	if err != nil {
		return err
	}
	return s.Files.Delete(rel)
}

// ReadAsset returns (bytes, mime) for the public asset endpoint.
func (s *Service) ReadAsset(assetID string) ([]byte, string, error) {
	a, err := s.Store.FindAsset(assetID)
	if err != nil {
		return nil, "", err
	}
	data, err := s.Files.Read(a.FilePath)
	if err != nil {
		return nil, "", err
	}
	return data, a.MimeType, nil
}
