package settings

import (
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/mail/accounts"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	"github.com/a-digi/coco-logger/logger"
)

// MigrateLegacySMTPIfNeeded runs once per boot. If no accounts exist yet
// but the old `smtp.*` KV rows are present (from the pre-accounts version
// of this project), it promotes them into a seed account called
// `default` (marked active) and then deletes the legacy keys. Safe to
// call repeatedly — subsequent calls short-circuit as soon as any
// account exists.
func MigrateLegacySMTPIfNeeded(
	settingsStore *Store,
	accountsStore *accounts.Store,
	envCfg mailsmtp.Config,
	log logger.Logger,
) error {
	existing, err := accountsStore.Count()
	if err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	host, hostOK, _ := settingsStore.Get(KeySMTPHost)
	portStr, portOK, _ := settingsStore.Get(KeySMTPPort)
	username, _, _ := settingsStore.Get(KeySMTPUsername)
	password, _, _ := settingsStore.Get(KeySMTPPassword)
	fromName, _, _ := settingsStore.Get(KeySMTPFromName)
	fromEmail, fromEmailOK, _ := settingsStore.Get(KeySMTPFromEmail)
	tlsStr, _, _ := settingsStore.Get(KeySMTPTLS)

	anyLegacy := hostOK || portOK || fromEmailOK
	if !anyLegacy {
		// No legacy rows. Env fallback will cover sends; no seed account needed.
		return nil
	}

	seed := accounts.Account{
		Name:      "default",
		Host:      firstNonEmpty(host, envCfg.Host),
		Port:      parseIntWithFallback(portStr, envCfg.Port),
		Username:  firstNonEmpty(username, envCfg.Username),
		Password:  firstNonEmpty(password, envCfg.Password),
		FromName:  firstNonEmpty(fromName, envCfg.From.Name),
		FromEmail: firstNonEmpty(fromEmail, envCfg.From.Email),
		UseTLS:    parseBoolWithFallback(tlsStr, envCfg.UseTLS),
		IsActive:  true,
	}
	if _, err := accountsStore.Create(seed); err != nil {
		return err
	}
	log.Info("mail migration: seeded 'default' SMTP account from legacy settings keys")

	// Clean up the legacy keys so the UI never shows them again.
	for _, k := range []string{
		KeySMTPHost, KeySMTPPort, KeySMTPUsername, KeySMTPPassword,
		KeySMTPFromName, KeySMTPFromEmail, KeySMTPTLS,
	} {
		if derr := settingsStore.Delete(k); derr != nil {
			log.Warning("mail migration: delete legacy key %q failed: %v", k, derr)
		}
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func parseIntWithFallback(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

func parseBoolWithFallback(s string, fallback bool) bool {
	if s == "" {
		return fallback
	}
	return strings.EqualFold(s, "true") || s == "1"
}
