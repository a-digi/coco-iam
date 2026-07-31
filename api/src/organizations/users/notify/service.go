package notify

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/a-digi/coco-iam/src/general"
	iam_notification "github.com/a-digi/coco-iam/src/notification"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-logger/logger"
	coconotification "github.com/a-digi/coco-notification"
)

// Service sends lifecycle notification emails to organisation users.
// All methods are non-blocking: errors are logged and never propagated.
type Service struct {
	db          *sql.DB
	orgRegistry *dbregistry.OrgUserDBRegistry
	mailConfig  *iam_notification.ScopedResolver
	mail        coconotification.Service
	log         logger.Logger
}

// NewService constructs the notify service.
// db is the main database used for the global PageTitle fallback.
func NewService(
	db *sql.DB,
	orgRegistry *dbregistry.OrgUserDBRegistry,
	mailConfig *iam_notification.ScopedResolver,
	mail coconotification.Service,
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
	tmpl := s.mailConfig.TemplateForEvent(orgID, "", event)
	account, resolvedOrgID, _ := s.mailConfig.AccountForEvent(orgID, "", event)
	websiteTitle := s.pageTitle(orgID)

	task := coconotification.Task{
		Ref: coconotification.SenderRef{Name: account, Scope: iam_notification.Scope(resolvedOrgID, "")},
		To:  []coconotification.Address{{Email: email, Name: username}},
	}

	if tmpl != "" {
		data := map[string]interface{}{
			"Username":     username,
			"WebsiteTitle": websiteTitle,
		}
		// Prefer this org's own active template of the same name over
		// the global renderer — falls through untouched (task.Template
		// set, exactly as before) when the org has none of its own.
		if subject, text, html, ok, err := s.mailConfig.RenderTemplate(orgID, "", tmpl, data); err == nil && ok {
			task.Subject, task.TextBody, task.HTMLBody = subject, text, html
		} else {
			if err != nil {
				s.log.Warning("org user notify: org template render for %q failed, falling back to global: %v", tmpl, err)
			}
			task.Template = tmpl
			task.Data = data
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
