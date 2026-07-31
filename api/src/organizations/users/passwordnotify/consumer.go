package orgpwnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/a-digi/coco-logger/logger"
	coconotification "github.com/a-digi/coco-notification"
	"github.com/a-digi/coco-queue"
)

const QueueName = "user-password-expiry-notification"
const eventKey = "user_password_expiry_warning"

// MailConfig resolves event-to-template and event-to-account bindings.
// *notsettings.Resolver satisfies this interface.
type MailConfig interface {
	TemplateForEvent(event string) string
	AccountForEvent(event string) string
}

func Register(mgr queue.Manager, mailConfig MailConfig, mailSvc coconotification.Service, log logger.Logger) error {
	return mgr.Register(QueueName, handler(mailConfig, mailSvc, log), queue.Config{
		MaxAttempts: 3,
		Backoff:     []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute},
		Workers:     1,
	})
}

func handler(mailConfig MailConfig, mailSvc coconotification.Service, log logger.Logger) queue.Handler {
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
		_, err := mailSvc.Enqueue(coconotification.Task{
			Template: tmpl,
			Ref:      coconotification.SenderRef{Name: acct},
			To:       []coconotification.Address{{Email: p.Email, Name: p.Username}},
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
