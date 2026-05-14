package template

import "regexp"

// NameFormat matches the backend + frontend rule. Lowercase letters,
// digits, underscore, and hyphen; must start with a letter.
var NameFormat = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// Template is the DB-backed representation of an email template. Name is
// the public identifier used by MailService when calling Render.
type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subject     string `json:"subject"`
	TextBody    string `json:"text_body"`
	HTMLBody    string `json:"html_body"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Patch carries the subset of fields PATCH requests may change. Name is
// deliberately excluded — renderers address templates by name and a
// rename would silently break every caller.
type Patch struct {
	Description *string `json:"description,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	TextBody    *string `json:"text_body,omitempty"`
	HTMLBody    *string `json:"html_body,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// ListFilter narrows the list endpoint. Empty fields mean "don't filter".
type ListFilter struct {
	NameLike        string
	DescriptionLike string
	Limit           int
	Offset          int
}
