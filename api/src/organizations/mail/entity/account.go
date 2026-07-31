// Package entity holds the request/response shapes for org-scoped mail
// settings — deliberately named structs (not the global mail package's
// own types) so swag can resolve them, even though the fields mirror
// api/src/mail/accounts, api/src/mail/template, and api/src/mail/settings
// exactly. See plan/org-app-email-settings/plan.md.
package entity

// OrgMailAccountResponse is one org-scoped SMTP account. Password is
// never populated in responses — mirrors accounts.Account.Redacted().
type OrgMailAccountResponse struct {
	ID        string `json:"id" example:"b3202323-6270-448c-8001-6b803aca8053"`
	Name      string `json:"name" example:"org-transactional"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"smtp-user"`
	FromName  string `json:"from_name" example:"Acme Support"`
	FromEmail string `json:"from_email" example:"support@acme.example"`
	UseTLS    bool   `json:"use_tls" example:"true"`
	IsActive  bool   `json:"is_active" example:"true"`
	CreatedAt string `json:"created_at" example:"2026-07-30T10:00:00Z"`
	UpdatedAt string `json:"updated_at" example:"2026-07-30T10:00:00Z"`
}

// OrgMailAccountCreateRequest is the POST body for a new org account.
type OrgMailAccountCreateRequest struct {
	Name      string `json:"name" example:"org-transactional"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"smtp-user"`
	Password  string `json:"password" example:"hunter2"`
	FromName  string `json:"from_name" example:"Acme Support"`
	FromEmail string `json:"from_email" example:"support@acme.example"`
	UseTLS    bool   `json:"use_tls" example:"true"`
	IsActive  bool   `json:"is_active" example:"false"`
}

// OrgMailAccountUpdateRequest is the PATCH body. Name is immutable —
// mirrors accounts.Patch. Nil pointers mean "leave unchanged"; an
// empty (but non-nil) Password leaves the stored secret unchanged too.
type OrgMailAccountUpdateRequest struct {
	Host      *string `json:"host,omitempty"`
	Port      *int    `json:"port,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	FromName  *string `json:"from_name,omitempty"`
	FromEmail *string `json:"from_email,omitempty"`
	UseTLS    *bool   `json:"use_tls,omitempty"`
}

type OrgMailAccountListSuccess struct {
	Success bool                     `json:"success" example:"true"`
	Message []OrgMailAccountResponse `json:"message"`
}

type OrgMailAccountSuccess struct {
	Success bool                   `json:"success" example:"true"`
	Message OrgMailAccountResponse `json:"message"`
}
