// Package smtp is the SMTP implementation of the mail.Mailer interface.
// It relies on github.com/wneessen/go-mail for MIME / TLS / auth.
package smtp

import (
	"context"
	"fmt"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-logger/logger"
	gomail "github.com/wneessen/go-mail"
)

// ConfigProvider returns the SMTP config to use for the next Send call.
// Allows callers (e.g. settings.Resolver) to reconfigure the mailer at
// runtime without rebuilding it.
type ConfigProvider func() Config

// Mailer is the SMTP-backed mail.Mailer.
type Mailer struct {
	cfg      Config
	provider ConfigProvider
	log      logger.Logger
}

// New constructs a Mailer with the given boot-time configuration. No
// connection is made here — DialAndSend establishes and tears down a
// connection per batch.
func New(cfg Config, log logger.Logger) *Mailer {
	return &Mailer{cfg: cfg, log: log}
}

// SetConfigProvider wires a live-reload source for the mailer's config.
// If set, Send uses the provider's return value for every dispatch;
// otherwise it uses the static cfg passed to New.
func (m *Mailer) SetConfigProvider(p ConfigProvider) {
	m.provider = p
}

// currentConfig returns the live config (from the provider if set) or the
// static boot-time value.
func (m *Mailer) currentConfig() Config {
	if m.provider != nil {
		return m.provider()
	}
	return m.cfg
}

// From returns the configured default sender address — live value when a
// provider is attached.
func (m *Mailer) From() iam_mail.Address { return m.currentConfig().From }

// Send delivers the message. The returned error is non-nil if composition or
// delivery fails; the queue consumer uses this to drive retry / DLQ.
func (m *Mailer) Send(ctx context.Context, msg iam_mail.Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/smtp: no recipients")
	}
	cfg := m.currentConfig()
	if msg.From.Email == "" {
		msg.From = cfg.From
	}

	message := gomail.NewMsg()
	if msg.From.Name != "" {
		if err := message.FromFormat(msg.From.Name, msg.From.Email); err != nil {
			return fmt.Errorf("mail/smtp: from: %w", err)
		}
	} else {
		if err := message.From(msg.From.Email); err != nil {
			return fmt.Errorf("mail/smtp: from: %w", err)
		}
	}

	for _, addr := range msg.To {
		if err := addTo(message, addr); err != nil {
			return err
		}
	}
	for _, addr := range msg.Cc {
		if err := addCc(message, addr); err != nil {
			return err
		}
	}
	for _, addr := range msg.Bcc {
		if err := addBcc(message, addr); err != nil {
			return err
		}
	}

	message.Subject(msg.Subject)
	for k, v := range msg.Headers {
		message.SetGenHeader(gomail.Header(k), v)
	}

	// Body: if both HTML and text are provided, text is the primary body and
	// HTML is the alternative (better plain-text fallback behaviour in
	// rendering clients).
	switch {
	case msg.TextBody != "" && msg.HTMLBody != "":
		message.SetBodyString(gomail.TypeTextPlain, msg.TextBody)
		message.AddAlternativeString(gomail.TypeTextHTML, msg.HTMLBody)
	case msg.HTMLBody != "":
		message.SetBodyString(gomail.TypeTextHTML, msg.HTMLBody)
	default:
		message.SetBodyString(gomail.TypeTextPlain, msg.TextBody)
	}

	options := []gomail.Option{gomail.WithPort(cfg.Port)}
	if cfg.UseTLS {
		options = append(options, gomail.WithTLSPolicy(gomail.TLSMandatory))
	} else {
		options = append(options, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	}
	if cfg.Username != "" {
		options = append(options,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(cfg.Username),
			gomail.WithPassword(cfg.Password),
		)
	}

	client, err := gomail.NewClient(cfg.Host, options...)
	if err != nil {
		return fmt.Errorf("mail/smtp: client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("mail/smtp: send: %w", err)
	}
	return nil
}

func addTo(m *gomail.Msg, a iam_mail.Address) error {
	if a.Name != "" {
		if err := m.AddToFormat(a.Name, a.Email); err != nil {
			return fmt.Errorf("mail/smtp: to %q: %w", a.Email, err)
		}
		return nil
	}
	if err := m.AddTo(a.Email); err != nil {
		return fmt.Errorf("mail/smtp: to %q: %w", a.Email, err)
	}
	return nil
}

func addCc(m *gomail.Msg, a iam_mail.Address) error {
	if a.Name != "" {
		if err := m.AddCcFormat(a.Name, a.Email); err != nil {
			return fmt.Errorf("mail/smtp: cc %q: %w", a.Email, err)
		}
		return nil
	}
	if err := m.AddCc(a.Email); err != nil {
		return fmt.Errorf("mail/smtp: cc %q: %w", a.Email, err)
	}
	return nil
}

func addBcc(m *gomail.Msg, a iam_mail.Address) error {
	if a.Name != "" {
		if err := m.AddBccFormat(a.Name, a.Email); err != nil {
			return fmt.Errorf("mail/smtp: bcc %q: %w", a.Email, err)
		}
		return nil
	}
	if err := m.AddBcc(a.Email); err != nil {
		return fmt.Errorf("mail/smtp: bcc %q: %w", a.Email, err)
	}
	return nil
}
