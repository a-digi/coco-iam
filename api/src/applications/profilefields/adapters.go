package profilefields

// This file holds the production adapters that connect the handler's
// narrow ports to the concrete services wired at startup. Each adapter
// is a thin wrapper — no business logic, no branching beyond error
// mapping — so they don't carry their own tests.

import (
	"crypto/rsa"
	"sort"

	"github.com/a-digi/coco-iam/src/applications/keys"
	"github.com/a-digi/coco-iam/src/applications/loginpage"
	userprofile "github.com/a-digi/coco-iam/src/applications/userprofile"
	profile "github.com/a-digi/coco-iam/src/organizations/profile"
	profile_dbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	users_dbregistry "github.com/a-digi/coco-iam/src/organizations/users/dbregistry"
	"github.com/a-digi/coco-iam/src/orgrouter"
)

// -- SlugResolver ------------------------------------------------------

// NewLoginpageSlugResolver wraps *loginpage.Service as a SlugResolver.
func NewLoginpageSlugResolver(svc *loginpage.Service) SlugResolver {
	return &loginpageSlugResolver{svc: svc}
}

type loginpageSlugResolver struct {
	svc *loginpage.Service
}

func (r *loginpageSlugResolver) ResolveSlugs(orgSlug, wsSlug, appSlug string) (appID, orgID string, err error) {
	info, err := r.svc.Store.FindBySlugs(orgSlug, wsSlug, appSlug)
	if err != nil {
		return "", "", err
	}
	return info.ID, info.OrganizationID, nil
}

// -- KeyLoader ---------------------------------------------------------

// NewKeysServiceKeyLoader wraps *keys.Service as a KeyLoader.
func NewKeysServiceKeyLoader(svc *keys.Service) KeyLoader {
	return &keysServiceKeyLoader{svc: svc}
}

type keysServiceKeyLoader struct {
	svc *keys.Service
}

func (l *keysServiceKeyLoader) LoadPublicKey(appID, kid string) (*rsa.PublicKey, error) {
	return l.svc.LoadVerifiablePublicKey(appID, kid)
}

// -- UserOrgReader -----------------------------------------------------

// NewOrgRegistryUserOrgReader returns a UserOrgReader that resolves a
// user's org by scanning the per-org DBs. Returns userprofile.ErrUserNotFound
// on miss so AuthenticateUser maps the failure to 401 rather than 500.
func NewOrgRegistryUserOrgReader(reg *users_dbregistry.OrgUserDBRegistry) UserOrgReader {
	return &orgRegistryUserOrgReader{reg: reg}
}

type orgRegistryUserOrgReader struct {
	reg *users_dbregistry.OrgUserDBRegistry
}

func (r *orgRegistryUserOrgReader) UserOrg(userID string) (string, error) {
	_, orgID, err := orgrouter.OrgDBFor(r.reg, userID)
	if err != nil {
		return "", userprofile.ErrUserNotFound
	}
	return orgID, nil
}

// -- FieldSchemaReader -------------------------------------------------

// NewOrgFieldSchemaReader wraps the per-org profile DB registry.
// Each call opens (or reuses) the per-org profiles.db, loads active
// fields via the profile repository, and maps them to ProfileFieldSchema
// (omitting AcceptMime/MaxBytes). Results are sorted by order_index ASC,
// name ASC.
func NewOrgFieldSchemaReader(reg *profile_dbregistry.OrgDBRegistry) FieldSchemaReader {
	return &orgFieldSchemaReader{reg: reg}
}

type orgFieldSchemaReader struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (r *orgFieldSchemaReader) ActiveFields(orgID string) ([]ProfileFieldSchema, error) {
	db, err := r.reg.For(orgID)
	if err != nil {
		return nil, err
	}
	repo := profile.NewRepository(db)
	fields, err := repo.ListFields(true)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].OrderIndex != fields[j].OrderIndex {
			return fields[i].OrderIndex < fields[j].OrderIndex
		}
		return fields[i].Name < fields[j].Name
	})
	out := make([]ProfileFieldSchema, 0, len(fields))
	for _, f := range fields {
		opts := f.Options
		if opts == nil {
			opts = []string{}
		}
		out = append(out, ProfileFieldSchema{
			ID:          f.ID,
			Name:        f.Name,
			Label:       f.Label,
			Description: f.Description,
			DataType:    f.DataType,
			IsRequired:  f.IsRequired,
			MinValue:    f.MinValue,
			MaxValue:    f.MaxValue,
			Options:     opts,
			Regex:       f.Regex,
			OrderIndex:  f.OrderIndex,
		})
	}
	return out, nil
}

// -- FullFieldLoader ---------------------------------------------------

// NewOrgFullFieldLoader returns a FullFieldLoader backed by the per-org
// profile DB registry. Returns full entity.ProfileField including
// AcceptMime/MaxBytes for file-upload validation.
func NewOrgFullFieldLoader(reg *profile_dbregistry.OrgDBRegistry) FullFieldLoader {
	return &orgFullFieldLoader{reg: reg}
}

type orgFullFieldLoader struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (l *orgFullFieldLoader) ActiveFieldsFull(orgID string) ([]profile_entity.ProfileField, error) {
	db, err := l.reg.For(orgID)
	if err != nil {
		return nil, err
	}
	return profile.NewRepository(db).ListFields(true)
}

// -- ProfileReader (for PUT handler) -----------------------------------

// NewOrgRegistryProfileReader delegates to userprofile.NewOrgRegistryProfileReader
// so the SQL lives in one place. The two ProfileReader interfaces are
// structurally identical, so the concrete type satisfies both.
func NewOrgRegistryProfileReader(reg *profile_dbregistry.OrgDBRegistry) ProfileReader {
	return userprofile.NewOrgRegistryProfileReader(reg)
}

// -- ProfileSaver ------------------------------------------------------

// NewOrgProfileSaver returns a ProfileSaver backed by the per-org profile
// DB registry. SaveProfile calls UpsertUserProfile which is an INSERT …
// ON CONFLICT UPDATE — safe for both create and replace.
func NewOrgProfileSaver(reg *profile_dbregistry.OrgDBRegistry) ProfileSaver {
	return &orgProfileSaver{reg: reg}
}

type orgProfileSaver struct {
	reg *profile_dbregistry.OrgDBRegistry
}

func (s *orgProfileSaver) SaveProfile(orgID, userID string, data map[string]interface{}) error {
	db, err := s.reg.For(orgID)
	if err != nil {
		return err
	}
	return profile.NewRepository(db).UpsertUserProfile(userID, data)
}
