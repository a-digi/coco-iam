package entity

// OrgEventBinding pairs a system mail event with the org-scoped
// template/account names bound to it — mirrors
// api/src/mail/settings.EventBinding. Empty Template/Account means this
// org has not customized that event; it falls back to the org own
// active account and then the global binding.
type OrgEventBinding struct {
	Event    string `json:"event" example:"user_invite"`
	Template string `json:"template" example:""`
	Account  string `json:"account" example:""`
}

// OrgActivationSettings carries the org overrides for the activation
// email cadence. Nil means "not customized here" — falls back to the
// global mail_settings value.
type OrgActivationSettings struct {
	TTLHours              *int `json:"ttl_hours,omitempty" example:"24"`
	ResendCooldownSeconds *int `json:"resend_cooldown_seconds,omitempty" example:"300"`
}

// OrgMailSettingsResponse reflects exactly what THIS org has stored —
// not a resolved cascade view. ActiveAccount is nil when the org has no
// active account of its own (falls back to global at send time).
type OrgMailSettingsResponse struct {
	ActiveAccount *OrgMailAccountResponse `json:"active_account"`
	Events        []OrgEventBinding       `json:"events"`
	Activation    OrgActivationSettings   `json:"activation"`
}

// OrgMailSettingsUpdateRequest is the PATCH body. Omitted fields are
// left unchanged; an included Events entry with both Template and
// Account empty clears that event's org-level override.
type OrgMailSettingsUpdateRequest struct {
	Events     []OrgEventBinding      `json:"events,omitempty"`
	Activation *OrgActivationSettings `json:"activation,omitempty"`
}

type OrgMailSettingsSuccess struct {
	Success bool                    `json:"success" example:"true"`
	Message OrgMailSettingsResponse `json:"message"`
}
