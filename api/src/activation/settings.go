package activation

import (
	"strconv"
	"strings"
	"time"

	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
)

// Activation-scoped keys in mail_settings. BaseURL and PageTitle now
// come from the per-org general.Store resolved at call time; only the
// flow-cadence knobs (TTL + resend cooldown) still live here.
const (
	KeyTTLHours              = "activation.ttl_hours"
	KeyResendCooldownSeconds = "activation.resend_cooldown_seconds"
)

const (
	defaultTTLHours          = 24
	defaultResendCooldownSec = 300
)

// SettingsReader looks up activation-scoped knobs from mail_settings.
// BaseURL and PageTitle are resolved per-org at call time by the Service.
type SettingsReader struct {
	store *mailsettings.Store
}

// NewSettingsReader binds a reader to the mail settings store.
func NewSettingsReader(store *mailsettings.Store) *SettingsReader {
	return &SettingsReader{store: store}
}

// TTL returns the activation-link time-to-live.
func (r *SettingsReader) TTL() time.Duration {
	hours := defaultTTLHours
	if v, ok, _ := r.store.Get(KeyTTLHours); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			hours = n
		}
	}
	return time.Duration(hours) * time.Hour
}

// ResendCooldown returns the minimum interval between two resend
// requests for the same user.
func (r *SettingsReader) ResendCooldown() time.Duration {
	seconds := defaultResendCooldownSec
	if v, ok, _ := r.store.Get(KeyResendCooldownSeconds); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
			seconds = n
		}
	}
	return time.Duration(seconds) * time.Second
}

// TTLHumanReadable is the string substituted into the `{{ .ExpiresIn }}`
// token in email templates (e.g. "24 hours").
func (r *SettingsReader) TTLHumanReadable() string {
	d := r.TTL()
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour"
	}
	return strconv.Itoa(hours) + " hours"
}
