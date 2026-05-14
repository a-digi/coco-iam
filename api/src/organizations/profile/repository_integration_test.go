package profile

import (
	"path/filepath"
	"runtime"
	"testing"

	profiledbregistry "github.com/a-digi/coco-iam/src/organizations/profile/dbregistry"
	"github.com/a-digi/coco-iam/src/organizations/profile/entity"
	_ "github.com/mattn/go-sqlite3"
)

// orgMigrationsPath returns the absolute path to the per-org profile
// migration files regardless of where `go test` is invoked from.
func orgMigrationsPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "../../../config/db/org_migrations")
}

// newProfileRepo provisions a fresh per-org profiles DB for the given
// orgID in t.TempDir() with real migrations applied, then wraps it in
// a Repository.
func newProfileRepo(t *testing.T, orgID string) *Repository {
	t.Helper()
	reg := profiledbregistry.New(t.TempDir(), orgMigrationsPath())
	mgr, err := reg.For(orgID)
	if err != nil {
		t.Fatalf("provision profiles db for %q: %v", orgID, err)
	}
	return NewRepository(mgr)
}

// --- profile field tests ---

func TestCreateField_PersistsToOrgProfilesDB(t *testing.T) {
	repo := newProfileRepo(t, "org-1")

	f := &entity.ProfileField{
		Name:      "bio",
		Label:     "Biography",
		DataType:  "text",
		IsRequired: false,
		Options:   []string{},
	}
	if err := repo.CreateField(f); err != nil {
		t.Fatalf("CreateField: %v", err)
	}
	if f.ID == "" {
		t.Fatal("expected non-empty ID after CreateField")
	}

	got, err := repo.GetField(f.ID)
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if got == nil {
		t.Fatal("GetField returned nil")
	}
	if got.Name != "bio" {
		t.Errorf("name: want bio, got %q", got.Name)
	}
	if got.Label != "Biography" {
		t.Errorf("label: want Biography, got %q", got.Label)
	}
	if got.DataType != "text" {
		t.Errorf("data_type: want text, got %q", got.DataType)
	}
	if got.IsRequired {
		t.Error("is_required: want false")
	}
}

func TestListFields_OrderedByOrderIndex(t *testing.T) {
	repo := newProfileRepo(t, "org-2")

	names := []struct{ name string; order int }{
		{"field_c", 3},
		{"field_a", 1},
		{"field_b", 2},
	}
	for _, n := range names {
		f := &entity.ProfileField{
			Name:       n.name,
			Label:      n.name,
			DataType:   "text",
			OrderIndex: n.order,
		}
		if err := repo.CreateField(f); err != nil {
			t.Fatalf("CreateField %q: %v", n.name, err)
		}
	}

	fields, err := repo.ListFields(false)
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("want 3 fields, got %d", len(fields))
	}
	want := []string{"field_a", "field_b", "field_c"}
	for i, f := range fields {
		if f.Name != want[i] {
			t.Errorf("fields[%d]: want %q, got %q", i, want[i], f.Name)
		}
	}
}

func TestListFields_ActiveOnlyFilter(t *testing.T) {
	repo := newProfileRepo(t, "org-3")

	active := &entity.ProfileField{Name: "visible", Label: "Visible", DataType: "text"}
	hidden := &entity.ProfileField{Name: "hidden", Label: "Hidden", DataType: "text"}
	if err := repo.CreateField(active); err != nil {
		t.Fatalf("CreateField active: %v", err)
	}
	if err := repo.CreateField(hidden); err != nil {
		t.Fatalf("CreateField hidden: %v", err)
	}
	if err := repo.SoftDeleteField(hidden.ID); err != nil {
		t.Fatalf("SoftDeleteField: %v", err)
	}

	all, err := repo.ListFields(false)
	if err != nil {
		t.Fatalf("ListFields(false): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("ListFields(false): want 2, got %d", len(all))
	}

	activeOnly, err := repo.ListFields(true)
	if err != nil {
		t.Fatalf("ListFields(true): %v", err)
	}
	if len(activeOnly) != 1 {
		t.Errorf("ListFields(true): want 1, got %d", len(activeOnly))
	}
	if activeOnly[0].Name != "visible" {
		t.Errorf("ListFields(true)[0]: want visible, got %q", activeOnly[0].Name)
	}
}

func TestUpdateField_ChangesLabel(t *testing.T) {
	repo := newProfileRepo(t, "org-4")

	f := &entity.ProfileField{Name: "phone", Label: "Phone", DataType: "text"}
	if err := repo.CreateField(f); err != nil {
		t.Fatalf("CreateField: %v", err)
	}

	f.Label = "Mobile Phone"
	if err := repo.UpdateField(f); err != nil {
		t.Fatalf("UpdateField: %v", err)
	}

	got, err := repo.GetField(f.ID)
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if got.Label != "Mobile Phone" {
		t.Errorf("label after update: want Mobile Phone, got %q", got.Label)
	}
}

func TestReorderFields_SetsOrderIndex(t *testing.T) {
	repo := newProfileRepo(t, "org-5")

	var ids []string
	for _, name := range []string{"a", "b", "c"} {
		f := &entity.ProfileField{Name: name, Label: name, DataType: "text"}
		if err := repo.CreateField(f); err != nil {
			t.Fatalf("CreateField %q: %v", name, err)
		}
		ids = append(ids, f.ID)
	}

	// Reorder: c, a, b
	newOrder := []string{ids[2], ids[0], ids[1]}
	if err := repo.ReorderFields(newOrder); err != nil {
		t.Fatalf("ReorderFields: %v", err)
	}

	fields, err := repo.ListFields(false)
	if err != nil {
		t.Fatalf("ListFields: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("want 3 fields, got %d", len(fields))
	}
	wantNames := []string{"c", "a", "b"}
	for i, f := range fields {
		if f.Name != wantNames[i] {
			t.Errorf("fields[%d]: want %q, got %q", i, wantNames[i], f.Name)
		}
	}
}

func TestCreateField_DuplicateName_Fails(t *testing.T) {
	repo := newProfileRepo(t, "org-6")

	f1 := &entity.ProfileField{Name: "dupe", Label: "First", DataType: "text"}
	if err := repo.CreateField(f1); err != nil {
		t.Fatalf("first CreateField: %v", err)
	}

	f2 := &entity.ProfileField{Name: "dupe", Label: "Second", DataType: "text"}
	if err := repo.CreateField(f2); err == nil {
		t.Error("expected UNIQUE constraint error for duplicate name, got nil")
	}
}

// --- user profile tests ---

func TestUpsertUserProfile_CreatesRow(t *testing.T) {
	repo := newProfileRepo(t, "org-7")

	data := map[string]interface{}{
		"city":    "Berlin",
		"country": "Germany",
	}
	if err := repo.UpsertUserProfile("user-1", data); err != nil {
		t.Fatalf("UpsertUserProfile: %v", err)
	}

	got, err := repo.GetUserProfile("user-1")
	if err != nil {
		t.Fatalf("GetUserProfile: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserProfile returned nil")
	}
	if got.ProfileData["city"] != "Berlin" {
		t.Errorf("city: want Berlin, got %v", got.ProfileData["city"])
	}
	if got.ProfileData["country"] != "Germany" {
		t.Errorf("country: want Germany, got %v", got.ProfileData["country"])
	}
}

func TestUpsertUserProfile_UpdatesExisting(t *testing.T) {
	repo := newProfileRepo(t, "org-8")

	if err := repo.UpsertUserProfile("user-1", map[string]interface{}{"role": "viewer"}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertUserProfile("user-1", map[string]interface{}{"role": "editor"}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.GetUserProfile("user-1")
	if err != nil {
		t.Fatalf("GetUserProfile: %v", err)
	}
	if got.ProfileData["role"] != "editor" {
		t.Errorf("role after update: want editor, got %v", got.ProfileData["role"])
	}
}

func TestGetUserProfile_MissingUser_ReturnsNil(t *testing.T) {
	repo := newProfileRepo(t, "org-9")

	got, err := repo.GetUserProfile("no-such-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("want nil for missing user, got %+v", got)
	}
}

func TestUpsertUserProfile_JSONRoundTrip(t *testing.T) {
	repo := newProfileRepo(t, "org-10")

	data := map[string]interface{}{
		"age":  float64(30), // JSON numbers unmarshal as float64
		"tags": []interface{}{"go", "sqlite"},
	}
	if err := repo.UpsertUserProfile("user-2", data); err != nil {
		t.Fatalf("UpsertUserProfile: %v", err)
	}

	got, err := repo.GetUserProfile("user-2")
	if err != nil {
		t.Fatalf("GetUserProfile: %v", err)
	}
	if got.ProfileData["age"] != float64(30) {
		t.Errorf("age: want 30, got %v (%T)", got.ProfileData["age"], got.ProfileData["age"])
	}
	tags, ok := got.ProfileData["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("tags: want [go sqlite], got %v", got.ProfileData["tags"])
	}
}

// --- isolation tests ---

func TestProfilesIsolatedBetweenOrgs(t *testing.T) {
	// Both registries must share the same baseDir so they resolve to the
	// same on-disk folder structure, but we give each a fresh TempDir to
	// guarantee isolation.
	repoA := newProfileRepo(t, "org-a")
	repoB := newProfileRepo(t, "org-b")

	f := &entity.ProfileField{Name: "department", Label: "Department", DataType: "text"}
	if err := repoA.CreateField(f); err != nil {
		t.Fatalf("CreateField in org-a: %v", err)
	}

	fields, err := repoB.ListFields(false)
	if err != nil {
		t.Fatalf("ListFields in org-b: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("org-b should have no fields, got %d", len(fields))
	}
}

func TestUserProfilesIsolatedBetweenOrgs(t *testing.T) {
	repoA := newProfileRepo(t, "org-c")
	repoB := newProfileRepo(t, "org-d")

	if err := repoA.UpsertUserProfile("user-shared-id", map[string]interface{}{"secret": "org-a-data"}); err != nil {
		t.Fatalf("UpsertUserProfile in org-a: %v", err)
	}

	got, err := repoB.GetUserProfile("user-shared-id")
	if err != nil {
		t.Fatalf("GetUserProfile in org-b: %v", err)
	}
	if got != nil {
		t.Errorf("org-b should not see org-a user profile, got %+v", got)
	}
}
