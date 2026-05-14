package userprofile

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/a-digi/coco-server/server/request"
)

func serveDelete(h *FileDeleteHandler, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, req, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

func TestFileDelete_NoCurrentUploadIsIdempotent(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	repo := &fakeFileRepo{findByFieldErr: ErrAssetNotFound}
	writer := &countingWriter{}
	h := &FileDeleteHandler{
		Slugs:  &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:   &fakeKeyLoader{pub: &key.PublicKey},
		Users:  &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields: &fakeFieldConfigReader{},
		Store:  store,
		Files:  repo,
		Writer: writer,
	}
	rec := serveDelete(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("idempotent delete want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.deleteCalls) != 0 {
		t.Errorf("no prior asset → FileStore.Delete must not be called")
	}
}

func TestFileDelete_ExistingUploadRemovesBytesAndRow(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	repo := &fakeFileRepo{findByFieldMeta: &FileMeta{AssetID: "asset-1", Ext: "png"}}
	writer := &countingWriter{}
	h := &FileDeleteHandler{
		Slugs:  &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:   &fakeKeyLoader{pub: &key.PublicKey},
		Users:  &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields: &fakeFieldConfigReader{},
		Store:  store,
		Files:  repo,
		Writer: writer,
	}
	rec := serveDelete(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.deleteCalls) != 1 || store.deleteCalls[0].assetID != "asset-1" {
		t.Errorf("want FileStore.Delete(asset-1); got %+v", store.deleteCalls)
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != "asset-1" {
		t.Errorf("want FileRepo.DeleteByAssetID(asset-1); got %+v", repo.deleteCalls)
	}
	if len(writer.calls) != 1 || writer.calls[0].fieldName != "passport" || writer.calls[0].value != nil {
		t.Errorf("writer should clear passport; got %+v", writer.calls)
	}
}

func TestFileDelete_StoreDeleteErrorLeavesRepoRowIntact(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{deleteErr: errors.New("disk gremlin")}
	repo := &fakeFileRepo{findByFieldMeta: &FileMeta{AssetID: "asset-1", Ext: "png"}}
	writer := &countingWriter{}
	h := &FileDeleteHandler{
		Slugs:  &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:   &fakeKeyLoader{pub: &key.PublicKey},
		Users:  &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields: &fakeFieldConfigReader{},
		Store:  store,
		Files:  repo,
		Writer: writer,
	}
	rec := serveDelete(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	// Row NOT removed → admin can retry the delete and the disk
	// unlink will run again.
	if len(repo.deleteCalls) != 0 {
		t.Errorf("repo row must be preserved on store error; got %+v", repo.deleteCalls)
	}
	if len(writer.calls) != 0 {
		t.Errorf("writer must not clear profile field on store error; got %+v", writer.calls)
	}
}
