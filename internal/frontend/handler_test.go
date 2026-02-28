package frontend

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestHandlerServesRootFromAssets(t *testing.T) {
	t.Parallel()

	h := Handler(Options{
		AssetsFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
		},
		MissingBuildHint: "missing build",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "<html>ok</html>" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestHandlerServesExistingAsset(t *testing.T) {
	t.Parallel()

	h := Handler(Options{
		AssetsFS: fstest.MapFS{
			"index.html":           &fstest.MapFile{Data: []byte("<html>ok</html>")},
			"assets/app.js":        &fstest.MapFile{Data: []byte("console.log('ok')")},
			"assets/styles.css":    &fstest.MapFile{Data: []byte("body{}")},
			"assets/logo-1x.png":   &fstest.MapFile{Data: []byte("png")},
			"assets/vector.svg":    &fstest.MapFile{Data: []byte("svg")},
			"assets/manifest.webm": &fstest.MapFile{Data: []byte("video")},
		},
		MissingBuildHint: "missing build",
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "console.log('ok')" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestSPAFileServerMissingAssetReturns404(t *testing.T) {
	t.Parallel()

	h := spaFileServer(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
	})

	req := httptest.NewRequest(http.MethodGet, "/assets/index-missing.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSPAFileServerClientRouteFallsBackToIndex(t *testing.T) {
	t.Parallel()

	h := spaFileServer(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
	})

	req := httptest.NewRequest(http.MethodGet, "/nodes", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "<html>ok</html>" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestHandlerFallsBackWhenBuildDirectoryMissing(t *testing.T) {
	t.Parallel()

	h := Handler(Options{
		DistPath:         filepath.Join(t.TempDir(), "missing"),
		MissingBuildHint: "frontend assets are not built",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "frontend assets are not built" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestHandlerUsesDistPathWhenPresent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>disk</html>"), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	h := Handler(Options{
		DistPath:         dir,
		MissingBuildHint: "frontend assets are not built",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "<html>disk</html>" {
		t.Fatalf("unexpected body: %q", got)
	}
}
