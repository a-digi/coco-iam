// Package oauthserver implements an OAuth 2.0 / OIDC
// authorization server. It's designed to live as a top-level
// package so it can be extracted as a standalone Go library
// in the future.
//
// # Dependency rule
//
// This package and its subpackages may import only:
//   - The Go standard library.
//   - Its own subpackages (pkce/, scope/, tokenid/, entity/).
//   - HTTP-framework-agnostic wrappers for writing responses.
//
// It must NOT import:
//   - src/applications/...
//   - src/organizations/...
//   - src/admin/...
//   - Any coco-iam business-domain package.
//
// Everything coco-iam-specific (per-org user pools, signing
// keys, scope catalog, admin sessions) is injected through the
// interfaces in ports.go. The concrete adapters live in a
// sibling wiring package (src/applications/oauthserverwiring/)
// that stays behind when the library is later extracted.
//
// # Layout
//
//	entity/       pure domain types (Client, AuthorizationCode, …)
//	pkce/         S256 code-challenge verification
//	scope/        scope-string parsing, claim filtering
//	tokenid/      opaque token generation + hashing
//	ports.go      interfaces the handlers depend on
//	authorize.go  AuthorizeHandler
//	token.go      TokenHandler
//	userinfo.go   UserinfoHandler
//	revoke.go     RevokeHandler
//	introspect.go IntrospectHandler
//	discovery.go  OIDC discovery metadata builder
//
// See plan/oauth-provider/plan.md for the architectural
// rationale and the phased rollout that produced this code.
package oauthserver
