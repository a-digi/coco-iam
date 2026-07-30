package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/a-digi/coco-iam/config"
	"github.com/a-digi/coco-iam/config/di"
	"github.com/a-digi/coco-iam/config/routes"
	"github.com/a-digi/coco-iam/src/activation"
	archives_persistent "github.com/a-digi/coco-iam/src/admin/security/archives/repository/persistent"
	loginlog_persistent "github.com/a-digi/coco-iam/src/admin/security/loginlog/repository/persistent"
	scans_persistent "github.com/a-digi/coco-iam/src/admin/security/scans/repository/persistent"
	adminpwnotify "github.com/a-digi/coco-iam/src/admin/users/passwordnotify"
	admin_avatar "github.com/a-digi/coco-iam/src/admin/users/profile/avatar"
	apicred_dbregistry "github.com/a-digi/coco-iam/src/applications/apicredentials/dbregistry"
	"github.com/a-digi/coco-iam/src/applications/cleanup"
	app_keys "github.com/a-digi/coco-iam/src/applications/keys"
	loginlog_dbregistry "github.com/a-digi/coco-iam/src/applications/loginlog/dbregistry"
	app_loginpage "github.com/a-digi/coco-iam/src/applications/loginpage"
	app_recoverypage "github.com/a-digi/coco-iam/src/applications/recoverypage"
	password_svc "github.com/a-digi/coco-iam/src/auth/password"
	"github.com/a-digi/coco-iam/src/auth/recovery"
	"github.com/a-digi/coco-iam/src/datamigration"
	"github.com/a-digi/coco-iam/src/general"
	iam_mail "github.com/a-digi/coco-iam/src/mail"
	mailaccounts "github.com/a-digi/coco-iam/src/mail/accounts"
	mailconsumer "github.com/a-digi/coco-iam/src/mail/consumer"
	mailsettings "github.com/a-digi/coco-iam/src/mail/settings"
	mailsmtp "github.com/a-digi/coco-iam/src/mail/smtp"
	mailstore "github.com/a-digi/coco-iam/src/mail/store"
	mailtemplate "github.com/a-digi/coco-iam/src/mail/template"
	oauth_archiver "github.com/a-digi/coco-iam/src/oauthserver/archiver"
	oauth_dbregistry "github.com/a-digi/coco-iam/src/oauthserver/dbregistry"
	organization_deleted "github.com/a-digi/coco-iam/src/organizations/deleted"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	org_user_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	org_user_notify "github.com/a-digi/coco-iam/src/organizations/users/notify"
	orgpwnotify "github.com/a-digi/coco-iam/src/organizations/users/passwordnotify"
	"github.com/a-digi/coco-iam/src/orgrouter"
	"github.com/a-digi/coco-iam/src/security/dbarchive"
	"github.com/a-digi/coco-iam/src/security/dbhandle"
	"github.com/a-digi/coco-iam/src/security/ipguard"
	"github.com/a-digi/coco-iam/src/security/scanwatch"
	"github.com/a-digi/coco-iam/src/userrules"
	"github.com/a-digi/coco-logger/logger"
	"github.com/a-digi/coco-queue"
	queue_dbregistry "github.com/a-digi/coco-queue/dbregistry"
	"github.com/a-digi/coco-server/server"
	app_media "github.com/a-digi/coco-server/server/media"

	dbInstall "github.com/a-digi/coco-iam/config/db"
	dbmanager "github.com/a-digi/coco-orm/orm"
)

func main() {
	configPath := "config.json"

	if len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	action := "start"

	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {

	case "start":
		log, err := initLogger()
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}

		defer log.Close()
		migrationsPath, err := config.ExtractMigrationsToTemp()
		if err != nil {
			fmt.Printf("Migrationen konnten nicht extrahiert werden: %v\n", err)
			os.Exit(1)
		}

		manager, err := dbmanager.NewDatabaseManager("users.db", "./data/db", []string{migrationsPath})

		if err != nil {
			fmt.Printf("DatabaseManager creation failed: %v\n", err)
			os.Exit(1)
		}

		err = dbInstall.Install(manager)

		if err != nil {
			fmt.Printf("Database install failed: %v\n", err)
			os.Exit(1)
		}

		err = dbInstall.EnsureHasSuperadmin(manager)
		if err != nil {
			fmt.Printf("Superadmin creation: %v\n", err)
			os.Exit(1)
		}

		// ip-attacks.db — a separate, self-contained database holding
		// the historical record of past IP-abuse episodes (own file,
		// own migration sequence), kept out of the main users.db so
		// that ever-growing history doesn't bloat it. Consumed by
		// IPGuardSecurityLayer, wired up below via ContextBag — see
		// plan/ip-abuse-protection/plan.md section 10.
		ipAttacksMigrationsPath, err := config.ExtractIPAttacksMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract ip-attacks migrations: %v", err)
			os.Exit(1)
		}
		ipAttacksDBManager, err := dbmanager.NewDatabaseManager("ip-attacks.db", "./data/db/security", []string{ipAttacksMigrationsPath})
		if err != nil {
			log.Error("ip-attacks DatabaseManager creation failed: %v", err)
			os.Exit(1)
		}
		if err := ipAttacksDBManager.SyncMigrations(); err != nil {
			log.Error("ip-attacks migrations failed: %v", err)
			os.Exit(1)
		}

		// Wraps ipAttacksDBManager.Connector.DB in a swappable handle —
		// ipguard and the admin attacks query handler both hold this
		// (not the raw *sql.DB), so the archiver below can rotate
		// ip-attacks.db out and hand every consumer a fresh connection
		// without reconstructing any of them.
		ipAttacksHandle, err := dbhandle.New(ipAttacksDBManager.Connector.DB)
		if err != nil {
			log.Error("ip-attacks handle creation failed: %v", err)
			os.Exit(1)
		}

		// Rotates ip-attacks.db into data/db/security/archives once its
		// entry count crosses the threshold, registering the rotated-out
		// file in the main DB so it stays queryable later. Started below
		// alongside the other sweepers, once queueCtx exists. See
		// plan/ip-attacks-db-archiving/plan.md.
		ipAttacksArchiveRecorder := archives_persistent.NewArchiveRecorder(manager.Connector.DB)
		dbArchiver := dbarchive.New(
			ipAttacksHandle, ipAttacksDBManager, ipAttacksArchiveRecorder,
			"ip-attacks.db", "./data/db/security", ipAttacksMigrationsPath,
			"./data/db/security/archives", dbarchive.DefaultThreshold, log,
		)

		// admin_login.db — a separate, self-contained database holding
		// the admin-console login attempt history (success/failure,
		// who, when, from where), kept out of the main users.db for the
		// same reason ip-attacks.db is. Rotated by the same generalized
		// dbarchive.Archiver. See plan/login-audit-log/plan.md Step 2.
		adminLoginMigrationsPath, err := config.ExtractAdminLoginMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract admin-login migrations: %v", err)
			os.Exit(1)
		}
		adminLoginDBManager, err := dbmanager.NewDatabaseManager("admin_login.db", "./data/db/security", []string{adminLoginMigrationsPath})
		if err != nil {
			log.Error("admin-login DatabaseManager creation failed: %v", err)
			os.Exit(1)
		}
		if err := adminLoginDBManager.SyncMigrations(); err != nil {
			log.Error("admin-login migrations failed: %v", err)
			os.Exit(1)
		}
		adminLoginHandle, err := dbhandle.New(adminLoginDBManager.Connector.DB)
		if err != nil {
			log.Error("admin-login handle creation failed: %v", err)
			os.Exit(1)
		}
		adminLoginArchiveRecorder := loginlog_persistent.NewArchiveRecorder(manager.Connector.DB)
		adminLoginArchiver := dbarchive.New(
			adminLoginHandle, adminLoginDBManager, adminLoginArchiveRecorder,
			"admin_login.db", "./data/db/security", adminLoginMigrationsPath,
			"./data/db/security/archives", dbarchive.DefaultThreshold, log,
		)

		// Port-scan detection — ingests OS-level firewall log lines to
		// see traffic against ports coco-iam isn't listening on at all,
		// architecturally invisible to ipguard's own rate limiter. Never
		// detects anything itself; it only consumes whichever log source
		// is actually available on this host (journald, else a syslog
		// file, else disabled) — see
		// plan/port-scan-detection/plan.md Phase B and
		// docs/setup/port-scan-detection.md for the one-time,
		// operator-run iptables logging rule this depends on.
		scanSource := scanwatch.Detect(log, scanwatch.DefaultSyslogFilePath)
		scanPersist := scans_persistent.NewScanPersistentRepo(ipAttacksHandle)
		// scanWatcher itself is constructed further down, right after
		// routes.Init(ctx) — it needs the same real geoip.Lookup
		// routes.Init builds (ctx.GeoIP), which doesn't exist yet at
		// this point in boot. Nothing between here and there depends
		// on scanWatcher already existing.

		// Dedicated attack log — one line per rejected request while an
		// IP is under an open attack episode, kept separate from the
		// main server log so operators can tail/rotate it independently
		// and so per-request detail never has to live in the (small,
		// aggregated-only) ip-attacks.db. See
		// plan/ip-abuse-protection/plan.md section 12.
		ipAttacksLog, err := logger.NewLogger("ip-attacks.log", "data/logs/security")
		if err != nil {
			log.Error("failed to initialize ip-attacks log: %v", err)
			os.Exit(1)
		}

		// Create ContextBag and inject manager and logger
		ctx := di.NewContextBag(manager, log)
		ctx.IPAttacksHandle = ipAttacksHandle
		ctx.DBArchiver = dbArchiver
		ctx.IPAttacksLog = ipAttacksLog
		ctx.ScanSource = scanSource
		ctx.AdminLoginHandle = adminLoginHandle
		ctx.AdminLoginArchiver = adminLoginArchiver

		// Per-organization profile databases. Each org has its own SQLite
		// file that holds the profile-field schema and the user profile
		// values. Files are provisioned on org-create via the listener
		// registered in config/resource/entities_api_resources.go.
		orgMigrationsPath, err := config.ExtractOrgMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract org migrations: %v", err)
			os.Exit(1)
		}
		orgDBRegistry := profile_dbregistry.New("./data/db", orgMigrationsPath)
		if err := orgDBRegistry.SweepExisting(); err != nil {
			log.Warning("org db sweep: %v", err)
		}
		ctx.Set(profile_dbregistry.ContextBagKey, orgDBRegistry)

		// Per-organization user databases. Parallel file to the profile
		// DB above — same lifecycle (provisioned on org-create via a
		// PostEventListener, migrations applied via SweepExisting at
		// boot), different content (users + passwords + ACLs +
		// user-group memberships). Split from profiles on purpose: large
		// orgs with millions of users won't contend with profile I/O,
		// and each file can be backed up / restored independently.
		orgUserMigrationsPath, err := config.ExtractOrgUserMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract org user migrations: %v", err)
			os.Exit(1)
		}
		orgUserDBRegistry := org_user_dbregistry.New("./data/db", orgUserMigrationsPath)
		if err := orgUserDBRegistry.SweepExisting(); err != nil {
			log.Warning("org user db sweep: %v", err)
		}
		ctx.Set(org_user_dbregistry.ContextBagKey, orgUserDBRegistry)

		// Per-application login-log databases — one SQLite file per
		// application, nested under its owning org, rotated by the
		// same generalized dbarchive.Archiver ip-attacks.db/
		// admin_login.db use. See plan/login-audit-log/plan.md Step 6.
		appLoginMigrationsPath, err := config.ExtractApplicationLoginMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract application login migrations: %v", err)
			os.Exit(1)
		}
		appLoginLogRegistry := loginlog_dbregistry.New(
			"./data/db", appLoginMigrationsPath, dbarchive.DefaultThreshold, orgUserDBRegistry, log,
		)
		if err := appLoginLogRegistry.SweepExisting(); err != nil {
			log.Warning("application login-log db sweep: %v", err)
		}
		ctx.Set(loginlog_dbregistry.ContextBagKey, appLoginLogRegistry)

		if err := datamigration.MigrateWorkspaceAndAppsToOrgDBs(
			manager.Connector.DB, orgUserDBRegistry, log,
		); err != nil {
			log.Warning("workspace/apps data migration: %v", err)
		}

		if err := datamigration.MigrateOrgGroupsToOrgDBs(
			manager.Connector.DB, orgUserDBRegistry, log,
		); err != nil {
			log.Warning("org groups data migration: %v", err)
		}

		// Per-organization api-credentials database. Third file in the
		// per-org folder, holds the machine-auth material (api_id +
		// bcrypt-hashed secret) that public /a/... endpoints consume.
		// Split from users.db on purpose: credentials are a different
		// auth domain with different access patterns (low-volume
		// lookups) and an independent lifecycle (revoke / expire).
		orgApiCredMigrationsPath, err := config.ExtractOrgApiCredentialsMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract org api-credentials migrations: %v", err)
			os.Exit(1)
		}
		orgApiCredDBRegistry := apicred_dbregistry.New("./data/db", orgApiCredMigrationsPath)
		if err := orgApiCredDBRegistry.SweepExisting(); err != nil {
			log.Warning("org api-credentials db sweep: %v", err)
		}
		ctx.Set(apicred_dbregistry.ContextBagKey, orgApiCredDBRegistry)

		// Per-organization OAuth database. Holds oauth_auth_requests,
		// oauth_authorization_codes, and oauth_refresh_tokens — the
		// token material issued by the coco-iam OAuth provider. Split
		// from users.db so token I/O doesn't contend with user lookups
		// and so the whole token store can be archived independently.
		// Routing uses per-org KnownOrgIDs scan; oauth_token_org_index has been removed.
		orgOAuthMigrationsPath, err := config.ExtractOrgOAuthMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract org oauth migrations: %v", err)
			os.Exit(1)
		}
		orgOAuthDBRegistry := oauth_dbregistry.New("./data/db", orgOAuthMigrationsPath)
		if err := orgOAuthDBRegistry.SweepExisting(); err != nil {
			log.Warning("org oauth db sweep: %v", err)
		}
		ctx.Set(oauth_dbregistry.ContextBagKey, orgOAuthDBRegistry)

		if err := datamigration.MigrateOAuthTablesToOrgDBs(
			manager.Connector.DB, orgOAuthDBRegistry, log,
		); err != nil {
			log.Warning("oauth data migration: %v", err)
		}

		if err := datamigration.MigrateActivationTablesToOrgDBs(
			manager.Connector.DB, orgUserDBRegistry, log,
		); err != nil {
			log.Warning("activation data migration: %v", err)
		}

		if err := datamigration.MigrateRecoveryTablesToOrgDBs(
			manager.Connector.DB, orgUserDBRegistry, log,
		); err != nil {
			log.Warning("recovery data migration: %v", err)
		}

		if err := datamigration.MigrateRuleSetsTablesToOrgDBs(
			manager.Connector.DB, orgUserDBRegistry, log,
		); err != nil {
			log.Warning("rule sets data migration: %v", err)
		}

		// Queue DB registry — main.db (queues + tasks_index) plus one
		// file per registered queue at ./data/db/queue/<id>_<name>.db
		// with a sibling folder ./data/db/queue/<id>_<name>/ for payload
		// JSON. Splitting per queue removes WAL contention between
		// domains and makes on-disk triage trivial.
		queueMainMigrationsPath, err := config.ExtractQueueMainMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract queue main migrations: %v", err)
			os.Exit(1)
		}
		queueMigrationsPath, err := config.ExtractQueueMigrationsToTemp()
		if err != nil {
			log.Error("failed to extract per-queue migrations: %v", err)
			os.Exit(1)
		}
		queueRegistry, err := queue_dbregistry.New("./data/db", queueMainMigrationsPath, queueMigrationsPath)
		if err != nil {
			log.Error("failed to initialise queue registry: %v", err)
			os.Exit(1)
		}
		if err := queueRegistry.SweepExisting(); err != nil {
			log.Warning("queue registry sweep: %v", err)
		}

		// Construct and start the async queue manager. Consumers register their
		// handlers via queueMgr.Register before Start is called; see per-domain
		// wiring packages (e.g. applications cleanup).
		queueMgr := queue.NewManager(queueRegistry, log)
		ctx.Set(queue.ContextBagKey, queue.Manager(queueMgr))

		// Register built-in queue consumers here. Adding a new queue from a
		// different domain? Import that package and call its Register function
		// before queueMgr.Start(ctx) runs.
		if err := cleanup.Register(queueMgr, manager, orgUserDBRegistry, log); err != nil {
			log.Error("failed to register application-user-cleanup consumer: %v", err)
			os.Exit(1)
		}
		if err := organization_deleted.Register(queueMgr, manager, orgUserDBRegistry, log); err != nil {
			log.Error("failed to register organization-deleted consumer: %v", err)
			os.Exit(1)
		}
		// Normalise the deletion archive to the new
		// `data/db/deleted/organization/<orgID>/` layout. Two passes
		// because two legacy shapes may be on disk:
		//   (a) ./data/db/deleted_databases/<stamp>__<orgID>/ — the
		//       oldest pre-rename layout.
		//   (b) ./data/db/deleted/<orgID>/ (flat under deleted/) — the
		//       intermediate layout from the previous migration.
		// Both end up at ./data/db/deleted/organization/<orgID>/.
		// Idempotent: missing source folders are no-ops.
		const newArchiveOrgRoot = "./data/db/deleted/organization"
		if report, mErr := organization_deleted.MigrateLegacyArchiveDir(
			organization_deleted.LegacyDeletedDir, newArchiveOrgRoot,
		); mErr != nil {
			log.Warning("deleted-archive migration (legacy root): %v", mErr)
		} else if len(report.Moved) > 0 || len(report.Skipped) > 0 || len(report.Failures) > 0 {
			log.Info("deleted-archive migration (legacy root): moved %d, skipped %d, failed %d",
				len(report.Moved), len(report.Skipped), len(report.Failures))
			for name, ferr := range report.Failures {
				log.Warning("deleted-archive migration (legacy root): failed for %s: %v", name, ferr)
			}
		}
		// The intermediate flat layout: anything directly under
		// `./data/db/deleted/` that isn't the `organization/`
		// subfolder itself gets promoted into it.
		if report, mErr := organization_deleted.MigrateLegacyArchiveDir(
			"./data/db/deleted", newArchiveOrgRoot, "organization",
		); mErr != nil {
			log.Warning("deleted-archive migration (flat): %v", mErr)
		} else if len(report.Moved) > 0 || len(report.Skipped) > 0 || len(report.Failures) > 0 {
			log.Info("deleted-archive migration (flat): moved %d, skipped %d, failed %d",
				len(report.Moved), len(report.Skipped), len(report.Failures))
			for name, ferr := range report.Failures {
				log.Warning("deleted-archive migration (flat): failed for %s: %v", name, ferr)
			}
		}

		// Mail engine — SMTP is the only implementation for now. Swapping
		// provider = a new package under src/mail/<provider>/ and one line
		// change here that builds a different Mailer.
		mailDB, err := dbmanager.NewDatabaseManager("mail.db", "./data/db", nil)
		if err != nil {
			log.Error("failed to open mail.db: %v", err)
			os.Exit(1)
		}
		if err := mailstore.Install(mailDB); err != nil {
			log.Error("failed to install mail schema: %v", err)
			os.Exit(1)
		}

		mailCfg := mailsmtp.ConfigFromEnv()
		mailer := mailsmtp.New(mailCfg, log)
		templateRepo := mailtemplate.NewRepository(mailDB)
		mailRenderer, err := mailtemplate.New(config.ConfigFS, mailtemplate.WithRepository(templateRepo))
		if err != nil {
			log.Error("failed to initialise mail templates: %v", err)
			os.Exit(1)
		}
		mailStoreInstance := mailstore.New(mailDB, log)
		if err := mailStoreInstance.ResetSending(); err != nil {
			log.Warning("mail: reset stale 'sending' rows failed: %v", err)
		}

		// Mail settings + named SMTP accounts. The resolver reads the active
		// account from mail.db (env fallback when none exists); wiring it as
		// a ConfigProvider on the SMTPMailer means admin edits take effect
		// on the next send with no restart.
		mailSettingsStore := mailsettings.NewStore(mailDB)
		mailAccountsStore := mailaccounts.NewStore(mailDB)
		if err := mailsettings.MigrateLegacySMTPIfNeeded(mailSettingsStore, mailAccountsStore, mailCfg, log); err != nil {
			log.Warning("mail migration: legacy SMTP settings could not be migrated: %v", err)
		}
		mailSettingsResolver := mailsettings.NewResolver(mailSettingsStore, mailAccountsStore, mailCfg, log)
		mailer.SetConfigProvider(func() mailsmtp.Config { return mailSettingsResolver.Config() })

		mailService := iam_mail.NewMailService(queueMgr, mailStoreInstance, mailRenderer, mailCfg.From)

		orchestratorCfg := mailconsumer.OrchestratorConfigFromEnv()
		orchestrator := mailconsumer.NewOrchestrator(queueMgr, mailStoreInstance, orchestratorCfg, log)

		ctx.Set(iam_mail.ContextBagKeyMailer, iam_mail.Mailer(mailer))
		ctx.Set(iam_mail.ContextBagKeyMailService, mailService)
		ctx.Set(iam_mail.ContextBagKeyMailStore, mailStoreInstance)
		ctx.Set(iam_mail.ContextBagKeyTemplateRepository, templateRepo)
		ctx.Set(iam_mail.ContextBagKeySettingsStore, mailSettingsStore)
		ctx.Set(iam_mail.ContextBagKeySettingsResolver, mailSettingsResolver)
		ctx.Set(iam_mail.ContextBagKeyAccountsStore, mailAccountsStore)

		// Activation service — admin_activations lives in users.db; per-org
		// user_activations live in each org's users.db. Base URL and branding
		// are read from each org's per-org DB at request time.
		adminActivationStore := activation.NewAdminStore(manager)
		orgActivationStore := activation.NewOrgStore(orgUserDBRegistry)
		activationSettings := activation.NewSettingsReader(mailSettingsStore)
		// User-rules store — configurable validation knobs for
		// Global branding settings (base URL, page title, description,
		// robots). Backed by the main DB app_settings table.
		generalStore := general.NewStoreFromDB(manager.Connector.DB)
		ctx.Set(general.ContextBagKeyStore, generalStore)

		// usernames / emails / passwords, split by scope (admin-wide
		// vs. per-organization). Consulted on every mutating flow.
		userRulesStore := userrules.NewStore(manager, orgUserDBRegistry)
		ctx.Set(userrules.ContextBagKeyStore, userRulesStore)
		adminRulesStore := userrules.NewAdminStore(manager)
		orgRulesStore := userrules.NewOrgStore(orgUserDBRegistry)

		activationService := activation.NewService(
			manager.Connector.DB,
			orgUserDBRegistry,
			adminActivationStore,
			orgActivationStore,
			mailService,
			mailSettingsResolver,
			activationSettings,
			userRulesStore,
			log,
		)
		ctx.Set(activation.ContextBagKeyService, activationService)

		// Org user lifecycle notifications — deactivation and removal emails.
		orgUserNotifyService := org_user_notify.NewService(
			manager.Connector.DB,
			orgUserDBRegistry,
			mailSettingsResolver,
			mailService,
			log,
		)
		ctx.Set(org_user_notify.ContextBagKey, orgUserNotifyService)

		// Self-service password change — takes the rules store so it
		// can reject weak passwords per the admin/org configuration.
		passwordService := password_svc.NewService(manager, orgUserDBRegistry, userRulesStore)
		ctx.Set(password_svc.ContextBagKeyService, passwordService)

		// Password recovery — email-reset flow. Parallel to activation
		// but with its own token table + TTL defaults. Reuses the same
		// mail engine, the general store for the base URL, and the
		// user-rules store for password validation.
		adminRecoveryStore := recovery.NewAdminStore(manager)
		orgRecoveryStore := recovery.NewOrgStore(orgUserDBRegistry)
		recoverySettings := recovery.NewSettingsReader(mailSettingsStore)
		recoveryService := recovery.NewService(
			manager.Connector.DB,
			orgUserDBRegistry,
			adminRecoveryStore,
			orgRecoveryStore,
			mailService,
			mailSettingsResolver,
			recoverySettings,
			userRulesStore,
			log,
		)
		ctx.Set(recovery.ContextBagKeyService, recoveryService)

		// Per-application login-page subsystem — full-HTML templates,
		// image assets, and the public authenticate endpoint scoped
		// to application_user_acl.
		appLoginFiles, err := app_loginpage.NewFileStore("./data/uploads/app-login")
		if err != nil {
			log.Error("app loginpage: init file store: %v", err)
			os.Exit(1)
		}
		appLoginService := app_loginpage.NewService(app_loginpage.NewStore(manager.Connector.DB, orgUserDBRegistry), appLoginFiles)
		ctx.Set(app_loginpage.ContextBagKeyService, appLoginService)

		// Per-application password-recovery subsystem — custom HTML
		// templates + the request/reset flow scoped to users on the
		// application's ACL. Reuses the global recovery.Store (token
		// rows) and mail engine so there's one source of truth for
		// token TTLs, cooldowns, and delivery.
		appRecoveryService := app_recoverypage.NewService(
			manager.Connector.DB,
			orgUserDBRegistry,
			app_recoverypage.NewStore(manager),
			orgRecoveryStore,
			mailService,
			mailSettingsResolver,
			recoverySettings,
			userRulesStore,
			log,
		)
		ctx.Set(app_recoverypage.ContextBagKeyService, appRecoveryService)

		// Per-application RSA keypair — used to sign the JWTs issued
		// by the public `/login/a/:ws/:app` endpoint and exposed via
		// JWKS for downstream verification. Keys land on disk at
		// `./data/db/organization/<orgID>/auth/<applicationID>/<kid>/`
		// so they ride the same archive-on-delete path as the per-org
		// SQLite files. Generation happens lazily from the
		// `applications` PostEventListener.
		appKeysResolver := func(appID string) (string, error) {
			_, orgID, err := orgrouter.OrgDBForApp(orgUserDBRegistry, appID)
			return orgID, err
		}
		appKeysFiles, err := app_keys.NewFileStore(app_keys.StorageBaseDir, appKeysResolver)
		if err != nil {
			log.Error("app keys: init file store: %v", err)
			os.Exit(1)
		}
		// One-shot migration from the legacy flat layout
		// (./data/appkeys/<appID>/...) into the new org-nested one.
		// Idempotent — running a second time is a no-op once the
		// legacy root is empty.
		if report, mErr := appKeysFiles.MigrateFromLegacy(app_keys.LegacyStorageSubdir); mErr != nil {
			log.Warning("app keys: legacy migration: %v", mErr)
		} else if len(report.Moved) > 0 || len(report.Skipped) > 0 || len(report.Failures) > 0 {
			log.Info("app keys: migrated %d, skipped %d, failed %d",
				len(report.Moved), len(report.Skipped), len(report.Failures))
			for appID, ferr := range report.Failures {
				log.Warning("app keys: migration failed for app %s: %v", appID, ferr)
			}
		}
		keysDBResolve := func(appID string) (*sql.DB, error) {
			orgDB, _, err := orgrouter.OrgDBForApp(orgUserDBRegistry, appID)
			if err != nil {
				return nil, fmt.Errorf("keys: resolve org for %s: %w", appID, err)
			}
			return orgDB, nil
		}
		appKeysService := app_keys.NewService(app_keys.NewStore(keysDBResolve), appKeysFiles)
		ctx.Set(app_keys.ContextBagKeyService, appKeysService)

		// Media — general-purpose per-application file storage (folders
		// + images/CSS/fonts/PDFs). Supersedes the single-purpose
		// login-page assets uploader going forward.
		mediaService, mediaErr := app_media.NewService(app_media.NewStore(manager), "./data/uploads/media")
		if mediaErr != nil {
			log.Error("media: init: %v", mediaErr)
			os.Exit(1)
		}
		ctx.Set(app_media.ContextBagKeyService, mediaService)

		// Admin-user avatar store — per-admin 1-file blob at
		// ./data/uploads/admin-avatars/<admin_user_id>.<ext>.
		// Separate from the per-app media system because admins
		// aren't applications and the avatar lifecycle is simpler
		// (one file, overwrite-on-upload).
		adminAvatarStore, err := admin_avatar.New("./data/uploads/admin-avatars")
		if err != nil {
			log.Error("admin avatar store: init: %v", err)
			os.Exit(1)
		}
		ctx.Set(admin_avatar.ContextBagKeyStore, adminAvatarStore)

		ctx.Set(mailconsumer.ContextBagKeyOrchestrator, orchestrator)

		if err := mailconsumer.Register(
			queueMgr, mailStoreInstance, mailer, mailAccountsStore,
			mailconsumer.Config{
				MaxAttempts:    5,
				InitialWorkers: orchestratorCfg.Min,
			},
			log,
		); err != nil {
			log.Error("failed to register mail-outbound consumer: %v", err)
			os.Exit(1)
		}

		if err := adminpwnotify.Register(queueMgr, mailSettingsResolver, mailService, log); err != nil {
			log.Error("failed to register admin-password-expiry-notification consumer: %v", err)
			os.Exit(1)
		}
		if err := orgpwnotify.Register(queueMgr, mailSettingsResolver, mailService, log); err != nil {
			log.Error("failed to register user-password-expiry-notification consumer: %v", err)
			os.Exit(1)
		}

		routes.Init(ctx)

		queueCtx, queueCancel := context.WithCancel(context.Background())
		if err := queueMgr.Start(queueCtx); err != nil {
			log.Error("queue manager start failed: %v", err)
			queueCancel()
			os.Exit(1)
		}
		defer func() {
			queueCancel()
			queueMgr.Stop(10 * time.Second)
		}()

		// Mail orchestrator + retention share the queue context so they shut
		// down cleanly alongside the workers.
		orchestrator.Start(queueCtx)
		mailconsumer.StartRetention(queueCtx, mailStoreInstance, log)

		// OAuth archiver — sweeps per-org oauth.db every 10 minutes,
		// moving expired and consumed token rows into dated archive
		// files. Shares the queue context for clean shutdown.
		oauthArchiver := oauth_archiver.New(orgOAuthDBRegistry, "./data/db", log)
		go oauthArchiver.Run(queueCtx)

		// Password expiry notification detectors — scan once at startup
		// then every 24 hours. Published to the notify queues registered above.
		adminPwDetector := adminpwnotify.NewAdminDetector(manager.Connector.DB, adminRulesStore, queueMgr, log)
		go adminPwDetector.Run(queueCtx)

		orgPwDetector := orgpwnotify.NewOrgDetector(orgUserDBRegistry, orgRulesStore, queueMgr, log)
		go orgPwDetector.Run(queueCtx)

		// IP-guard sweeper — prunes expired bans (DB + in-memory) and
		// stale rate-limit counters every 5 minutes. ctx.IPGuard is set
		// inside routes.Init above; construction there panics on
		// failure, so it is always populated by this point. Shares the
		// queue context for clean shutdown. See
		// plan/ip-abuse-protection/plan.md section 6.
		ipGuardSweeper := ipguard.NewSweeper(ctx.IPGuard, log)
		go ipGuardSweeper.Run(queueCtx)

		// ip-attacks.db archiver — rotates the file out once it crosses
		// the entry-count threshold, every 10 minutes. Separate ticker
		// from the sweeper above since rotation is a much rarer, distinct
		// concern. See plan/ip-attacks-db-archiving/plan.md.
		go dbArchiver.Run(queueCtx)

		// admin_login.db archiver — same rotation mechanism as
		// dbArchiver above, applied to the admin login log instead of
		// ip-attacks.db. See plan/login-audit-log/plan.md Step 2.
		go adminLoginArchiver.Run(queueCtx)

		// geoip.db hot-reload watcher — notices when the separate
		// geoip-updater process (started/stopped via the admin UI, see
		// geoip.Manager) replaces geoip.db and reopens it, without this
		// server ever needing a restart. ctx.GeoIPWatcher is set inside
		// routes.Init above. See plan/geoip-enrichment/plan.md.
		go ctx.GeoIPWatcher.Run(queueCtx)

		// scanWatcher is constructed here rather than earlier, since it
		// needs the same real geoip.Lookup routes.Init just built
		// (ctx.GeoIP) — see plan/geoip-enrichment/plan.md.
		scanWatcher, err := scanwatch.NewWatcher(scanPersist, scanwatch.DefaultThreshold, scanwatch.DefaultWindow, log, ctx.GeoIP)
		if err != nil {
			log.Error("scanwatch: failed to initialize: %v", err)
			os.Exit(1)
		}

		// Port-scan detection: start the log source (a no-op if
		// unavailable — Available()/Detail() report why on the admin
		// Security page), consume its lines into scan episodes, and
		// flush those episodes to disk on its own ticker. All three
		// share the queue context for clean shutdown.
		if err := scanSource.Start(queueCtx); err != nil {
			log.Warning("scanwatch: failed to start log source (%s): %v", scanSource.Name(), err)
		} else {
			go scanWatcher.Consume(scanSource.Lines(), scanwatch.DefaultLogPrefix)
		}
		go scanWatcher.Run(queueCtx)

		serv, config, err := server.StartServer(configPath, log)

		if err != nil {
			log.Error("%v", err)
			os.Exit(1)
		}
		log.Info("Server started successfully and will now run until shutdown signal is received.")
		server.GracefulShutdown(serv, config.PidFile, log)
	case "shutdown":
		config, err := server.LoadConfig(configPath)
		if err != nil {
			fmt.Printf("Could not load config: %v\n", err)
			os.Exit(1)
		}
		pidFile := config.PidFile
		if pidFile == "" {
			pidFile = "./server.pid"
		}
		pid, err := server.ReadPID(pidFile)
		if err != nil {
			fmt.Printf("Could not read PID file: %v\n", err)
			os.Exit(1)
		}
		if pid == os.Getpid() {
			c := make(chan os.Signal, 1)
			c <- os.Interrupt
			return
		} else {
			err = server.SendSIGTERM(pid)
			if err != nil {
				os.Exit(1)
			}
		}
	case "create-admin":
		log, err := initLogger()
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}

		defer log.Close()
		migrationsPath, err := config.ExtractMigrationsToTemp()
		if err != nil {
			fmt.Printf("Migrationen konnten nicht extrahiert werden: %v\n", err)
			os.Exit(1)
		}

		manager, err := dbmanager.NewDatabaseManager("users.db", "./data/db", []string{migrationsPath})
		if err != nil {
			fmt.Printf("DatabaseManager creation failed: %v\n", err)
			os.Exit(1)
		}

		// coco-orm doesn't auto-apply migrations. The `start` path runs
		// dbInstall.Install which calls SyncMigrations; the create-admin
		// CLI bypasses that and would otherwise hit an empty DB on first
		// run after a wipe.
		if err := dbInstall.Install(manager); err != nil {
			fmt.Printf("[ERROR] Failed to apply migrations: %v\n", err)
			os.Exit(1)
		}

		if len(os.Args) < 5 {
			fmt.Printf("[ERROR] Missing arguments. Usage: %s create-admin <username> <email> <password>\n", os.Args[0])
			os.Exit(1)
		}

		username := strings.TrimSpace(os.Args[2])
		email := strings.TrimSpace(os.Args[3])
		password := strings.TrimSpace(os.Args[4])

		if err := dbInstall.AddSuperadminWithArgs(manager, username, email, password); err != nil {
			fmt.Printf("[ERROR] Failed to create superadmin: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Superadmin creation completed.")
		os.Exit(0)
	default:
		fmt.Printf("Unknown action: %s. Use 'start' or 'shutdown'\n", action)
		os.Exit(1)
	}
}

func initLogger() (logger.Logger, error) {
	logFilePath := server.LogFileName("server")
	return logger.NewLogger(logFilePath, "data/logs")
}
