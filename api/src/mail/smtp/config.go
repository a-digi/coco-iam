package smtp

import (
	"os"
	"strconv"
	"strings"

	"github.com/a-digi/coco-iam/src/mail"
)

// Config holds the SMTP connection + identity settings.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	UseTLS   bool // implicit TLS (port 465). Otherwise STARTTLS is used opportunistically.
	From     mail.Address
}

// ConfigFromEnv reads SMTP_* variables with Mailpit-friendly defaults so a
// fresh checkout can send via `mailpit` on localhost:1025 with no extra
// configuration.
func ConfigFromEnv() Config {
	cfg := Config{
		Host:     getenv("SMTP_HOST", "localhost"),
		Port:     getenvInt("SMTP_PORT", 1025),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		UseTLS:   strings.EqualFold(os.Getenv("SMTP_TLS"), "true"),
		From:     parseFrom(getenv("SMTP_FROM", "coco-iam <noreply@coco-iam.local>")),
	}
	return cfg
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// parseFrom accepts either `"Display Name <addr@host>"` or a bare `addr@host`.
func parseFrom(s string) mail.Address {
	s = strings.TrimSpace(s)
	if lt := strings.LastIndex(s, "<"); lt >= 0 {
		if gt := strings.LastIndex(s, ">"); gt > lt {
			name := strings.TrimSpace(s[:lt])
			addr := strings.TrimSpace(s[lt+1 : gt])
			name = strings.Trim(name, `"`)
			return mail.Address{Name: name, Email: addr}
		}
	}
	return mail.Address{Email: s}
}
