package login

import (
	"errors"
	"testing"

	"github.com/a-digi/coco-iam/src/applications/oauthproviders/entity"
)

// Pure-function tests for ResolveLogin. Every branch of the
// decision tree is pinned with a dedicated case. Fakes record
// which method was called with which args so we can assert on
// the side-effects (link vs create) without touching a DB.

type fakeLinker struct {
	byIdentity  map[string]string // key "provider|sub" → userID
	byEmail     map[string]string // "email" → userID
	linkCalls   []linkCall
	createCalls []entity.Identity
	createErr   error
	linkErr     error
	findIDErr   error
	findEmailErr error
}

type linkCall struct {
	userID string
	id     entity.Identity
}

func key(p entity.Provider, sub string) string { return string(p) + "|" + sub }

func (f *fakeLinker) FindByIdentity(_ string, p entity.Provider, sub string) (string, bool, error) {
	if f.findIDErr != nil {
		return "", false, f.findIDErr
	}
	if id, ok := f.byIdentity[key(p, sub)]; ok {
		return id, true, nil
	}
	return "", false, nil
}

func (f *fakeLinker) FindByEmail(_, email string) (string, bool, error) {
	if f.findEmailErr != nil {
		return "", false, f.findEmailErr
	}
	if id, ok := f.byEmail[email]; ok {
		return id, true, nil
	}
	return "", false, nil
}

func (f *fakeLinker) CreateUserFromIdentity(_ string, id entity.Identity) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.createCalls = append(f.createCalls, id)
	newID := "new-user-" + id.Sub
	if f.byIdentity == nil {
		f.byIdentity = map[string]string{}
	}
	f.byIdentity[key(id.Provider, id.Sub)] = newID
	return newID, nil
}

func (f *fakeLinker) LinkIdentity(_, userID string, id entity.Identity) error {
	if f.linkErr != nil {
		return f.linkErr
	}
	f.linkCalls = append(f.linkCalls, linkCall{userID, id})
	if f.byIdentity == nil {
		f.byIdentity = map[string]string{}
	}
	f.byIdentity[key(id.Provider, id.Sub)] = userID
	return nil
}

func cfg(allowLogin, allowReg bool) entity.ProviderConfig {
	return entity.ProviderConfig{
		Provider:          entity.ProviderGoogle,
		AllowLogin:        allowLogin,
		AllowRegistration: allowReg,
		IsActive:          true,
	}
}

func appSettings(allowReg bool) AppSettings {
	return AppSettings{
		ApplicationID:     "app-1",
		OrganizationID:    "org-1",
		AllowRegistration: allowReg,
	}
}

// ------- tests ------------------------------------------------------

func TestResolve_ExistingIdentityHitReturnsLogin(t *testing.T) {
	lk := &fakeLinker{byIdentity: map[string]string{key(entity.ProviderGoogle, "sub-1"): "user-1"}}
	id := entity.Identity{Provider: entity.ProviderGoogle, Sub: "sub-1"}
	uid, mode, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if err != nil {
		t.Fatalf("ResolveLogin: %v", err)
	}
	if mode != ModeLogin {
		t.Errorf("mode: got %s want login", mode)
	}
	if uid != "user-1" {
		t.Errorf("uid: got %q", uid)
	}
	if len(lk.createCalls) != 0 || len(lk.linkCalls) != 0 {
		t.Errorf("must not touch side-effect paths: creates=%v links=%v",
			lk.createCalls, lk.linkCalls)
	}
}

func TestResolve_VerifiedEmailMatchLinksNewProvider(t *testing.T) {
	lk := &fakeLinker{
		byEmail: map[string]string{"alice@example.com": "user-A"},
	}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-1",
		Email: "alice@example.com", EmailVerified: true,
	}
	uid, mode, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if err != nil {
		t.Fatalf("ResolveLogin: %v", err)
	}
	if mode != ModeLinked {
		t.Errorf("mode: got %s want linked", mode)
	}
	if uid != "user-A" {
		t.Errorf("uid: %q", uid)
	}
	if len(lk.linkCalls) != 1 || lk.linkCalls[0].userID != "user-A" {
		t.Errorf("link call missing or wrong user: %v", lk.linkCalls)
	}
}

func TestResolve_NewUserRegisteredWhenAllowed(t *testing.T) {
	lk := &fakeLinker{}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-new",
		Email: "bob@example.com", EmailVerified: true,
	}
	uid, mode, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if err != nil {
		t.Fatalf("ResolveLogin: %v", err)
	}
	if mode != ModeRegistered {
		t.Errorf("mode: got %s want registered", mode)
	}
	if uid != "new-user-sub-new" {
		t.Errorf("uid: %q", uid)
	}
	if len(lk.createCalls) != 1 {
		t.Errorf("want 1 create call, got %d", len(lk.createCalls))
	}
}

func TestResolve_RegistrationClosedRejectsNewUser(t *testing.T) {
	// Email-match miss + registration closed → ErrRegistrationClosed.
	lk := &fakeLinker{}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-new",
		Email: "nobody@example.com", EmailVerified: true,
	}
	_, _, err := ResolveLogin(id, cfg(true, false), appSettings(true), lk)
	if !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("want ErrRegistrationClosed, got %v", err)
	}
	if len(lk.createCalls) != 0 {
		t.Error("must not create user when registration closed")
	}
}

func TestResolve_AppLevelRegistrationClosedOverridesProvider(t *testing.T) {
	// Provider says OK but the app-level flag says no — no
	// new users. Intent of the two flags: either can veto.
	lk := &fakeLinker{}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-new",
		Email: "nobody@example.com", EmailVerified: true,
	}
	_, _, err := ResolveLogin(id, cfg(true, true), appSettings(false), lk)
	if !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("want ErrRegistrationClosed, got %v", err)
	}
}

func TestResolve_UnverifiedEmailRejectedWhenRegistrationClosed(t *testing.T) {
	lk := &fakeLinker{}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "sub-x",
		Email: "private@example.com", EmailVerified: false,
	}
	_, _, err := ResolveLogin(id, cfg(true, false), appSettings(true), lk)
	if !errors.Is(err, ErrUntrustedEmail) {
		t.Fatalf("want ErrUntrustedEmail, got %v", err)
	}
}

func TestResolve_UnverifiedEmailStillRegistersWhenAllowed(t *testing.T) {
	// GitHub private-email path: no email or unverified. When
	// registration is open we create a fresh account and move
	// on — email-match linking is skipped.
	lk := &fakeLinker{
		byEmail: map[string]string{"should-not-hit@example.com": "user-Z"},
	}
	id := entity.Identity{
		Provider: entity.ProviderGitHub, Sub: "gh-1",
		Email: "should-not-hit@example.com", EmailVerified: false,
	}
	uid, mode, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if err != nil {
		t.Fatalf("ResolveLogin: %v", err)
	}
	if mode != ModeRegistered {
		t.Errorf("mode: got %s want registered", mode)
	}
	if uid == "user-Z" {
		t.Errorf("must not match by email when email is unverified")
	}
	if len(lk.linkCalls) != 0 {
		t.Error("must not call LinkIdentity on unverified email")
	}
}

func TestResolve_ProviderLoginDisabledIsRejected(t *testing.T) {
	lk := &fakeLinker{}
	id := entity.Identity{Provider: entity.ProviderGoogle, Sub: "any"}
	_, _, err := ResolveLogin(id, cfg(false, true), appSettings(true), lk)
	if !errors.Is(err, ErrLoginDisabled) {
		t.Fatalf("want ErrLoginDisabled, got %v", err)
	}
}

func TestResolve_ProviderInactiveIsRejected(t *testing.T) {
	lk := &fakeLinker{}
	c := cfg(true, true)
	c.IsActive = false
	id := entity.Identity{Provider: entity.ProviderGoogle, Sub: "any"}
	_, _, err := ResolveLogin(id, c, appSettings(true), lk)
	if !errors.Is(err, ErrProviderInactive) {
		t.Fatalf("want ErrProviderInactive, got %v", err)
	}
}

func TestResolve_LinkerFindErrorPropagates(t *testing.T) {
	boom := errors.New("db down")
	lk := &fakeLinker{findIDErr: boom}
	id := entity.Identity{Provider: entity.ProviderGoogle, Sub: "s"}
	_, _, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if !errors.Is(err, boom) {
		t.Fatalf("want propagated error, got %v", err)
	}
}

func TestResolve_CreateErrorPropagates(t *testing.T) {
	boom := errors.New("insert failed")
	lk := &fakeLinker{createErr: boom}
	id := entity.Identity{
		Provider: entity.ProviderGoogle, Sub: "s",
		Email: "a@b", EmailVerified: true,
	}
	_, _, err := ResolveLogin(id, cfg(true, true), appSettings(true), lk)
	if !errors.Is(err, boom) {
		t.Fatalf("want propagated create error, got %v", err)
	}
}
