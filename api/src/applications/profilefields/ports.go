package profilefields

import "crypto/rsa"

// SlugResolver turns (orgSlug, wsSlug, appSlug) into the
// application UUID + its parent organisation UUID.
type SlugResolver interface {
	ResolveSlugs(orgSlug, wsSlug, appSlug string) (appID, orgID string, err error)
}

// KeyLoader returns the RSA public key identified by (appID, kid).
type KeyLoader interface {
	LoadPublicKey(appID, kid string) (*rsa.PublicKey, error)
}

// UserOrgReader returns the organisation id the given user belongs to.
// Implementations must return userprofile.ErrUserNotFound when the user
// does not exist so AuthenticateUser maps it to a 401 rather than a 500.
type UserOrgReader interface {
	UserOrg(userID string) (orgID string, err error)
}

// FieldSchemaReader loads the active profile field definitions for an org.
type FieldSchemaReader interface {
	ActiveFields(orgID string) ([]ProfileFieldSchema, error)
}
