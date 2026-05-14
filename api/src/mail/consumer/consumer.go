// Package consumer wires the mail-outbound queue to a concrete Mailer and
// the mail.db store. It is the only place in the codebase that knows about
// both the queue transport and the Mailer interface — application code goes
// through mail.MailService and never touches either directly.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/accounts"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	"github.com/a-digi/coco-iam/src/mail/store"
	"github.com/a-digi/coco-queue"
	"github.com/a-digi/coco-logger/logger"
)

// Config lets main.go tune the mail-outbound queue's retry policy + the
// initial worker-pool size. The orchestrator goroutine in orchestrator.go
// resizes the pool at runtime based on backlog.
type Config struct {
	MaxAttempts    int
	InitialWorkers int
}

// Register constructs a handler bound to the supplied Mailer + Store and
// registers it for the outbound mail queue. Must be called before
// queue.Start.
//
// accountsStore is optional: when non-nil, tasks carrying a non-empty
// Account field are dispatched via a one-off SMTPMailer built from that
// account's config. Tasks without an Account fall through to the default
// mailer (which uses the globally-active account's config via the
// resolver).
func Register(
	mgr queue.Manager,
	st *store.Store,
	mailer iam_mail.Mailer,
	accountsStore *accounts.Store,
	cfg Config,
	log logger.Logger,
) error {
	qcfg := queue.Config{
		MaxAttempts: cfg.MaxAttempts,
		Backoff: []time.Duration{
			5 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			10 * time.Minute,
			30 * time.Minute,
		},
		Workers: cfg.InitialWorkers,
	}
	return mgr.Register(iam_mail.QueueNameOutbound, handler(st, mailer, accountsStore, cfg.MaxAttempts, log), qcfg)
}

func handler(
	st *store.Store,
	mailer iam_mail.Mailer,
	accountsStore *accounts.Store,
	maxAttempts int,
	log logger.Logger,
) queue.Handler {
	return func(ctx context.Context, payload []byte) error {
		var task iam_mail.MailTask
		if err := json.Unmarshal(payload, &task); err != nil {
			return fmt.Errorf("mail consumer: decode payload: %w", err)
		}

		// Pick the mailer for this specific task — per-task account
		// override wins over the default.
		effectiveMailer, senderr := selectMailer(task, mailer, accountsStore, log)
		if senderr != nil {
			// A bad account reference is a hard failure — queue it as an
			// error so retries + DLQ apply.
			if task.MailID != "" {
				_ = st.MarkSending(task.MailID)
				row, _ := st.Get(task.MailID)
				willRetry := true
				if row != nil && row.Attempts >= row.MaxAttempts {
					willRetry = false
				}
				next := time.Now().Add(30 * time.Second)
				_ = st.MarkFailed(task.MailID, senderr.Error(), willRetry, next)
			}
			return senderr
		}

		if task.MailID == "" {
			return doSend(ctx, task, effectiveMailer, log)
		}

		if err := st.MarkSending(task.MailID); err != nil {
			log.Warning("mail consumer: mark_sending %s failed: %v", task.MailID, err)
		}

		sendErr := doSend(ctx, task, effectiveMailer, log)
		if sendErr == nil {
			if err := st.MarkSent(task.MailID); err != nil {
				log.Warning("mail consumer: mark_sent %s failed: %v", task.MailID, err)
			}
			return nil
		}

		row, rerr := st.Get(task.MailID)
		willRetry := true
		if rerr == nil && row != nil && row.Attempts >= row.MaxAttempts {
			willRetry = false
		} else if maxAttempts > 0 && rerr == nil && row != nil && row.Attempts >= maxAttempts {
			willRetry = false
		}
		next := time.Now().Add(30 * time.Second)
		if merr := st.MarkFailed(task.MailID, sendErr.Error(), willRetry, next); merr != nil {
			log.Warning("mail consumer: mark_failed %s failed: %v", task.MailID, merr)
		}
		return sendErr
	}
}

// selectMailer returns the concrete Mailer to use for this task. When the
// task carries a non-empty Account, we build a one-off SMTPMailer bound
// to that account's stored credentials — so event-driven sends use the
// bound account regardless of which one is globally active.
func selectMailer(
	task iam_mail.MailTask,
	defaultMailer iam_mail.Mailer,
	accountsStore *accounts.Store,
	log logger.Logger,
) (iam_mail.Mailer, error) {
	if task.Account == "" || accountsStore == nil {
		return defaultMailer, nil
	}
	acc, err := accountsStore.GetByName(task.Account)
	if err != nil {
		return nil, fmt.Errorf("mail consumer: account %q lookup failed: %w", task.Account, err)
	}
	cfg := mailsmtp.Config{
		Host:     acc.Host,
		Port:     acc.Port,
		Username: acc.Username,
		Password: acc.Password,
		UseTLS:   acc.UseTLS,
		From:     iam_mail.Address{Name: acc.FromName, Email: acc.FromEmail},
	}
	return mailsmtp.New(cfg, log), nil
}

func doSend(ctx context.Context, task iam_mail.MailTask, mailer iam_mail.Mailer, log logger.Logger) error {
	msg := iam_mail.Message{
		From:     task.From,
		To:       task.To,
		Cc:       task.Cc,
		Bcc:      task.Bcc,
		Subject:  task.Subject,
		TextBody: task.TextBody,
		HTMLBody: task.HTMLBody,
	}
	if err := mailer.Send(ctx, msg); err != nil {
		log.Warning("mail consumer: send to %v failed: %v", recipientsFor(msg), err)
		return err
	}
	log.Info("mail consumer: sent %q to %v", msg.Subject, recipientsFor(msg))
	return nil
}

func recipientsFor(msg iam_mail.Message) []string {
	out := make([]string, 0, len(msg.To))
	for _, addr := range msg.To {
		out = append(out, addr.Email)
	}
	return out
}
