package notify

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/general"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
)

// Service sends lifecycle notification emails to organisation users.
// All methods are non-blocking: errors are logged and never propagated.
type Service struct {
	db          *sql.DB
	orgRegistry *dbregistry.OrgUserDBRegistry
	mailConfig  *mailsettings.Resolver
	mail        iam_mail.MailService
	log         logger.Logger
}

// NewService constructs the notify service.
// db is the main database used for the global PageTitle fallback.
func NewService(
	db *sql.DB,
	orgRegistry *dbregistry.OrgUserDBRegistry,
	mailConfig *mailsettings.Resolver,
	mail iam_mail.MailService,
	log logger.Logger,
) *Service {
	return &Service{
		db:          db,
		orgRegistry: orgRegistry,
		mailConfig:  mailConfig,
		mail:        mail,
		log:         log,
	}
}

// OnDeactivated fires after is_active transitions true → false.
// Errors are logged; never propagated to the caller.
func (s *Service) OnDeactivated(_ context.Context, _, username, email, orgID string) {
	s.send(EventOrgUserDeactivated, username, email, orgID)
}

// OnRemoved fires after a successful DELETE.
// Errors are logged; never propagated to the caller.
func (s *Service) OnRemoved(_ context.Context, _, username, email, orgID string) {
	s.send(EventOrgUserRemoved, username, email, orgID)
}

func (s *Service) send(event, username, email, orgID string) {
	tmpl := s.mailConfig.TemplateForEvent(event)
	account := s.mailConfig.AccountForEvent(event)
	websiteTitle := s.pageTitle(orgID)

	task := iam_mail.MailTask{
		Account: account,
		To:      []iam_mail.Address{{Email: email, Name: username}},
	}

	if tmpl != "" {
		task.Template = tmpl
		task.Data = map[string]interface{}{
			"Username":     username,
			"WebsiteTitle": websiteTitle,
		}
	} else {
		switch event {
		case EventOrgUserDeactivated:
			task.Subject = "Your account has been deactivated"
			task.TextBody = fmt.Sprintf(
				"Hello %s, your account on %s has been deactivated. If you believe this was a mistake, please contact your administrator.",
				username, websiteTitle,
			)
		case EventOrgUserRemoved:
			task.Subject = "Your account has been removed"
			task.TextBody = fmt.Sprintf(
				"Hello %s, your account on %s has been permanently removed. If you believe this was a mistake, please contact your administrator.",
				username, websiteTitle,
			)
		}
	}

	if _, err := s.mail.Enqueue(task); err != nil {
		s.log.Error("org user notify: enqueue %s for %s: %v", event, email, err)
	}
}

// pageTitle returns the per-org page title or falls back to the global one.
func (s *Service) pageTitle(orgID string) string {
	if orgID != "" && s.orgRegistry != nil {
		if orgDB, err := orgrouter.ForOrg(s.orgRegistry, orgID); err == nil {
			if title := general.NewStoreFromDB(orgDB).PageTitle(); title != "" {
				return title
			}
		}
	}
	if s.db != nil {
		if title := general.NewStoreFromDB(s.db).PageTitle(); title != "" {
			return title
		}
	}
	return ""
}
