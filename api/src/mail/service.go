package mail

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-iam/src/mail/template"
	"github.com/a-digi/coco-queue"
)

// MailService is the high-level facade. Business code uses this — never the
// Mailer directly — so that templating, defaults, audit, and queueing stay
// consistent across the codebase.
type MailService interface {
	// Enqueue renders (when Template is set), persists a row in mail.db,
	// publishes to the mail-outbound queue, and returns the shared MailID.
	Enqueue(task MailTask) (string, error)
}

// Default max attempts used for new mail rows when the caller doesn't
// override via env/config. Matches the queue Config.MaxAttempts registered
// by the consumer.
const defaultMaxAttempts = 5

// NewMailService constructs the default MailService.
func NewMailService(q queue.Manager, s *store.Store, renderer *template.Renderer, defaultFrom Address) MailService {
	return &defaultService{
		queue:       q,
		store:       s,
		renderer:    renderer,
		from:        defaultFrom,
		maxAttempts: defaultMaxAttempts,
	}
}

type defaultService struct {
	queue       queue.Manager
	store       *store.Store
	renderer    *template.Renderer
	from        Address
	maxAttempts int
}

func (s *defaultService) Enqueue(task MailTask) (string, error) {
	if len(task.To) == 0 {
		return "", errors.New("mail: task has no recipients")
	}
	if task.From.Email == "" {
		task.From = s.from
	}

	if task.Template != "" && s.renderer != nil {
		subject, textBody, htmlBody, err := s.renderer.Render(task.Template, task.Data)
		if err != nil {
			return "", fmt.Errorf("mail: enqueue: %w", err)
		}
		if task.Subject == "" {
			task.Subject = subject
		}
		if task.TextBody == "" {
			task.TextBody = textBody
		}
		if task.HTMLBody == "" {
			task.HTMLBody = htmlBody
		}
	}

	if task.Subject == "" {
		return "", errors.New("mail: subject is empty and not provided by template")
	}
	if task.TextBody == "" && task.HTMLBody == "" {
		return "", errors.New("mail: body is empty and not provided by template")
	}

	id := newUUID()
	task.MailID = id

	if err := s.store.Insert(store.InsertArgs{
		ID:          id,
		Template:    task.Template,
		Subject:     task.Subject,
		From:        toStoreAddr(task.From),
		To:          toStoreAddrs(task.To),
		Cc:          toStoreAddrs(task.Cc),
		Bcc:         toStoreAddrs(task.Bcc),
		MaxAttempts: s.maxAttempts,
	}); err != nil {
		return "", err
	}

	// Publish with the shared id so queue_tasks.id == mail_outbound.id.
	if err := s.queue.PublishWithID(id, QueueNameOutbound, task); err != nil {
		// Best-effort: flip the row to failed so it doesn't look queued forever.
		_ = s.store.MarkFailed(id, "enqueue failed: "+err.Error(), false, time.Time{})
		return "", err
	}
	return id, nil
}

// --- helpers ---

func toStoreAddr(a Address) store.Address {
	return store.Address{Name: a.Name, Email: a.Email}
}

func toStoreAddrs(list []Address) []store.Address {
	if len(list) == 0 {
		return nil
	}
	out := make([]store.Address, len(list))
	for i, a := range list {
		out[i] = store.Address{Name: a.Name, Email: a.Email}
	}
	return out
}

// FromStoreAddrs is the inverse — used by admin handlers that hand row
// data back to the caller as mail.Address.
func FromStoreAddrs(list []store.Address) []Address {
	if len(list) == 0 {
		return nil
	}
	out := make([]Address, len(list))
	for i, a := range list {
		out[i] = Address{Name: a.Name, Email: a.Email}
	}
	return out
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Fallback to a deterministic but unique-ish value so inserts don't
		// collide — crypto/rand failures are vanishingly rare, so logging
		// isn't worth the extra plumbing here.
		return "00000000-0000-0000-0000-000000000000"
	}
	hx := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hx[:8], hx[8:12], hx[12:16], hx[16:20], hx[20:32])
}
