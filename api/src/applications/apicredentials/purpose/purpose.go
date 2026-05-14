// Package purpose enumerates the permissions a machine-auth API
// credential can carry. New purposes are added by declaring a new
// constant here; no schema change is needed — the `purposes` JSON
// column holds an opaque list of these strings.
package purpose

// Purpose is the typed form of the strings stored in the credential's
// `purposes` JSON array. Using a named type catches typos at the
// handler boundary (handlers can't pass arbitrary strings to the
// authn function).
type Purpose string

const (
	// SecurityKeyRead lets a credential GET the application's RSA
	// signing keys — both the public and private endpoints under
	// `/a/{org}/{ws}/{app}/security-key`.
	SecurityKeyRead Purpose = "security_key:read"
)

// String returns the raw purpose string for storage / comparison.
func (p Purpose) String() string { return string(p) }
