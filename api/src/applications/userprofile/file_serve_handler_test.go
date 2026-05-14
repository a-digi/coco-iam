package userprofile

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-digi/coco-server/server/request"
)

func serveServe(h *FileServeHandler, path, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, req, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

func TestFileServe_NoAssetReturns404(t *testing.T) {
	key := testKey(t)
	h := &FileServeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Store: &fakeFileStore{},
		Files: &fakeFileRepo{findByFieldErr: ErrAssetNotFound},
	}
	rec := serveServe(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestFileServe_HappyPathReturnsBytesAndHeaders(t *testing.T) {
	key := testKey(t)
	meta := &FileMeta{
		AssetID:  "asset-1",
		Ext:      "png",
		Filename: "passport.png",
		MimeType: "image/png",
	}
	store := &fakeFileStore{openData: []byte("\x89PNG\r\n\x1a\n")}
	h := &FileServeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Store: store,
		Files: &fakeFileRepo{findByFieldMeta: meta},
	}
	rec := serveServe(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if rec.Body.String() != string(store.openData) {
		t.Errorf("body mismatch: got %q", rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("Content-Type: want image/png, got %q", rec.Header().Get("Content-Type"))
	}
	cd := rec.Header().Get("Content-Disposition")
	if !strings.Contains(cd, `filename="passport.png"`) || !strings.HasPrefix(cd, "inline;") {
		t.Errorf("Content-Disposition: got %q", cd)
	}
}

func TestFileServe_RepoErrorReturns500(t *testing.T) {
	key := testKey(t)
	h := &FileServeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Store: &fakeFileStore{},
		Files: &fakeFileRepo{findByFieldErr: errors.New("db explosion")},
	}
	rec := serveServe(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rec.Code)
	}
}

func TestFileServe_StoreOpenMissingReturns404(t *testing.T) {
	// DB row says the asset exists but the file is gone from disk —
	// rare orphan case. Return 404, not 500; the client can re-upload.
	key := testKey(t)
	meta := &FileMeta{AssetID: "asset-1", Ext: "png", Filename: "p.png", MimeType: "image/png"}
	h := &FileServeHandler{
		Slugs: &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:  &fakeKeyLoader{pub: &key.PublicKey},
		Users: &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Store: &fakeFileStore{openErr: ErrAssetNotFound},
		Files: &fakeFileRepo{findByFieldMeta: meta},
	}
	rec := serveServe(h, "/a/acme/prod/web/profile/me/files/passport",
		signBearer(t, key, "user-1"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404 (orphan), got %d", rec.Code)
	}
}
