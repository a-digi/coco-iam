package entity

type Application struct {
	_                 struct{} `table:"applications"`
	ID                string   `db:"id" dbtype:"UUID" nullable:"false" json:"id"`
	WorkspaceID       string   `db:"workspace_id" dbtype:"TEXT" nullable:"false" json:"workspace_id"`
	ClientID          string   `db:"client_id" dbtype:"TEXT" nullable:"false" json:"client_id"`
	Title             string   `db:"title" dbtype:"TEXT" nullable:"false" json:"title"`
	Description       string   `db:"description" dbtype:"TEXT" nullable:"false" default:"" json:"description"`
	CreatedAt         string   `db:"created_at" dbtype:"DATETIME" nullable:"true" json:"created_at"`
	IsActive          bool     `db:"is_active" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"is_active"`
	AllowRecovery      bool `db:"allow_recovery" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"allow_recovery"`
	AllowRegistration  bool `db:"allow_registration" dbtype:"BOOLEAN" nullable:"false" default:"false" json:"allow_registration"`
	// AllowPasswordLogin toggles the username+password login endpoint
	// for this app. Default true preserves legacy behaviour; an admin
	// flips it off to force the OAuth-only path. Both the login
	// handler and the public auth-methods endpoint honour this flag.
	AllowPasswordLogin bool `db:"allow_password_login" dbtype:"BOOLEAN" nullable:"false" default:"true" json:"allow_password_login"`
	// RegistrationType declares how registration is collected:
	//   - "legacy": consumers render a form with username + password
	//     implicit and the admin-configured extras on top.
	//   - "oauth":  provider delivers identity; extras only.
	// Only "legacy" is wired today; "oauth" is a declarative label
	// we surface so the consumer can branch its UI ahead of the
	// provider-flow implementation landing.
	RegistrationType string `db:"registration_type" dbtype:"TEXT" nullable:"false" default:"legacy" json:"registration_type"`
}
