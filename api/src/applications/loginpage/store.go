package loginpage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
)

type Store struct {
	mainDB *sql.DB
	reg    *dbregistry.OrgUserDBRegistry
}

// NewStore constructs a Store backed by the global main DB (for
// organization lookups and media_files) and the per-org registry
// (for workspace, applications, and all application_* tables that
// now live in the per-org DB).
func NewStore(mainDB *sql.DB, reg *dbregistry.OrgUserDBRegistry) *Store {
	return &Store{mainDB: mainDB, reg: reg}
}

var (
	ErrNotFound = errors.New("loginpage: not found")
)

// orgDBForApp resolves the per-org DB for the given application id by
// scanning all known per-org DBs via the registry.
func (s *Store) orgDBForApp(appID string) (*sql.DB, error) {
	db, _, err := orgrouter.OrgDBForApp(s.reg, appID)
	if err != nil {
		return nil, ErrNotFound
	}
	return db, nil
}

// orgDBForOrg opens the per-org DB for the given org UUID.
func (s *Store) orgDBForOrg(orgID string) (*sql.DB, error) {
	return orgrouter.ForOrg(s.reg, orgID)
}

// -- lookups --

// FindByIDs resolves (workspaceID, appID) UUIDs to the app row. Used
// by admin contexts that already hold the UUIDs. Missing -> ErrNotFound.
func (s *Store) FindByIDs(wsID, appID string) (AppInfo, error) {
	// Resolve orgDB by scanning per-org DBs.
	orgDB, orgID, err := orgrouter.OrgDBForApp(s.reg, appID)
	if err != nil {
		return AppInfo{}, ErrNotFound
	}

	// Application + workspace slugs from per-org DB.
	var info AppInfo
	var wsDBID string
	var wsSlug, wsTitle, appClientID, appTitle string
	if err := orgDB.QueryRow(
		`SELECT a.id, a.client_id, a.workspace_id, a.title,
		        w.workspace_id, w.title
		 FROM applications a
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE a.workspace_id = ? AND a.id = ? LIMIT 1`,
		wsID, appID,
	).Scan(&info.ID, &appClientID, &wsDBID, &appTitle, &wsSlug, &wsTitle); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppInfo{}, ErrNotFound
		}
		return AppInfo{}, fmt.Errorf("loginpage: find by ids: %w", err)
	}

	// Org slug from main DB.
	var orgSlug string
	_ = s.mainDB.QueryRow(
		`SELECT organization_id FROM organization WHERE id = ? LIMIT 1`, orgID,
	).Scan(&orgSlug)

	info.ClientID = appClientID
	info.WorkspaceID = wsDBID
	info.ApplicationName = appTitle
	info.WorkspaceSlug = wsSlug
	info.WorkspaceName = wsTitle
	info.OrganizationID = orgID
	info.OrganizationSlug = orgSlug
	return info, nil
}

// SlugTriple is the (organization, workspace, client) admin-chosen
// identifier set that is used to build public URLs. Admins see these;
// end users never see the corresponding UUIDs.
type SlugTriple struct {
	OrganizationSlug string
	WorkspaceSlug    string
	ClientID         string
}

// LookupSlugsByAppID returns the slug trio for the application row
// identified by appUUID. Used by the admin settings endpoint to
// return a pre-built /login/a/<org>/<ws>/<app> URL to the UI.
func (s *Store) LookupSlugsByAppID(appUUID string) (SlugTriple, error) {
	// Resolve orgDB + orgID by scanning per-org DBs.
	orgDB, orgID, err := orgrouter.OrgDBForApp(s.reg, appUUID)
	if err != nil {
		return SlugTriple{}, ErrNotFound
	}

	// Org slug from main DB.
	var orgSlug string
	if err := s.mainDB.QueryRow(
		`SELECT organization_id FROM organization WHERE id = ? LIMIT 1`, orgID,
	).Scan(&orgSlug); err != nil {
		return SlugTriple{}, fmt.Errorf("loginpage: lookup slugs: org slug: %w", err)
	}
	var wsID, wsSlug, clientID string
	if err := orgDB.QueryRow(
		`SELECT a.client_id, w.workspace_id
		 FROM applications a
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE a.id = ? LIMIT 1`,
		appUUID,
	).Scan(&clientID, &wsSlug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SlugTriple{}, ErrNotFound
		}
		return SlugTriple{}, fmt.Errorf("loginpage: lookup slugs: %w", err)
	}
	_ = wsID
	return SlugTriple{
		OrganizationSlug: orgSlug,
		WorkspaceSlug:    wsSlug,
		ClientID:         clientID,
	}, nil
}

// FindBySlugs resolves (organization_id, workspace_id, client_id) slug
// trio to the app row. This is the lookup path used by all public
// endpoints. Missing -> ErrNotFound.
func (s *Store) FindBySlugs(orgSlug, wsSlug, clientID string) (AppInfo, error) {
	// Resolve org UUID from main DB.
	var orgID string
	if err := s.mainDB.QueryRow(
		`SELECT id FROM organization WHERE organization_id = ? LIMIT 1`,
		orgSlug,
	).Scan(&orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppInfo{}, ErrNotFound
		}
		return AppInfo{}, fmt.Errorf("loginpage: find by slugs: resolve org: %w", err)
	}

	// Open per-org DB.
	orgDB, err := orgrouter.ForOrg(s.reg, orgID)
	if err != nil {
		return AppInfo{}, fmt.Errorf("loginpage: find by slugs: open org db: %w", err)
	}

	// Application + workspace from per-org DB using slug columns.
	var info AppInfo
	var wsDBID string
	if err := orgDB.QueryRow(
		`SELECT a.id, a.client_id, a.workspace_id, a.title,
		        w.workspace_id, w.title
		 FROM applications a
		 JOIN workspace w ON w.id = a.workspace_id
		 WHERE w.workspace_id = ? AND a.client_id = ? LIMIT 1`,
		wsSlug, clientID,
	).Scan(&info.ID, &info.ClientID, &wsDBID, &info.ApplicationName,
		&info.WorkspaceSlug, &info.WorkspaceName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AppInfo{}, ErrNotFound
		}
		return AppInfo{}, fmt.Errorf("loginpage: find by slugs: %w", err)
	}
	info.WorkspaceID = wsDBID
	info.OrganizationID = orgID
	info.OrganizationSlug = orgSlug
	return info, nil
}

// -- ACL-scoped user lookup for login --

// FindUserForLogin returns the user_id (from users) that matches
// the given identifier (username or email) AND is on the
// application user ACL (direct row). Both tables live in the
// per-org DB; pass orgDB from orgrouter.ForOrg. Missing -> ErrNotFound.
func (s *Store) FindUserForLogin(orgDB *sql.DB, appID, identifier string) (string, error) {
	var id string
	err := orgDB.QueryRow(
		`SELECT u.id FROM users u
		 JOIN application_user_acl acl ON acl.user_id = u.id
		 WHERE acl.application_id = ?
		   AND acl.is_active = 1
		   AND u.is_active = 1
		   AND (LOWER(u.username) = LOWER(?) OR LOWER(u.email) = LOWER(?))
		 LIMIT 1`,
		appID, identifier, identifier,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("loginpage: find user for login: %w", err)
	}
	return id, nil
}

// FindOAuthClientForDispatch returns the application-scoped OAuth
// client for the given dispatch target. application_oauth_clients
// now lives in the per-org DB.
func (s *Store) FindOAuthClientForDispatch(appID, clientRowID string) (redirectURIs []string, active bool, err error) {
	orgDB, e := s.orgDBForApp(appID)
	if e != nil {
		return nil, false, fmt.Errorf("loginpage: find oauth client: resolve db: %w", e)
	}
	var urisJSON string
	var isActive bool
	e = orgDB.QueryRow(
		`SELECT redirect_uris, is_active
		 FROM application_oauth_clients
		 WHERE id = ? AND application_id = ?
		 LIMIT 1`,
		clientRowID, appID,
	).Scan(&urisJSON, &isActive)
	if e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return nil, false, ErrNotFound
		}
		return nil, false, fmt.Errorf("loginpage: find oauth client: %w", e)
	}
	var uris []string
	if urisJSON != "" {
		if je := json.Unmarshal([]byte(urisJSON), &uris); je != nil {
			return nil, false, fmt.Errorf("loginpage: parse oauth client redirect_uris: %w", je)
		}
	}
	return uris, isActive, nil
}

// -- assets --

func (s *Store) InsertAsset(a Asset) error {
	orgDB, err := s.orgDBForApp(a.ApplicationID)
	if err != nil {
		return fmt.Errorf("loginpage: insert asset: resolve db: %w", err)
	}
	if a.ID == "" {
		a.ID = newID()
	}
	if a.Kind == "" {
		a.Kind = AssetKindOther
	}
	_, err = orgDB.Exec(
		`INSERT INTO application_login_assets
		  (id, application_id, file_path, mime_type, size_bytes, kind, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		a.ID, a.ApplicationID, a.FilePath, a.MimeType, a.SizeBytes, string(a.Kind),
	)
	if err != nil {
		return fmt.Errorf("loginpage: insert asset: %w", err)
	}
	return nil
}

func (s *Store) ListAssets(appID string) ([]Asset, error) {
	orgDB, err := s.orgDBForApp(appID)
	if err != nil {
		return nil, fmt.Errorf("loginpage: list assets: resolve db: %w", err)
	}
	rows, err := orgDB.Query(
		`SELECT id, application_id, file_path, mime_type, size_bytes, kind, created_at
		 FROM application_login_assets WHERE application_id = ?
		 ORDER BY created_at DESC`, appID,
	)
	if err != nil {
		return nil, fmt.Errorf("loginpage: list assets: %w", err)
	}
	defer rows.Close()
	var out []Asset
	for rows.Next() {
		var a Asset
		var kind string
		var created sql.NullString
		if err := rows.Scan(&a.ID, &a.ApplicationID, &a.FilePath, &a.MimeType, &a.SizeBytes, &kind, &created); err != nil {
			return nil, fmt.Errorf("loginpage: scan asset: %w", err)
		}
		a.Kind = AssetKind(kind)
		if created.Valid {
			if p, pe := parseTime(created.String); pe == nil {
				a.CreatedAt = p
			}
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) FindAsset(id string) (Asset, error) {
	// Asset lookup requires knowing the appID first. We query the main
	// routing index by asset id - but assets are in per-org DB. We need
	// to find the asset in some org DB. Since we don't have appID here,
	// we scan all orgs (acceptable - this is called rarely, for delete/serve).
	// Better: check application_org_index after querying a universal search.
	// Simplest for now: try all known orgs.
	// Actually we can do a 2-step: query all orgs that might have this asset
	// using the known org IDs from the main routing index.
	// For correctness: iterate over org IDs known to registry and query each.
	// This is a rare operation so the cost is acceptable.
	orgIDs, err := s.allOrgIDs()
	if err != nil {
		return Asset{}, fmt.Errorf("loginpage: find asset: list orgs: %w", err)
	}
	for _, orgID := range orgIDs {
		orgDB, err := orgrouter.ForOrg(s.reg, orgID)
		if err != nil {
			continue
		}
		var a Asset
		var kind string
		var created sql.NullString
		e := orgDB.QueryRow(
			`SELECT id, application_id, file_path, mime_type, size_bytes, kind, created_at
			 FROM application_login_assets WHERE id = ?`, id,
		).Scan(&a.ID, &a.ApplicationID, &a.FilePath, &a.MimeType, &a.SizeBytes, &kind, &created)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				continue
			}
			return Asset{}, fmt.Errorf("loginpage: load asset: %w", e)
		}
		a.Kind = AssetKind(kind)
		if created.Valid {
			if p, pe := parseTime(created.String); pe == nil {
				a.CreatedAt = p
			}
		}
		return a, nil
	}
	return Asset{}, ErrNotFound
}

func (s *Store) DeleteAsset(id string) (string, error) {
	a, err := s.FindAsset(id)
	if err != nil {
		return "", err
	}
	orgDB, err := s.orgDBForApp(a.ApplicationID)
	if err != nil {
		return "", fmt.Errorf("loginpage: delete asset: resolve db: %w", err)
	}
	if _, err := orgDB.Exec(
		`DELETE FROM application_login_assets WHERE id = ?`, id,
	); err != nil {
		return "", fmt.Errorf("loginpage: delete asset: %w", err)
	}
	return a.FilePath, nil
}

// allOrgIDs returns all org UUIDs currently known to the registry.
func (s *Store) allOrgIDs() ([]string, error) {
	return s.reg.KnownOrgIDs(), nil
}

// -- settings --

func (s *Store) GetSettings(appID string) (Settings, error) {
	orgDB, err := s.orgDBForApp(appID)
	if err != nil {
		// On routing miss return sensible defaults (app may not exist yet)
		return Settings{
			ApplicationID:           appID,
			RedirectMethod:          "POST",
			CustomHeaders:           map[string]string{},
			TemplateKind:            TemplateCentered1Col,
			BackgroundColor:         "#f9fafb",
			BackgroundGradientAngle: 135,
			ShowLogo:                true,
		}, nil
	}
	out := Settings{
		ApplicationID:           appID,
		RedirectMethod:          "POST",
		CustomHeaders:           map[string]string{},
		TemplateKind:            TemplateCentered1Col,
		BackgroundColor:         "#f9fafb",
		BackgroundGradientAngle: 135,
		ShowLogo:                true,
	}
	var headersJSON string
	var updated sql.NullString
	var bgAsset sql.NullString
	var richTextDefaultsJSON sql.NullString
	var oauthClientID sql.NullString
	err = orgDB.QueryRow(
		`SELECT redirect_url, redirect_method, redirect_secret, custom_headers,
		        template_kind, background_color, background_asset_id,
		        background_gradient_from, background_gradient_to, background_gradient_angle,
		        show_logo, page_title, brand_text,
		        rich_text_defaults, updated_at, oauth_client_id
		 FROM application_login_settings WHERE application_id = ?`, appID,
	).Scan(
		&out.RedirectURL, &out.RedirectMethod, &out.RedirectSecret, &headersJSON,
		&out.TemplateKind, &out.BackgroundColor, &bgAsset,
		&out.BackgroundGradientFrom, &out.BackgroundGradientTo, &out.BackgroundGradientAngle,
		&out.ShowLogo, &out.PageTitle, &out.BrandText,
		&richTextDefaultsJSON, &updated, &oauthClientID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return out, fmt.Errorf("loginpage: load settings: %w", err)
	}
	if headersJSON != "" {
		headers := map[string]string{}
		if e := json.Unmarshal([]byte(headersJSON), &headers); e == nil {
			out.CustomHeaders = headers
		}
	}
	if bgAsset.Valid && bgAsset.String != "" {
		v := bgAsset.String
		out.BackgroundAssetID = &v
	}
	if oauthClientID.Valid && oauthClientID.String != "" {
		v := oauthClientID.String
		out.OAuthClientID = &v
	}
	if updated.Valid {
		if p, pe := parseTime(updated.String); pe == nil {
			out.UpdatedAt = p
		}
	}
	if richTextDefaultsJSON.Valid && richTextDefaultsJSON.String != "" {
		_ = json.Unmarshal([]byte(richTextDefaultsJSON.String), &out.RichTextDefaults)
	}

	// Per-column overrides (0..N). Empty slice when no rows exist.
	cols, err := s.loadColumns(orgDB, appID)
	if err != nil {
		return out, fmt.Errorf("loginpage: load columns: %w", err)
	}
	out.Columns = cols
	return out, nil
}

// loadColumns reads the per-column overrides for an application,
// ordered by column_index. Nil-valued DB columns map to nil pointers
// on ColumnConfig so the admin UI can distinguish "unset" from
// "explicitly empty".
func (s *Store) loadColumns(orgDB *sql.DB, appID string) ([]ColumnConfig, error) {
	rows, err := orgDB.Query(
		`SELECT column_index, background_color, background_asset_id,
		        background_gradient_from, background_gradient_to,
		        background_gradient_angle, text_color_override,
		        text_block_title, text_contents
		 FROM application_login_columns
		 WHERE application_id = ?
		 ORDER BY column_index ASC`,
		appID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ColumnConfig, 0, 2)
	for rows.Next() {
		var (
			c          ColumnConfig
			color      sql.NullString
			asset      sql.NullString
			gradFrom   sql.NullString
			gradTo     sql.NullString
			angle      sql.NullInt64
			textCol    sql.NullString
			blockTitle sql.NullString
			contentsS  sql.NullString
		)
		if err := rows.Scan(
			&c.ColumnIndex, &color, &asset, &gradFrom, &gradTo, &angle,
			&textCol, &blockTitle, &contentsS,
		); err != nil {
			return nil, err
		}
		if color.Valid {
			v := color.String
			c.BackgroundColor = &v
		}
		if asset.Valid {
			v := asset.String
			c.BackgroundAssetID = &v
		}
		if gradFrom.Valid {
			v := gradFrom.String
			c.BackgroundGradientFrom = &v
		}
		if gradTo.Valid {
			v := gradTo.String
			c.BackgroundGradientTo = &v
		}
		if angle.Valid {
			v := int(angle.Int64)
			c.BackgroundGradientAngle = &v
		}
		if textCol.Valid {
			v := textCol.String
			c.TextColorOverride = &v
		}
		if blockTitle.Valid {
			v := blockTitle.String
			c.TextBlockTitle = &v
		}
		if contentsS.Valid && contentsS.String != "" {
			_ = json.Unmarshal([]byte(contentsS.String), &c.TextContents)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertSettings writes the template/layout row. The allow_recovery /
// allow_registration flags live on the applications entity (central
// to the app), so this write path has no concept of feature toggles.
func (s *Store) UpsertSettings(in Settings) error {
	if in.ApplicationID == "" {
		return errors.New("loginpage: application_id required")
	}
	orgDB, err := s.orgDBForApp(in.ApplicationID)
	if err != nil {
		return fmt.Errorf("loginpage: upsert settings: resolve db: %w", err)
	}
	headers := in.CustomHeaders
	if headers == nil {
		headers = map[string]string{}
	}
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("loginpage: marshal headers: %w", err)
	}
	var bgAsset interface{}
	if in.BackgroundAssetID != nil {
		bgAsset = *in.BackgroundAssetID
	}
	var oauthClientID interface{}
	if in.OAuthClientID != nil {
		oauthClientID = *in.OAuthClientID
	}
	richTextDefaultsJSON, err := json.Marshal(in.RichTextDefaults)
	if err != nil {
		return fmt.Errorf("loginpage: marshal rich_text_defaults: %w", err)
	}
	_, err = orgDB.Exec(
		`INSERT INTO application_login_settings
		   (application_id, redirect_url, redirect_method, redirect_secret, custom_headers,
		    template_kind, background_color, background_asset_id,
		    background_gradient_from, background_gradient_to, background_gradient_angle,
		    show_logo, page_title, brand_text,
		    rich_text_defaults, oauth_client_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(application_id) DO UPDATE SET
		   redirect_url              = excluded.redirect_url,
		   redirect_method           = excluded.redirect_method,
		   redirect_secret           = excluded.redirect_secret,
		   custom_headers            = excluded.custom_headers,
		   template_kind             = excluded.template_kind,
		   background_color          = excluded.background_color,
		   background_asset_id       = excluded.background_asset_id,
		   background_gradient_from  = excluded.background_gradient_from,
		   background_gradient_to    = excluded.background_gradient_to,
		   background_gradient_angle = excluded.background_gradient_angle,
		   show_logo                 = excluded.show_logo,
		   page_title                = excluded.page_title,
		   brand_text                = excluded.brand_text,
		   rich_text_defaults        = excluded.rich_text_defaults,
		   oauth_client_id           = excluded.oauth_client_id,
		   updated_at                = CURRENT_TIMESTAMP`,
		in.ApplicationID, in.RedirectURL, in.RedirectMethod, in.RedirectSecret, string(headersJSON),
		string(in.TemplateKind), in.BackgroundColor, bgAsset,
		in.BackgroundGradientFrom, in.BackgroundGradientTo, in.BackgroundGradientAngle,
		in.ShowLogo, in.PageTitle, in.BrandText,
		string(richTextDefaultsJSON), oauthClientID,
	)
	if err != nil {
		return fmt.Errorf("loginpage: upsert settings: %w", err)
	}

	// Per-column overrides. The callers slice is authoritative:
	// rows for columns not present in the slice are deleted so the
	// admin can drop a customisation by omission.
	if err := s.replaceColumns(orgDB, in.ApplicationID, in.Columns); err != nil {
		return fmt.Errorf("loginpage: upsert columns: %w", err)
	}
	return nil
}

// replaceColumns makes the set of rows in application_login_columns
// match the caller-provided slice exactly: column indices present in
// the slice are upserted, any others on disk are deleted.
func (s *Store) replaceColumns(orgDB *sql.DB, appID string, cols []ColumnConfig) error {
	// Upsert every incoming column.
	for _, c := range cols {
		contents := c.TextContents
		if contents == nil {
			contents = []ColumnTextContent{}
		}
		contentsJSON, err := json.Marshal(contents)
		if err != nil {
			return fmt.Errorf("marshal text_contents: %w", err)
		}
		if _, err := orgDB.Exec(
			`INSERT INTO application_login_columns
			   (application_id, column_index,
			    background_color, background_asset_id,
			    background_gradient_from, background_gradient_to,
			    background_gradient_angle, text_color_override,
			    text_block_title, text_contents, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(application_id, column_index) DO UPDATE SET
			   background_color          = excluded.background_color,
			   background_asset_id       = excluded.background_asset_id,
			   background_gradient_from  = excluded.background_gradient_from,
			   background_gradient_to    = excluded.background_gradient_to,
			   background_gradient_angle = excluded.background_gradient_angle,
			   text_color_override       = excluded.text_color_override,
			   text_block_title          = excluded.text_block_title,
			   text_contents             = excluded.text_contents,
			   updated_at                = CURRENT_TIMESTAMP`,
			appID, c.ColumnIndex,
			nullable(c.BackgroundColor), nullable(c.BackgroundAssetID),
			nullable(c.BackgroundGradientFrom), nullable(c.BackgroundGradientTo),
			nullableInt(c.BackgroundGradientAngle), nullable(c.TextColorOverride),
			nullable(c.TextBlockTitle), string(contentsJSON),
		); err != nil {
			return err
		}
	}
	// Delete any row whose column_index isn't in the incoming set.
	if len(cols) == 0 {
		_, err := orgDB.Exec(`DELETE FROM application_login_columns WHERE application_id = ?`, appID)
		return err
	}
	keep := make([]interface{}, 0, len(cols)+1)
	keep = append(keep, appID)
	ph := "?"
	for i, c := range cols {
		if i > 0 {
			ph += ",?"
		}
		keep = append(keep, c.ColumnIndex)
	}
	_, err := orgDB.Exec(
		`DELETE FROM application_login_columns WHERE application_id = ? AND column_index NOT IN (`+ph+`)`,
		keep...,
	)
	return err
}

func nullable(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func nullableInt(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// FindLogoFilename returns the filename of the media file that
// represents the application logo. media_files still lives in the
// global DB. Returns "" when no logo is set (never an error).
func (s *Store) FindLogoFilename(appID string) (string, error) {
	var filename string
	err := s.mainDB.QueryRow(
		`SELECT filename FROM media_files
		 WHERE owner_id = ?
		   AND folder_id IS NULL
		   AND filename LIKE 'logo.%'
		 ORDER BY created_at DESC
		 LIMIT 1`,
		appID,
	).Scan(&filename)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("loginpage: find logo filename: %w", err)
	}
	return filename, nil
}

// IsRecoveryAllowed reads the per-app flag from the applications row
// in the per-org DB. Defaults to true when the row cannot be loaded.
func (s *Store) IsRecoveryAllowed(appID string) (bool, error) {
	orgDB, err := s.orgDBForApp(appID)
	if err != nil {
		return true, nil
	}
	var allow bool
	err = orgDB.QueryRow(
		`SELECT allow_recovery FROM applications WHERE id = ?`, appID,
	).Scan(&allow)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, fmt.Errorf("loginpage: load allow_recovery: %w", err)
	}
	return allow, nil
}

// IsRegistrationAllowed reads the per-app flag from the applications row
// in the per-org DB. Defaults to false — registration is opt-in.
func (s *Store) IsRegistrationAllowed(appID string) (bool, error) {
	orgDB, err := s.orgDBForApp(appID)
	if err != nil {
		return false, nil
	}
	var allow bool
	err = orgDB.QueryRow(
		`SELECT allow_registration FROM applications WHERE id = ?`, appID,
	).Scan(&allow)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("loginpage: load allow_registration: %w", err)
	}
	return allow, nil
}

// AssetIsForApp reports whether the asset exists AND belongs to the
// given application. Used to validate write-path references.
func (s *Store) AssetIsForApp(assetID, appID string) (bool, error) {
	orgDB, err := s.orgDBForApp(appID)
	if err != nil {
		return false, fmt.Errorf("loginpage: asset is for app: resolve db: %w", err)
	}
	var owner string
	err = orgDB.QueryRow(
		`SELECT application_id FROM application_login_assets WHERE id = ?`,
		assetID,
	).Scan(&owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return owner == appID, nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("loginpage: unparseable timestamp %q", s)
}
