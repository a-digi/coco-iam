// Package accounts manages named SMTP accounts stored in mail.db. Exactly
// one account is flagged active at any time; the mail resolver reads that
// one when constructing the SMTP config for a send.
package accounts

import "regexp"

// NameFormat mirrors the template name rule. Lowercase letters, digits,
// underscore, hyphen; must start with a letter.
var NameFormat = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Account is the full SMTP account record.
type Account struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	FromName  string `json:"from_name"`
	FromEmail string `json:"from_email"`
	UseTLS    bool   `json:"use_tls"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Patch carries the subset of fields allowed on PATCH. Name is immutable.
type Patch struct {
	Host      *string `json:"host,omitempty"`
	Port      *int    `json:"port,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	FromName  *string `json:"from_name,omitempty"`
	FromEmail *string `json:"from_email,omitempty"`
	UseTLS    *bool   `json:"use_tls,omitempty"`
}

// Redacted returns a copy with the Password cleared — used before any
// HTTP response to avoid leaking the stored secret.
func (a Account) Redacted() Account {
	a.Password = ""
	return a
}
