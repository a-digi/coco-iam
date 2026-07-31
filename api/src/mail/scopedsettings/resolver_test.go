package scopedsettings

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	appmail_persistent "github.com/a-digi/coco-iam/src/applications/mail/repository/persistent"
	appmail_query "github.com/a-digi/coco-iam/src/applications/mail/repository/query"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	mailstore "github.com/a-digi/coco-iam/src/mail/store"
	orgmail_persistent "github.com/a-digi/coco-iam/src/organizations/mail/repository/persistent"
	orgmail_query "github.com/a-digi/coco-iam/src/organizations/mail/repository/query"
	"github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-orm/orm"
	_ "github.com/mattn/go-sqlite3"
)

// createTestApp inserts a minimal applications row into orgDB so
// orgrouter.OrgDBForApp can find it — mirrors the real
// api/config/db/org_user_migrations/11_applications.sql shape,
// populating only the NOT NULL columns a test needs.
func createTestApp(t *testing.T, orgDB *sql.DB, appID string) {
	t.Helper()
	_, err := orgDB.Exec(
		`INSERT INTO applications (id, workspace_id, client_id, title) VALUES (?, ?, ?, ?)`,
		appID, "ws-1", appID+"-client", appID,
	)
	if err != nil {
		t.Fatalf("create test app %s: %v", appID, err)
	}
}

// orgUserMigrationsPath returns the absolute path to the per-org user
// migration files regardless of where `go test` is invoked from —
// mirrors organizations/users/admin/insert_user_integration_test.go.
func orgUserMigrationsPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "../../../config/db/org_user_migrations")
}

// newTestGlobalResolver builds a real global mailsettings.Resolver
// against an in-memory mail.db — same schema install function main.go
// uses, so this test would catch a real schema drift.
func newTestGlobalResolver(t *testing.T, envCfg mailsmtp.Config) (*mailsettings.Resolver, *mailsettings.Store, *mailaccounts.Store) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open mail db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	dbm := &orm.DatabaseManager{Connector: &orm.Connector{DB: db}}
	if err := mailstore.Install(dbm); err != nil {
		t.Fatalf("install mail schema: %v", err)
	}

	settingsStore := mailsettings.NewStore(dbm)
	accountsStore := mailaccounts.NewStore(dbm)
	resolver := mailsettings.NewResolver(settingsStore, accountsStore, envCfg, nil)
	return resolver, settingsStore, accountsStore
}

// newTestOrgRegistry builds a real on-disk OrgUserDBRegistry rooted at
// t.TempDir(), migrated from the real org_user_migrations directory —
// so this test exercises the actual 34/35/36 migrations, not a
// hand-copied schema.
func newTestOrgRegistry(t *testing.T) *dbregistry.OrgUserDBRegistry {
	t.Helper()
	return dbregistry.New(t.TempDir(), orgUserMigrationsPath())
}

func TestScopedResolver_Config_FallsBackToGlobalEnvWhenNeitherTierHasAnAccount(t *testing.T) {
	envCfg := mailsmtp.Config{Host: "env.example", Port: 25}
	global, _, _ := newTestGlobalResolver(t, envCfg)
	reg := newTestOrgRegistry(t)

	r := NewScopedResolver(global, reg, nil)
	got := r.Config("org-1", "")
	if got.Host != "env.example" {
		t.Fatalf("Config() = %+v, want env fallback host %q", got, "env.example")
	}
}

func TestScopedResolver_Config_UsesGlobalActiveAccountWhenOrgHasNone(t *testing.T) {
	envCfg := mailsmtp.Config{Host: "env.example", Port: 25}
	global, _, accountsStore := newTestGlobalResolver(t, envCfg)
	reg := newTestOrgRegistry(t)

	if _, err := accountsStore.Create(mailaccounts.Account{
		Name: "global-acc", Host: "global.example", Port: 587, FromEmail: "a@b.com", IsActive: true,
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	r := NewScopedResolver(global, reg, nil)
	got := r.Config("org-1", "")
	if got.Host != "global.example" {
		t.Fatalf("Config() = %+v, want the global active account host %q", got, "global.example")
	}
}

func TestScopedResolver_Config_OrgActiveAccountWinsOverGlobal(t *testing.T) {
	envCfg := mailsmtp.Config{Host: "env.example", Port: 25}
	global, _, accountsStore := newTestGlobalResolver(t, envCfg)
	reg := newTestOrgRegistry(t)

	if _, err := accountsStore.Create(mailaccounts.Account{
		Name: "global-acc", Host: "global.example", Port: 587, FromEmail: "a@b.com", IsActive: true,
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	id, err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailAccountWrite{
		Name: "org-acc", Host: "org.example", Port: 587, FromEmail: "c@d.com", IsActive: true,
	})
	if err != nil {
		t.Fatalf("create org account: %v", err)
	}
	if _, err := orgmail_query.NewOrgMailAccountsQueryRepo(orgDB).Get(id); err != nil {
		t.Fatalf("sanity get org account: %v", err)
	}

	r := NewScopedResolver(global, reg, nil)

	// This org has its own active account — it must win.
	got := r.Config("org-1", "")
	if got.Host != "org.example" {
		t.Fatalf("Config(org-1) = %+v, want the org own active account host %q", got, "org.example")
	}

	// A DIFFERENT org with no account of its own must still fall back
	// to global — confirms the override is scoped per-org, not global.
	other := r.Config("org-2", "")
	if other.Host != "global.example" {
		t.Fatalf("Config(org-2) = %+v, want the global account host %q (org-2 has no account of its own)", other, "global.example")
	}
}

func TestScopedResolver_TemplateAndAccountForEvent_OrgBindingWinsOverGlobal(t *testing.T) {
	global, settingsStore, accountsStore := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)

	if _, err := accountsStore.Create(mailaccounts.Account{Name: "global-acc", Host: "h", FromEmail: "a@b.com"}); err != nil {
		t.Fatalf("create global account: %v", err)
	}
	if err := settingsStore.Set(mailsettings.EventTemplateKey("user_invite"), "global_tpl"); err != nil {
		t.Fatalf("set global template binding: %v", err)
	}
	if err := settingsStore.Set(mailsettings.EventAccountKey("user_invite"), "global-acc"); err != nil {
		t.Fatalf("set global account binding: %v", err)
	}

	r := NewScopedResolver(global, reg, nil)

	// No org-level binding yet — falls back to global. resolvedOrgID
	// must be "" (the global tier, not org-1) so a caller building a
	// MailTask knows to look this account name up in the GLOBAL table.
	if got := r.TemplateForEvent("org-1", "", "user_invite"); got != "global_tpl" {
		t.Fatalf("TemplateForEvent() = %q, want global fallback %q", got, "global_tpl")
	}
	if name, resolvedOrgID, resolvedAppID := r.AccountForEvent("org-1", "", "user_invite"); name != "global-acc" || resolvedOrgID != "" || resolvedAppID != "" {
		t.Fatalf("AccountForEvent() = (%q, %q, %q), want (%q, \"\", \"\")", name, resolvedOrgID, resolvedAppID, "global-acc")
	}

	// Now bind the event at the org level — it must win.
	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	orgSettings := orgmail_persistent.NewOrgMailSettingsPersistentRepo(orgDB)
	if err := orgSettings.Set(orgmail_query.EventTemplateKey("user_invite"), "org_tpl"); err != nil {
		t.Fatalf("set org template binding: %v", err)
	}
	if err := orgSettings.Set(orgmail_query.EventAccountKey("user_invite"), "org-acc"); err != nil {
		t.Fatalf("set org account binding: %v", err)
	}

	if got := r.TemplateForEvent("org-1", "", "user_invite"); got != "org_tpl" {
		t.Fatalf("TemplateForEvent() after org override = %q, want %q", got, "org_tpl")
	}
	// resolvedOrgID must now be "org-1" — the caller needs this to know
	// "org-acc" lives in org-1's own accounts table, not the global one.
	if name, resolvedOrgID, resolvedAppID := r.AccountForEvent("org-1", "", "user_invite"); name != "org-acc" || resolvedOrgID != "org-1" || resolvedAppID != "" {
		t.Fatalf("AccountForEvent() after org override = (%q, %q, %q), want (%q, %q, \"\")", name, resolvedOrgID, resolvedAppID, "org-acc", "org-1")
	}
}

func TestScopedResolver_RenderTemplate_UsesOrgTemplateWhenActive(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	if _, err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailTemplateWrite{
		Name: "user_invite", Subject: "Welcome to {{.Org}}", TextBody: "Hi {{.Name}}", IsActive: true,
	}); err != nil {
		t.Fatalf("create org template: %v", err)
	}

	subject, text, _, ok, err := r.RenderTemplate("org-1", "", "user_invite", map[string]interface{}{"Org": "Acme", "Name": "Jane"})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if !ok {
		t.Fatal("RenderTemplate() ok = false, want true — org has an active template for this name")
	}
	if subject != "Welcome to Acme" {
		t.Fatalf("subject = %q, want %q", subject, "Welcome to Acme")
	}
	if text != "Hi Jane" {
		t.Fatalf("text = %q, want %q", text, "Hi Jane")
	}
}

func TestScopedResolver_RenderTemplate_FallsThroughWhenOrgHasNoTemplate(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	_, _, _, ok, err := r.RenderTemplate("org-1", "", "user_invite", nil)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v, want nil (absence is not an error)", err)
	}
	if ok {
		t.Fatal("RenderTemplate() ok = true, want false — org has no template of its own, caller should use the global renderer")
	}
}

func TestScopedResolver_RenderTemplate_FallsThroughWhenOrgTemplateInactive(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	// Create() always defaults new rows to active (mirrors the global
	// template.Repository.Create's own behavior) — so an inactive row
	// is produced the same way an admin would via the CRUD API:
	// create, then flip is_active off with an update.
	repo := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(orgDB)
	id, err := repo.Create(orgmail_persistent.OrgMailTemplateWrite{Name: "user_invite", Subject: "s", TextBody: "b"})
	if err != nil {
		t.Fatalf("create org template: %v", err)
	}
	if err := repo.Update(orgmail_persistent.OrgMailTemplateWrite{ID: id, Subject: "s", TextBody: "b", IsActive: false}); err != nil {
		t.Fatalf("deactivate org template: %v", err)
	}

	_, _, _, ok, err := r.RenderTemplate("org-1", "", "user_invite", nil)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if ok {
		t.Fatal("RenderTemplate() ok = true, want false — the org template exists but is inactive")
	}
}

func TestScopedResolver_ActivationSettings_OrgOverrideWinsOverGlobalDefault(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	// Neither tier has customized anything — global's own hardcoded
	// defaults (24h / 300s) apply.
	got := r.ActivationSettings("org-1", "")
	if got.TTLHours != 24 || got.ResendCooldownSeconds != 300 {
		t.Fatalf("ActivationSettings() = %+v, want the global defaults (24, 300)", got)
	}

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	orgSettings := orgmail_persistent.NewOrgMailSettingsPersistentRepo(orgDB)
	if err := orgSettings.Set(orgmail_query.KeyActivationTTLHours, "72"); err != nil {
		t.Fatalf("set org ttl override: %v", err)
	}

	got = r.ActivationSettings("org-1", "")
	if got.TTLHours != 72 {
		t.Fatalf("ActivationSettings().TTLHours = %d, want the org override 72", got.TTLHours)
	}
	if got.ResendCooldownSeconds != 300 {
		t.Fatalf("ActivationSettings().ResendCooldownSeconds = %d, want the untouched global default 300", got.ResendCooldownSeconds)
	}
}

func TestScopedResolver_NilOrgRegistry_BehavesExactlyLikeGlobalOnly(t *testing.T) {
	envCfg := mailsmtp.Config{Host: "env.example", Port: 25}
	global, _, _ := newTestGlobalResolver(t, envCfg)

	r := NewScopedResolver(global, nil, nil)
	got := r.Config("org-1", "")
	if got.Host != "env.example" {
		t.Fatalf("Config() with nil org registry = %+v, want plain global fallback %q", got, "env.example")
	}
	// appID must also degrade gracefully with no registry — no panic,
	// same global fallback.
	got = r.Config("", "app-1")
	if got.Host != "env.example" {
		t.Fatalf("Config() with nil org registry and an appID = %+v, want plain global fallback %q", got, "env.example")
	}
}

func TestScopedResolver_Config_AppActiveAccountWinsOverOrgAndGlobal(t *testing.T) {
	envCfg := mailsmtp.Config{Host: "env.example", Port: 25}
	global, _, accountsStore := newTestGlobalResolver(t, envCfg)
	reg := newTestOrgRegistry(t)

	if _, err := accountsStore.Create(mailaccounts.Account{
		Name: "global-acc", Host: "global.example", Port: 587, FromEmail: "a@b.com", IsActive: true,
	}); err != nil {
		t.Fatalf("create global account: %v", err)
	}

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	if _, err := orgmail_persistent.NewOrgMailAccountsPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailAccountWrite{
		Name: "org-acc", Host: "org.example", Port: 587, FromEmail: "c@d.com", IsActive: true,
	}); err != nil {
		t.Fatalf("create org account: %v", err)
	}
	createTestApp(t, orgDB, "app-1")

	r := NewScopedResolver(global, reg, nil)

	// Before the app has its own account, the org's own account wins
	// over global.
	got := r.Config("org-1", "app-1")
	if got.Host != "org.example" {
		t.Fatalf("Config() before app override = %+v, want the org active account host %q", got, "org.example")
	}

	if _, err := appmail_persistent.NewAppMailAccountsPersistentRepo(orgDB, "app-1").Create(appmail_persistent.AppMailAccountWrite{
		Name: "app-acc", Host: "app.example", Port: 587, FromEmail: "e@f.com", IsActive: true,
	}); err != nil {
		t.Fatalf("create app account: %v", err)
	}

	// Now the app has its own active account — it must win over both
	// the org's and the global one.
	got = r.Config("org-1", "app-1")
	if got.Host != "app.example" {
		t.Fatalf("Config() = %+v, want the app own active account host %q", got, "app.example")
	}

	// A DIFFERENT application in the SAME org, with no account of its
	// own, must still fall back to the org's account — confirms the
	// override is scoped per-application, not per-org.
	createTestApp(t, orgDB, "app-2")
	other := r.Config("org-1", "app-2")
	if other.Host != "org.example" {
		t.Fatalf("Config() for a sibling app with no account of its own = %+v, want the org account host %q", other, "org.example")
	}
}

func TestScopedResolver_TemplateAndAccountForEvent_AppBindingWinsOverOrgAndGlobal(t *testing.T) {
	global, settingsStore, accountsStore := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)

	if _, err := accountsStore.Create(mailaccounts.Account{Name: "global-acc", Host: "h", FromEmail: "a@b.com"}); err != nil {
		t.Fatalf("create global account: %v", err)
	}
	if err := settingsStore.Set(mailsettings.EventTemplateKey("user_invite"), "global_tpl"); err != nil {
		t.Fatalf("set global template binding: %v", err)
	}
	if err := settingsStore.Set(mailsettings.EventAccountKey("user_invite"), "global-acc"); err != nil {
		t.Fatalf("set global account binding: %v", err)
	}

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	orgSettings := orgmail_persistent.NewOrgMailSettingsPersistentRepo(orgDB)
	if err := orgSettings.Set(orgmail_query.EventTemplateKey("user_invite"), "org_tpl"); err != nil {
		t.Fatalf("set org template binding: %v", err)
	}
	if err := orgSettings.Set(orgmail_query.EventAccountKey("user_invite"), "org-acc"); err != nil {
		t.Fatalf("set org account binding: %v", err)
	}
	createTestApp(t, orgDB, "app-1")

	r := NewScopedResolver(global, reg, nil)

	// No app-level binding yet — falls back to the org's own binding.
	if got := r.TemplateForEvent("org-1", "app-1", "user_invite"); got != "org_tpl" {
		t.Fatalf("TemplateForEvent() before app override = %q, want the org binding %q", got, "org_tpl")
	}
	if name, resolvedOrgID, resolvedAppID := r.AccountForEvent("org-1", "app-1", "user_invite"); name != "org-acc" || resolvedOrgID != "org-1" || resolvedAppID != "" {
		t.Fatalf("AccountForEvent() before app override = (%q, %q, %q), want (%q, %q, \"\")", name, resolvedOrgID, resolvedAppID, "org-acc", "org-1")
	}

	// Now bind the event at the application level — it must win over
	// both the org's and the global bindings.
	appSettings := appmail_persistent.NewAppMailSettingsPersistentRepo(orgDB, "app-1")
	if err := appSettings.Set(appmail_query.EventTemplateKey("user_invite"), "app_tpl"); err != nil {
		t.Fatalf("set app template binding: %v", err)
	}
	if err := appSettings.Set(appmail_query.EventAccountKey("user_invite"), "app-acc"); err != nil {
		t.Fatalf("set app account binding: %v", err)
	}

	if got := r.TemplateForEvent("org-1", "app-1", "user_invite"); got != "app_tpl" {
		t.Fatalf("TemplateForEvent() after app override = %q, want %q", got, "app_tpl")
	}
	// resolvedAppID must now be "app-1" (and resolvedOrgID empty) — the
	// caller needs this to know "app-acc" lives in app-1's own
	// accounts table, not org-1's or the global one.
	if name, resolvedOrgID, resolvedAppID := r.AccountForEvent("org-1", "app-1", "user_invite"); name != "app-acc" || resolvedOrgID != "" || resolvedAppID != "app-1" {
		t.Fatalf("AccountForEvent() after app override = (%q, %q, %q), want (%q, \"\", %q)", name, resolvedOrgID, resolvedAppID, "app-acc", "app-1")
	}

	// A DIFFERENT application in the SAME org, with no binding of its
	// own, must still fall back to the org's binding — confirms the
	// override is scoped per-application, not per-org.
	createTestApp(t, orgDB, "app-2")
	if got := r.TemplateForEvent("org-1", "app-2", "user_invite"); got != "org_tpl" {
		t.Fatalf("TemplateForEvent() for a sibling app with no binding of its own = %q, want the org binding %q", got, "org_tpl")
	}
}

func TestScopedResolver_RenderTemplate_UsesAppTemplateWhenActive(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	if _, err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailTemplateWrite{
		Name: "user_invite", Subject: "Org welcome to {{.Org}}", TextBody: "Hi {{.Name}}", IsActive: true,
	}); err != nil {
		t.Fatalf("create org template: %v", err)
	}
	createTestApp(t, orgDB, "app-1")
	if _, err := appmail_persistent.NewAppMailTemplatesPersistentRepo(orgDB, "app-1").Create(appmail_persistent.AppMailTemplateWrite{
		Name: "user_invite", Subject: "App welcome to {{.Org}}", TextBody: "Hi {{.Name}}", IsActive: true,
	}); err != nil {
		t.Fatalf("create app template: %v", err)
	}

	subject, text, _, ok, err := r.RenderTemplate("org-1", "app-1", "user_invite", map[string]interface{}{"Org": "Acme", "Name": "Jane"})
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if !ok {
		t.Fatal("RenderTemplate() ok = false, want true — the app has an active template for this name")
	}
	if subject != "App welcome to Acme" {
		t.Fatalf("subject = %q, want the APP template's rendering %q (not the org's)", subject, "App welcome to Acme")
	}
	if text != "Hi Jane" {
		t.Fatalf("text = %q, want %q", text, "Hi Jane")
	}
}

func TestScopedResolver_RenderTemplate_FallsThroughToOrgWhenAppHasNoTemplate(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	if _, err := orgmail_persistent.NewOrgMailTemplatesPersistentRepo(orgDB).Create(orgmail_persistent.OrgMailTemplateWrite{
		Name: "user_invite", Subject: "Org welcome", TextBody: "Hi", IsActive: true,
	}); err != nil {
		t.Fatalf("create org template: %v", err)
	}
	createTestApp(t, orgDB, "app-1")

	subject, _, _, ok, err := r.RenderTemplate("org-1", "app-1", "user_invite", nil)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}
	if !ok {
		t.Fatal("RenderTemplate() ok = false, want true — should fall through to the org's own template")
	}
	if subject != "Org welcome" {
		t.Fatalf("subject = %q, want the ORG template's rendering %q since the app has none of its own", subject, "Org welcome")
	}
}

func TestScopedResolver_ActivationSettings_AppOverrideWinsOverOrgAndGlobal(t *testing.T) {
	global, _, _ := newTestGlobalResolver(t, mailsmtp.Config{})
	reg := newTestOrgRegistry(t)
	r := NewScopedResolver(global, reg, nil)

	orgDB, err := orgrouter.ForOrg(reg, "org-1")
	if err != nil {
		t.Fatalf("open org db: %v", err)
	}
	orgSettings := orgmail_persistent.NewOrgMailSettingsPersistentRepo(orgDB)
	if err := orgSettings.Set(orgmail_query.KeyActivationTTLHours, "72"); err != nil {
		t.Fatalf("set org ttl override: %v", err)
	}
	createTestApp(t, orgDB, "app-1")

	// Before any app override, the org's own override applies.
	got := r.ActivationSettings("org-1", "app-1")
	if got.TTLHours != 72 {
		t.Fatalf("ActivationSettings() before app override = %+v, want the org override 72", got)
	}
	if got.ResendCooldownSeconds != 300 {
		t.Fatalf("ActivationSettings().ResendCooldownSeconds = %d, want the untouched global default 300", got.ResendCooldownSeconds)
	}

	appSettings := appmail_persistent.NewAppMailSettingsPersistentRepo(orgDB, "app-1")
	if err := appSettings.Set(appmail_query.KeyActivationTTLHours, "12"); err != nil {
		t.Fatalf("set app ttl override: %v", err)
	}

	got = r.ActivationSettings("org-1", "app-1")
	if got.TTLHours != 12 {
		t.Fatalf("ActivationSettings().TTLHours = %d, want the app override 12 to win over the org's 72", got.TTLHours)
	}

	// A DIFFERENT application in the SAME org, with no override of its
	// own, must still fall back to the org's override — confirms the
	// app override is scoped per-application, not per-org.
	createTestApp(t, orgDB, "app-2")
	other := r.ActivationSettings("org-1", "app-2")
	if other.TTLHours != 72 {
		t.Fatalf("ActivationSettings() for a sibling app with no override of its own = %+v, want the org override 72", other)
	}
}
