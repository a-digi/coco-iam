package recovery

import (
	"strconv"
	"strings"
	"time"

	notsettings "github.com/a-digi/coco-notification/settings"
)

// Recovery-scoped keys in mail_settings. Shorter defaults than
// activation — a recovery link that outlives the user's memory of
// requesting it is a security risk.
// BaseURL and PageTitle now come from the per-org general.Store resolved
// at call time; only the flow-cadence knobs (TTL + cooldown) still live here.
const (
	KeyTTLHours              = "recovery.ttl_hours"
	KeyResendCooldownSeconds = "recovery.resend_cooldown_seconds"
)

const (
	defaultTTLHours          = 1
	defaultResendCooldownSec = 300
)

// SettingsReader looks up recovery-scoped knobs from mail_settings.
// BaseURL and PageTitle are resolved per-org at call time by the Service.
type SettingsReader struct {
	store *notsettings.Store
}

// NewSettingsReader binds a reader to the notification settings store.
func NewSettingsReader(store *notsettings.Store) *SettingsReader {
	return &SettingsReader{store: store}
}

// TTL returns the recovery-link time-to-live.
func (r *SettingsReader) TTL() time.Duration {
	hours := defaultTTLHours
	if v, ok, _ := r.store.Get(KeyTTLHours); ok && v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			hours = n
		}
	}
	return time.Duration(hours) * time.Hour
}

// ResendCooldown returns the minimum interval between two recovery
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
// template token (e.g. "1 hour").
func (r *SettingsReader) TTLHumanReadable() string {
	d := r.TTL()
	hours := int(d.Hours())
	if hours == 1 {
		return "1 hour"
	}
	return strconv.Itoa(hours) + " hours"
}
