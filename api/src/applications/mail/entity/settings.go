package entity

// AppEventBinding pairs a system mail event with the application-scoped
// template/account names bound to it — mirrors
// api/src/mail/settings.EventBinding. Empty Template/Account means this
// application has not customized that event; it falls back to the
// app's own active account, then the organization's binding, then the
// global binding.
type AppEventBinding struct {
	Event    string `json:"event" example:"user_invite"`
	Template string `json:"template" example:""`
	Account  string `json:"account" example:""`
}

// AppActivationSettings carries the application overrides for the
// activation email cadence. Nil means "not customized here" — falls
// back to the organization's override, then the global mail_settings
// value.
type AppActivationSettings struct {
	TTLHours              *int `json:"ttl_hours,omitempty" example:"24"`
	ResendCooldownSeconds *int `json:"resend_cooldown_seconds,omitempty" example:"300"`
}

// AppMailSettingsResponse reflects exactly what THIS application has
// stored — not a resolved cascade view. ActiveAccount is nil when the
// application has no active account of its own (falls back to the
// org's, then the global, active account at send time).
type AppMailSettingsResponse struct {
	ActiveAccount *AppMailAccountResponse `json:"active_account"`
	Events        []AppEventBinding       `json:"events"`
	Activation    AppActivationSettings   `json:"activation"`
}

// AppMailSettingsUpdateRequest is the PATCH body. Omitted fields are
// left unchanged; an included Events entry with both Template and
// Account empty clears that event's application-level override.
type AppMailSettingsUpdateRequest struct {
	Events     []AppEventBinding      `json:"events,omitempty"`
	Activation *AppActivationSettings `json:"activation,omitempty"`
}

type AppMailSettingsSuccess struct {
	Success bool                    `json:"success" example:"true"`
	Message AppMailSettingsResponse `json:"message"`
}
