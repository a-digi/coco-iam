// Package mail defines the provider-agnostic mail abstraction. Concrete
// implementations (SMTP for now) live in sub-packages so that swapping a
// provider is a one-file change.
package mail

// Address pairs a display name with an email. Name is optional.
type Address struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// Attachment is a single file attached to an outgoing message.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Data        []byte `json:"data"`
}

// Message is the fully-resolved representation of an outgoing email.
// Templates have already been rendered by the time the mailer receives it.
type Message struct {
	From        Address           `json:"from"`
	To          []Address         `json:"to"`
	Cc          []Address         `json:"cc,omitempty"`
	Bcc         []Address         `json:"bcc,omitempty"`
	Subject     string            `json:"subject"`
	TextBody    string            `json:"text_body,omitempty"`
	HTMLBody    string            `json:"html_body,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// MailTask is the JSON payload carried on the mail-outbound queue. The
// consumer either uses the rendered Subject/TextBody/HTMLBody directly or —
// if Template is set — renders the template with Data first.
//
// MailID, when present, is the shared UUID between queue_tasks.id and
// mail_outbound.id so operators can cross-reference. MailService fills it.
//
// Account, when non-empty, overrides the globally-active SMTP account for
// this specific send. Used by event-driven callers that look up their
// account via settings.Resolver.AccountForEvent.
//
// OrgID, when non-empty, tells the consumer that Account names an
// ORGANIZATION-scoped account (looked up in that org's own users.db),
// not the global mail_smtp_accounts table — set alongside Account by
// callers using scopedsettings.ScopedResolver.AccountForEvent, which
// reports which tier actually satisfied the binding. Never carries raw
// credentials itself — only an id, resolved fresh from a database at
// consume time, same as the existing Account-by-name mechanism.
type MailTask struct {
	MailID   string                 `json:"mail_id,omitempty"`
	Template string                 `json:"template,omitempty"`
	Subject  string                 `json:"subject,omitempty"`
	From     Address                `json:"from,omitempty"`
	To       []Address              `json:"to"`
	Cc       []Address              `json:"cc,omitempty"`
	Bcc      []Address              `json:"bcc,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	TextBody string                 `json:"text_body,omitempty"`
	HTMLBody string                 `json:"html_body,omitempty"`
	Account  string                 `json:"account,omitempty"`
	OrgID    string                 `json:"org_id,omitempty"`
}
