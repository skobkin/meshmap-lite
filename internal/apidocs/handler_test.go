package apidocs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	t.Parallel()

	h := Handler(Options{SpecPath: writeSpecFile(t)})
	req := httptest.NewRequest(http.MethodGet, "/api/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rapi-doc") {
		t.Fatalf("expected RapiDoc page, got %q", rec.Body.String())
	}
}

func TestHandlerServesSpec(t *testing.T) {
	t.Parallel()

	specPath := writeSpecFile(t)
	h := Handler(Options{SpecPath: specPath})
	req := httptest.NewRequest(http.MethodGet, "/api/openapi.yaml", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/yaml; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", got)
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3.2.0") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandlerServesAssets(t *testing.T) {
	t.Parallel()

	h := Handler(Options{SpecPath: writeSpecFile(t)})
	req := httptest.NewRequest(http.MethodGet, "/api/assets/rapidoc-min.js", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "javascript") && !strings.Contains(got, "text/plain") {
		t.Fatalf("unexpected content type: %q", got)
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected asset body")
	}
}

func TestHandlerRedirectsSlashlessIndex(t *testing.T) {
	t.Parallel()

	h := Handler(Options{SpecPath: writeSpecFile(t)})
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/api/" {
		t.Fatalf("unexpected redirect target: %q", got)
	}
}

func writeSpecFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(path, []byte("openapi: 3.2.0\ninfo:\n  title: test\n  version: test\npaths: {}\n"), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	return path
}
