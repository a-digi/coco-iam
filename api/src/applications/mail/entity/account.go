// Package entity holds the request/response shapes for app-scoped mail
// settings — deliberately named structs (not the global mail package's
// own types, nor the org tier's org/mail/entity types) so swag can
// resolve them, even though the fields mirror
// api/src/organizations/mail/entity exactly, one tier deeper. See
// plan/org-app-email-settings/plan.md.
package entity

// AppMailAccountResponse is one application-scoped SMTP account.
// Password is never populated in responses — mirrors
// accounts.Account.Redacted().
type AppMailAccountResponse struct {
	ID        string `json:"id" example:"b3202323-6270-448c-8001-6b803aca8053"`
	Name      string `json:"name" example:"app-transactional"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"smtp-user"`
	FromName  string `json:"from_name" example:"Acme App Support"`
	FromEmail string `json:"from_email" example:"support@acme-app.example"`
	UseTLS    bool   `json:"use_tls" example:"true"`
	IsActive  bool   `json:"is_active" example:"true"`
	CreatedAt string `json:"created_at" example:"2026-07-30T10:00:00Z"`
	UpdatedAt string `json:"updated_at" example:"2026-07-30T10:00:00Z"`
}

// AppMailAccountCreateRequest is the POST body for a new app account.
type AppMailAccountCreateRequest struct {
	Name      string `json:"name" example:"app-transactional"`
	Host      string `json:"host" example:"smtp.example.com"`
	Port      int    `json:"port" example:"587"`
	Username  string `json:"username" example:"smtp-user"`
	Password  string `json:"password" example:"hunter2"`
	FromName  string `json:"from_name" example:"Acme App Support"`
	FromEmail string `json:"from_email" example:"support@acme-app.example"`
	UseTLS    bool   `json:"use_tls" example:"true"`
	IsActive  bool   `json:"is_active" example:"false"`
}

// AppMailAccountUpdateRequest is the PATCH body. Name is immutable —
// mirrors accounts.Patch. Nil pointers mean "leave unchanged"; an
// empty (but non-nil) Password leaves the stored secret unchanged too.
type AppMailAccountUpdateRequest struct {
	Host      *string `json:"host,omitempty"`
	Port      *int    `json:"port,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	FromName  *string `json:"from_name,omitempty"`
	FromEmail *string `json:"from_email,omitempty"`
	UseTLS    *bool   `json:"use_tls,omitempty"`
}

type AppMailAccountListSuccess struct {
	Success bool                     `json:"success" example:"true"`
	Message []AppMailAccountResponse `json:"message"`
}

type AppMailAccountSuccess struct {
	Success bool                   `json:"success" example:"true"`
	Message AppMailAccountResponse `json:"message"`
}
