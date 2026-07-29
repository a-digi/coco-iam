package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloader_Download_WritesBodyToDestPath(t *testing.T) {
	var gotPath, gotAuthUser, gotAuthPass string
	var gotAuthOK bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotAuthUser, gotAuthPass, gotAuthOK = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("fake zip contents"))
	}))
	defer srv.Close()

	d := NewDownloader("acct-1", "key-1")
	d.baseURL = srv.URL

	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := d.Download(context.Background(), editionCountryCSV, dest); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if gotPath != "/GeoLite2-Country-CSV/download?suffix=zip" {
		t.Errorf("request path = %q, want /GeoLite2-Country-CSV/download?suffix=zip", gotPath)
	}
	if !gotAuthOK || gotAuthUser != "acct-1" || gotAuthPass != "key-1" {
		t.Errorf("basic auth = (%q, %q, ok=%v), want (acct-1, key-1, true)", gotAuthUser, gotAuthPass, gotAuthOK)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(content) != "fake zip contents" {
		t.Errorf("dest content = %q, want %q", content, "fake zip contents")
	}
}

func TestDownloader_Download_NonOKStatusIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	d := NewDownloader("bad-acct", "bad-key")
	d.baseURL = srv.URL

	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := d.Download(context.Background(), editionASNCSV, dest); err == nil {
		t.Fatal("Download() error = nil, want an error for a 401 response")
	}
}

func TestDownloader_Download_ContextCancelledIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDownloader("acct-1", "key-1")
	d.baseURL = srv.URL

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "out.zip")
	if err := d.Download(ctx, editionCountryCSV, dest); err == nil {
		t.Fatal("Download() error = nil, want an error for an already-cancelled context")
	}
}
