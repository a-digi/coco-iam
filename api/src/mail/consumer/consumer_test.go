package consumer

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	iam_mail "github.com/a-digi/coco-iam/src/mail"
	"github.com/a-digi/coco-iam/src/mail/accounts"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	orgmail_persistent "github.com/a-digi/coco-iam/src/organizations/mail/repository/persistent"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

// orgUserMigrationsPath returns the absolute path to the per-org user
// migration files regardless of where `go test` is invoked from —
// mirrors mail/scopedsettings/resolver_test.go's own helper.
func orgUserMigrationsPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "../../../config/db/org_user_migrations")
}

func newTestOrgRegistry(t *testing.T) *dbregistry.OrgUserDBRegistry {
	t.Helper()
	return dbregistry.New(t.TempDir(), orgUserMigrationsPath())
}

func newTestGlobalAccountsStore(t *testing.T) *accounts.Store {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open mail db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE mail_smtp_accounts (
		    id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, host TEXT NOT NULL, port INTEGER NOT NULL DEFAULT 587,
		    username TEXT NOT NULL DEFAULT '', password TEXT NOT NULL DEFAULT '', from_name TEXT NOT NULL DEFAULT '',
		    from_email TEXT NOT NULL DEFAULT '', use_tls BOOLEAN NOT NULL DEFAULT FALSE, is_active BOOLEAN NOT NULL DEFAULT FALSE,
		    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("create mail_smtp_accounts: %v", err)
	}
	return accounts.NewStore(&orm.DatabaseManager{Connector: &orm.Connector{DB: db}})
}

// recordingMailer is a no-op iam_mail.Mailer test double — selectMailer
// only needs to return it unchanged when a task has no Account.
type recordingMailer struct{}

func (r *recordingMailer) Send(_ context.Context, _ iam_mail.Message) error { return nil }

// fromOf type-asserts to the concrete *mailsmtp.Mailer selectMailer's
// account-resolution branches actually construct, so the test can read
// back which config was selected via its exported From() accessor —
// the iam_mail.Mailer interface itself only exposes Send.
func fromOf(t *testing.T, m iam_mail.Mailer) iam_mail.Address {
	t.Helper()
	concrete, ok := m.(*mailsmtp.Mailer)
	if !ok {
		t.Fatalf("selectMailer() returned %T, want *mailsmtp.Mailer", m)
	}
	return concrete.From()
}

func TestSelectMailer_NoAccount_ReturnsDefaultMailer(t *testing.T) {
	defaultMailer := &recordingMailer{}
	got, err := selectMailer(iam_mail.MailTask{}, defaultMailer, nil, nil, nil)
	if err != nil {
		t.Fatalf("selectMailer() error = %v", err)
	}
	if got != iam_mail.Mailer(defaultMailer) {
		t.Fatal("selectMailer() with no Account must return the default mailer unchanged")
	}
}

func TestSelectMailer_GlobalAccount_NoOrgID_UsesGlobalAccountsStore(t *testing.T) {
	accountsStore := newTestGlobalAccountsStore(t)
	if _, err := accountsStore.Create(accounts.Account{
		Name: "global-acc", Host: "global.example", Port: 587, FromEmail: "global@example.com",
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	got, err := selectMailer(iam_mail.MailTask{Account: "global-acc"}, &recordingMailer{}, accountsStore, nil, nil)
	if err != nil {
		t.Fatalf("selectMailer() error = %v", err)
	}
	if from := fromOf(t, got); from.Email != "global@example.com" {
		t.Fatalf("From().Email = %q, want the global account's email", from.Email)
	}
}

func TestSelectMailer_OrgAccount_UsesOrgOwnAccountsTable_NotGlobal(t *testing.T) {
	reg := newTestOrgRegistry(t)
	accountsStore := newTestGlobalAccountsStore(t)

	// A GLOBAL account with the SAME NAME as the org account below —
	// proves the two namespaces never cross-resolve.
	if _, err := accountsStore.Create(accounts.Account{
		Name: "shared-name", Host: "global.example", Port: 587, FromEmail: "global@example.com",
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	if _, err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailAccountWrite{
		Name: "shared-name", Host: "org.example", Port: 587, FromEmail: "org@example.com",
	}); err != nil {
		t.Fatalf("create org account: %v", err)
	}

	got, err := selectMailer(iam_mail.MailTask{Account: "shared-name", OrgID: "org-1"}, &recordingMailer{}, accountsStore, reg, nil)
	if err != nil {
		t.Fatalf("selectMailer() error = %v", err)
	}
	if from := fromOf(t, got); from.Email != "org@example.com" {
		t.Fatalf("From().Email = %q, want the ORG account's email (%q) — must never resolve to the same-named global account", from.Email, "org@example.com")
	}
}

func TestSelectMailer_OrgAccount_MissingOrgRegistry_Errors(t *testing.T) {
	_, err := selectMailer(iam_mail.MailTask{Account: "acc", OrgID: "org-1"}, &recordingMailer{}, nil, nil, nil)
	if err == nil {
		t.Fatal("selectMailer() error = nil, want an error — a task references an org account but no registry is configured")
	}
}

func TestSelectMailer_OrgAccount_NotFoundInOrg_ErrorsWithoutFallingBackToGlobal(t *testing.T) {
	reg := newTestOrgRegistry(t)
	accountsStore := newTestGlobalAccountsStore(t)
	// A global account with this exact name exists — must NOT be used
	// as a fallback just because the org's own lookup failed.
	if _, err := accountsStore.Create(accounts.Account{
		Name: "acc", Host: "global.example", Port: 587, FromEmail: "global@example.com",
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	_, err := selectMailer(iam_mail.MailTask{Account: "acc", OrgID: "org-1"}, &recordingMailer{}, accountsStore, reg, nil)
	if err == nil {
		t.Fatal("selectMailer() error = nil, want an error — the org has no account named this, and it must not silently fall back to the global one")
	}
}
