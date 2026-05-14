package userprofile

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	profile_entity "github.com/a-digi/coco-iam/src/organizations/profile/entity"
	"github.com/a-digi/coco-server/server/media"
	"github.com/a-digi/coco-server/server/request"
)

// ------- fakes for the new ports -----------------------------------

type fakeFieldConfigReader struct {
	field *profile_entity.ProfileField
	err   error
}

func (f *fakeFieldConfigReader) FieldByName(_, _ string) (*profile_entity.ProfileField, error) {
	return f.field, f.err
}

type fakeScanner struct {
	mime, ext string
	err       error
}

func (f *fakeScanner) DetectAndValidate(_ []byte, _ string) (string, string, error) {
	return f.mime, f.ext, f.err
}

type fakeFileStore struct {
	saveCalls   []saveCall
	deleteCalls []deleteCall
	openData    []byte
	openErr     error
	saveErr     error
	deleteErr   error
}

type saveCall struct {
	orgID, userID, assetID, ext string
	data                        []byte
}

type deleteCall struct {
	orgID, userID, assetID, ext string
}

func (s *fakeFileStore) Save(orgID, userID, assetID, ext string, data []byte) error {
	s.saveCalls = append(s.saveCalls, saveCall{orgID, userID, assetID, ext, data})
	return s.saveErr
}

func (s *fakeFileStore) Open(_, _, _, _ string) ([]byte, error) {
	return s.openData, s.openErr
}

func (s *fakeFileStore) Delete(orgID, userID, assetID, ext string) error {
	s.deleteCalls = append(s.deleteCalls, deleteCall{orgID, userID, assetID, ext})
	return s.deleteErr
}

type fakeFileRepo struct {
	insertErr        error
	findByFieldMeta  *FileMeta
	findByFieldErr   error
	findByIDMeta     *FileMeta
	findByIDErr      error
	deleteCalls      []string
	deleteErr        error
	insertedMetas    []FileMeta
	insertedAssetIDs []string
}

func (r *fakeFileRepo) Insert(_ string, meta FileMeta) (string, error) {
	r.insertedMetas = append(r.insertedMetas, meta)
	id := meta.AssetID
	if id == "" {
		id = "minted"
	}
	r.insertedAssetIDs = append(r.insertedAssetIDs, id)
	return id, r.insertErr
}

func (r *fakeFileRepo) FindByAssetID(_, _, _ string) (*FileMeta, error) {
	return r.findByIDMeta, r.findByIDErr
}

func (r *fakeFileRepo) FindByField(_, _, _ string) (*FileMeta, error) {
	return r.findByFieldMeta, r.findByFieldErr
}

func (r *fakeFileRepo) DeleteByAssetID(_, _, assetID string) error {
	r.deleteCalls = append(r.deleteCalls, assetID)
	return r.deleteErr
}

// buildUpload wraps the given bytes as a multipart payload.
func buildUpload(t *testing.T, filename string, data []byte) (*http.Request, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(fw, bytes.NewReader(data)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost,
		"/a/acme/prod/web/profile/me/files/passport", body)
	return req, mw.FormDataContentType()
}

func serveUpload(t *testing.T, h *FileUploadHandler, req *http.Request, contentType, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("Content-Type", contentType)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	reqCtx := request.NewContext(rec, req, nil)
	h.ServeHTTP(reqCtx)
	return rec
}

// a minimal file-type profile field helper for these tests.
func fileField(mimeWhitelist string, maxBytes int) *profile_entity.ProfileField {
	return &profile_entity.ProfileField{
		Name:       "passport",
		DataType:   profile_entity.DataTypeFile,
		IsActive:   true,
		AcceptMime: mimeWhitelist,
		MaxBytes:   maxBytes,
	}
}

// ------- tests ------------------------------------------------------

func TestFileUpload_MissingFileFieldReturns400(t *testing.T) {
	key := testKey(t)
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("", 0)},
		Scanner: &fakeScanner{},
		Store:   &fakeFileStore{},
		Files:   &fakeFileRepo{},
		Writer:  &countingWriter{},
	}
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/a/acme/prod/web/profile/me/files/passport", body)
	rec := serveUpload(t, h, req, mw.FormDataContentType(), signBearer(t, key, "user-1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFileUpload_UnknownFieldReturns400(t *testing.T) {
	key := testKey(t)
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{err: ErrFieldNotFound},
		Scanner: &fakeScanner{},
		Store:   &fakeFileStore{},
		Files:   &fakeFileRepo{},
		Writer:  &countingWriter{},
	}
	req, ct := buildUpload(t, "x.png", []byte("data"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestFileUpload_NonFileFieldReturns400(t *testing.T) {
	key := testKey(t)
	textField := &profile_entity.ProfileField{
		Name: "passport", DataType: profile_entity.DataTypeText, IsActive: true,
	}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: textField},
		Scanner: &fakeScanner{},
		Store:   &fakeFileStore{},
		Files:   &fakeFileRepo{},
		Writer:  &countingWriter{},
	}
	req, ct := buildUpload(t, "x.png", []byte("data"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestFileUpload_ScannerRejectReturns415(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("", 0)},
		Scanner: &fakeScanner{err: media.ErrMimeNotAllowed},
		Store:   store,
		Files:   &fakeFileRepo{},
		Writer:  &countingWriter{},
	}
	req, ct := buildUpload(t, "x.bin", []byte("not an image"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", rec.Code)
	}
	if len(store.saveCalls) != 0 {
		t.Errorf("Save must not be called when the scanner rejects")
	}
}

func TestFileUpload_NarrowerAllowlistExcludesDetectedReturns415(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("image/png", 0)},
		Scanner: &fakeScanner{mime: "image/jpeg", ext: "jpg"},
		Store:   store,
		Files:   &fakeFileRepo{},
		Writer:  &countingWriter{},
	}
	req, ct := buildUpload(t, "x.jpg", []byte("jpeg-bytes"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415 from per-field allowlist, got %d", rec.Code)
	}
	if len(store.saveCalls) != 0 {
		t.Errorf("Save must not run when per-field allowlist rejects")
	}
}

func TestFileUpload_HappyPathNoPrior(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	repo := &fakeFileRepo{findByFieldErr: ErrAssetNotFound}
	writer := &countingWriter{}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("", 0)},
		Scanner: &fakeScanner{mime: "image/png", ext: "png"},
		Store:   store,
		Files:   repo,
		Writer:  writer,
	}
	req, ct := buildUpload(t, "hello.png", []byte("\x89PNG\r\n\x1a\n"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(store.saveCalls) != 1 {
		t.Fatalf("want 1 Save, got %d", len(store.saveCalls))
	}
	if len(repo.insertedMetas) != 1 {
		t.Fatalf("want 1 Insert, got %d", len(repo.insertedMetas))
	}
	if len(writer.calls) != 1 || writer.calls[0].fieldName != "passport" {
		t.Fatalf("writer should be called once with passport: %+v", writer.calls)
	}
	// Writer must receive the same asset id that was saved to disk.
	if writer.calls[0].value != store.saveCalls[0].assetID {
		t.Errorf("writer value=%v does not match saved asset id=%v",
			writer.calls[0].value, store.saveCalls[0].assetID)
	}
}

func TestFileUpload_HappyPathRotatesPriorAsset(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	prior := &FileMeta{AssetID: "old-id", Ext: "jpg"}
	repo := &fakeFileRepo{findByFieldMeta: prior}
	writer := &countingWriter{}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("", 0)},
		Scanner: &fakeScanner{mime: "image/png", ext: "png"},
		Store:   store,
		Files:   repo,
		Writer:  writer,
	}
	req, ct := buildUpload(t, "new.png", []byte("\x89PNG"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	// One Delete for the prior asset (bytes) + one repo delete for
	// the prior row. The fresh asset's Save is in saveCalls[0],
	// prior cleanup Delete is the only delete call.
	if len(store.deleteCalls) != 1 || store.deleteCalls[0].assetID != "old-id" {
		t.Fatalf("expected prior asset deleted, got %+v", store.deleteCalls)
	}
	if len(repo.deleteCalls) != 1 || repo.deleteCalls[0] != "old-id" {
		t.Fatalf("expected prior repo row deleted, got %+v", repo.deleteCalls)
	}
}

func TestFileUpload_RepoInsertErrorCleansUpSavedBytes(t *testing.T) {
	key := testKey(t)
	store := &fakeFileStore{}
	repo := &fakeFileRepo{findByFieldErr: ErrAssetNotFound, insertErr: errors.New("db down")}
	h := &FileUploadHandler{
		Slugs:   &fakeSlugResolver{appID: "app-1", orgID: "org-1"},
		Keys:    &fakeKeyLoader{pub: &key.PublicKey},
		Users:   &fakeUserOrgReader{mapping: map[string]string{"user-1": "org-1"}},
		Fields:  &fakeFieldConfigReader{field: fileField("", 0)},
		Scanner: &fakeScanner{mime: "image/png", ext: "png"},
		Store:   store,
		Files:   repo,
		Writer:  &countingWriter{},
	}
	req, ct := buildUpload(t, "hi.png", []byte("\x89PNG"))
	rec := serveUpload(t, h, req, ct, signBearer(t, key, "user-1"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	if len(store.deleteCalls) != 1 {
		t.Fatalf("want Save cleanup delete, got %d calls", len(store.deleteCalls))
	}
	if store.deleteCalls[0].assetID != store.saveCalls[0].assetID {
		t.Errorf("cleanup should target the just-saved asset id")
	}
}
