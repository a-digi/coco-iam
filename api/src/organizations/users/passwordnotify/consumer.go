package orgpwnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-queue"
)

const QueueName = "user-password-expiry-notification"
const eventKey = "user_password_expiry_warning"

// MailConfig resolves event-to-template and event-to-account bindings.
// *mailsettings.Resolver satisfies this interface.
type MailConfig interface {
	TemplateForEvent(event string) string
	AccountForEvent(event string) string
}

func Register(mgr queue.Manager, mailConfig MailConfig, mailSvc iam_mail.MailService, log logger.Logger) error {
	return mgr.Register(QueueName, handler(mailConfig, mailSvc, log), queue.Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute},
		Workers:     1,
	})
}

func handler(mailConfig MailConfig, mailSvc iam_mail.MailService, log logger.Logger) queue.Handler {
	return func(_ context.Context, raw []byte) error {
		var p Payload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("orgpwnotify: decode payload: %w", err)
		}
		tmpl := mailConfig.TemplateForEvent(eventKey)
		acct := mailConfig.AccountForEvent(eventKey)
		if tmpl == "" {
			tmpl = eventKey
		}
		_, err := mailSvc.Enqueue(iam_mail.MailTask{
			Template: tmpl,
			Account:  acct,
			To:       []iam_mail.Address{{Email: p.Email, Name: p.Username}},
			Data: map[string]interface{}{
				"WebsiteTitle":    "coco-iam",
				"Username":        p.Username,
				"DaysUntilExpiry": p.DaysUntilExpiry,
				"ExpiryDate":      p.ExpiryDate,
			},
		})
		if err != nil {
			log.Warning("orgpwnotify: send email for user %s org %s: %v", p.UserID, p.OrgID, err)
		}
		return err
	}
}
