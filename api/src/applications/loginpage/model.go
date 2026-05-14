// Package loginpage implements per-application branded login pages.
// Admins pick one of a fixed catalogue of layouts and edit typed
// settings (background, logo, title, brand, text block). The public
// endpoint serves a JSON payload; the frontend renders it with a
// React component — no HTML injection anywhere.
package loginpage

import "time"

const ContextBagKeyService = "applications.loginpage.Service"

// TemplateKind enumerates the fixed catalogue of layouts. Admins
// choose one; there is no custom HTML path.
type TemplateKind string

const (
	TemplateCentered1Col    TemplateKind = "centered_1col"
	TemplateSplitLoginLeft  TemplateKind = "split_login_left"
	TemplateSplitLoginRight TemplateKind = "split_login_right"
)

// IsValidTemplateKind guards the write path against typos / bad input.
func IsValidTemplateKind(k TemplateKind) bool {
	switch k {
	case TemplateCentered1Col, TemplateSplitLoginLeft, TemplateSplitLoginRight:
		return true
	}
	return false
}

// AssetKind tags uploaded assets so the admin UI can show the right
// picker (background vs. logo) without loading everything at once.
type AssetKind string

const (
	AssetKindBackground AssetKind = "background"
	AssetKindLogo       AssetKind = "logo"
	AssetKindOther      AssetKind = "other"
)

// IsValidAssetKind gates the upload endpoint.
func IsValidAssetKind(k AssetKind) bool {
	switch k {
	case AssetKindBackground, AssetKindLogo, AssetKindOther:
		return true
	}
	return false
}

// Asset is an uploaded binary (background / logo / other).
type Asset struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	FilePath      string    `json:"-"` // never leaked to clients
	MimeType      string    `json:"mime_type"`
	SizeBytes     int64     `json:"size_bytes"`
	Kind          AssetKind `json:"kind"`
	CreatedAt     time.Time `json:"created_at"`
}

// AppInfo is the small subset returned by ID/slug lookups. The UUID
// trio (OrganizationID, WorkspaceID, ID) is used by the auth layer to
// key ACLs + tokens; the slug trio (OrganizationSlug, WorkspaceSlug,
// ClientID) is used by URL-builders (logo, login, recovery mail link).
type AppInfo struct {
	ID                string
	WorkspaceID       string
	OrganizationID    string
	WorkspaceName     string
	ApplicationName   string
	OrganizationSlug  string
	WorkspaceSlug     string
	ClientID          string
}

// Settings is the full per-application login configuration. Combines
// redirect-dispatch config (existing) with the new layout settings.
type Settings struct {
	ApplicationID string `json:"application_id"`

	// Redirect (existing — kept as-is)
	RedirectURL    string            `json:"redirect_url"`
	RedirectMethod string            `json:"redirect_method"` // "POST" | "GET"
	RedirectSecret string            `json:"redirect_secret"`
	CustomHeaders  map[string]string `json:"custom_headers"`

	// OAuthClientID pins the dispatch target to a registered OAuth
	// client. When set, SaveSettings enforces that RedirectURL is one
	// of that client's registered redirect_uris. Nil means the URL is
	// set manually (existing behaviour).
	OAuthClientID *string `json:"oauth_client_id"`

	// Template / layout. The logo is no longer stored here — it lives
	// in the media subsystem under filename `logo.<ext>` at the app's
	// root folder (see plan/application-detail/plan.md).
	TemplateKind      TemplateKind `json:"template_kind"`
	BackgroundColor   string       `json:"background_color"`
	BackgroundAssetID *string      `json:"background_asset_id"`
	// Gradient background. When both `from` and `to` are set the
	// gradient is applied as a CSS linear-gradient at `angle` degrees.
	// Precedence (highest first) in the public render: image,
	// gradient, solid color.
	BackgroundGradientFrom  string `json:"background_gradient_from"`
	BackgroundGradientTo    string `json:"background_gradient_to"`
	BackgroundGradientAngle int    `json:"background_gradient_angle"`
	ShowLogo                bool   `json:"show_logo"`
	PageTitle               string `json:"page_title"`
	BrandText               string `json:"brand_text"`

	// Per-column overrides. Only meaningful for split templates (0 =
	// left, 1 = right). Every field is nullable — an unset field
	// inherits from the wrapper-level background.
	Columns []ColumnConfig `json:"columns"`

	// RichTextDefaults stores per-editor toolbar state (colour /
	// font size / margin) keyed by the editor's id (e.g.
	// "wrapper.body", "col.0.title"). Each editor instance persists
	// its own settings so picking a colour in one doesn't bleed into
	// the others.
	RichTextDefaults map[string]RichTextDefaults `json:"rich_text_defaults"`

	UpdatedAt time.Time `json:"updated_at"`
}

// RichTextDefaults is the persisted toolbar state for a single
// WYSIWYG editor. All fields are optional; empty strings mean "use
// the editor's built-in defaults".
type RichTextDefaults struct {
	ForegroundColor string `json:"foreground_color,omitempty"`
	FontSize        string `json:"font_size,omitempty"`
	BlockMargin     string `json:"block_margin,omitempty"`
}

// ColumnConfig is the admin-write shape for a single login-template
// column override.
type ColumnConfig struct {
	ColumnIndex             int     `json:"column_index"`
	BackgroundColor         *string `json:"background_color"`
	BackgroundAssetID       *string `json:"background_asset_id"`
	BackgroundGradientFrom  *string `json:"background_gradient_from"`
	BackgroundGradientTo    *string `json:"background_gradient_to"`
	BackgroundGradientAngle *int    `json:"background_gradient_angle"`
	TextColorOverride       *string `json:"text_color_override"`
	// Per-column side-panel text: a single title plus an ordered
	// list of content entries. Both are optional. Empty title plus
	// an empty list renders no side panel.
	TextBlockTitle *string             `json:"text_block_title"`
	TextContents   []ColumnTextContent `json:"text_contents"`
}

// ColumnTextContent is one content entry in a text column's list.
// `ID` is a client-generated UUID that stays with the entry across
// saves so the RichTextEditor's toolbar defaults (keyed by editor
// id) follow it through reorders / removes.
type ColumnTextContent struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

// IsConfigured is the one gate that decides whether `/login/a/:ws/:app`
// will accept credentials. An OAuth client pin is self-sufficient —
// the redirect URI and secret are resolved from the client at dispatch
// time, so no manual URL/secret is required.
func (s Settings) IsConfigured() bool {
	if s.OAuthClientID != nil {
		return true
	}
	return s.RedirectURL != "" && s.RedirectSecret != "" &&
		(s.RedirectMethod == "POST" || s.RedirectMethod == "GET")
}

// PublicLoginConfig is the render-only payload served to unauthenticated
// visitors of /login/a/:ws/:app. No secrets, no redirect URL, no headers.
type PublicLoginConfig struct {
	WorkspaceID     string `json:"workspace_id"`
	ApplicationID   string `json:"application_id"`
	WorkspaceName   string `json:"workspace_name"`
	ApplicationName string `json:"application_name"`
	Configured      bool   `json:"configured"`

	TemplateKind      TemplateKind `json:"template_kind"`
	BackgroundColor   string       `json:"background_color"`
	BackgroundURL     string       `json:"background_url,omitempty"`
	// Pre-computed CSS linear-gradient, e.g.
	// "linear-gradient(135deg, #6366f1, #ec4899)". Empty when the
	// admin hasn't configured one. The FE drops this verbatim into
	// the wrapper's background-image so no CSS is synthesised client
	// side.
	BackgroundGradient string       `json:"background_gradient,omitempty"`
	LogoURL            string       `json:"logo_url,omitempty"`
	ShowLogo          bool         `json:"show_logo"`
	PageTitle         string       `json:"page_title"`
	BrandText         string       `json:"brand_text"`
	AllowRecovery     bool         `json:"allow_recovery"`
	AllowRegistration bool         `json:"allow_registration"`

	// Per-column backgrounds, pre-composed for direct FE consumption.
	// Empty when no column overrides exist; unset fields on a column
	// mean "inherit wrapper background".
	Columns []PublicColumnConfig `json:"columns,omitempty"`
}

// PublicColumnConfig is the render-only per-column payload. Image and
// gradient are pre-resolved so the FE drops strings verbatim into the
// column's backgroundImage / backgroundColor style. Side-panel text is
// a single title plus an ordered list of content strings.
type PublicColumnConfig struct {
	ColumnIndex        int    `json:"column_index"`
	BackgroundColor    string `json:"background_color,omitempty"`
	BackgroundURL      string `json:"background_url,omitempty"`
	BackgroundGradient string `json:"background_gradient,omitempty"`
	TextColor          string `json:"text_color,omitempty"`
	// Optional single title shown above the content list.
	TextBlockTitle string `json:"text_block_title,omitempty"`
	// Ordered list of HTML content strings. Empty list + empty title
	// → no text panel rendered.
	TextContents []string `json:"text_contents,omitempty"`
}
