// Package recoverypage implements per-application password recovery
// (request + reset). The flow reuses the existing recovery.Service
// token machinery. Only users with an active application_user_acl
// row for the calling application can recover; unknown emails get
// the same 200 OK to avoid enumeration.
//
// Custom HTML templates were removed (plan/application-login-templates).
// The public recovery pages are rendered by the React app driven by
// the app's login-template settings.
package recoverypage

const ContextBagKeyService = "applications.recoverypage.Service"
