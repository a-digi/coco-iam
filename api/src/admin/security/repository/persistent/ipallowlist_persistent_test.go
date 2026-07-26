package persistent

import (
	"testing"

	security_query "github.com/a-digi/coco-iam/src/admin/security/repository/query"
)

func TestInsertAllowlistEntry_CreatesThenUpdatesOnConflict(t *testing.T) {
	db := freshDB(t)
	persist := NewIPAllowlistPersistentRepo(db)
	query := security_query.NewIPAllowlistQueryRepo(db)

	if err := persist.InsertAllowlistEntry("203.0.113.7", "office egress", "admin-1"); err != nil {
		t.Fatalf("InsertAllowlistEntry() error = %v", err)
	}
	e, err := query.FindAllowlistEntry("203.0.113.7")
	if err != nil {
		t.Fatalf("FindAllowlistEntry() error = %v", err)
	}
	if e.Note != "office egress" || e.CreatedBy != "admin-1" {
		t.Fatalf("entry = %+v, unexpected", e)
	}

	if err := persist.InsertAllowlistEntry("203.0.113.7", "updated note", "admin-2"); err != nil {
		t.Fatalf("InsertAllowlistEntry() on conflict error = %v", err)
	}
	e, err = query.FindAllowlistEntry("203.0.113.7")
	if err != nil {
		t.Fatalf("FindAllowlistEntry() after update error = %v", err)
	}
	if e.Note != "updated note" || e.CreatedBy != "admin-2" {
		t.Fatalf("entry after conflict update = %+v, want note=updated note createdBy=admin-2", e)
	}
}

func TestDeleteAllowlistEntry_ErrorsWhenNoMatchingRow(t *testing.T) {
	db := freshDB(t)
	persist := NewIPAllowlistPersistentRepo(db)

	if err := persist.DeleteAllowlistEntry("203.0.113.7"); err == nil {
		t.Fatal("expected an error removing an IP that was never allowlisted")
	}
}

func TestDeleteAllowlistEntry_RemovesExistingRow(t *testing.T) {
	db := freshDB(t)
	persist := NewIPAllowlistPersistentRepo(db)
	query := security_query.NewIPAllowlistQueryRepo(db)

	if err := persist.InsertAllowlistEntry("203.0.113.7", "", "admin-1"); err != nil {
		t.Fatalf("InsertAllowlistEntry() error = %v", err)
	}
	if err := persist.DeleteAllowlistEntry("203.0.113.7"); err != nil {
		t.Fatalf("DeleteAllowlistEntry() error = %v", err)
	}
	if _, err := query.FindAllowlistEntry("203.0.113.7"); err != security_query.ErrAllowlistEntryNotFound {
		t.Fatalf("FindAllowlistEntry() after delete error = %v, want ErrAllowlistEntryNotFound", err)
	}
}

func TestListAllowlist_ReturnsAllEntries(t *testing.T) {
	db := freshDB(t)
	persist := NewIPAllowlistPersistentRepo(db)
	query := security_query.NewIPAllowlistQueryRepo(db)

	if err := persist.InsertAllowlistEntry("1.1.1.1", "a", "admin-1"); err != nil {
		t.Fatalf("InsertAllowlistEntry() error = %v", err)
	}
	if err := persist.InsertAllowlistEntry("2.2.2.2", "b", "admin-1"); err != nil {
		t.Fatalf("InsertAllowlistEntry() error = %v", err)
	}

	all, err := query.ListAllowlist()
	if err != nil {
		t.Fatalf("ListAllowlist() error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAllowlist() returned %d entries, want 2", len(all))
	}
}
