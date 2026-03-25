package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutesServeDocsIndex(t *testing.T) {
	t.Parallel()

	srv := New(Config{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	docs := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("docs"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/", nil)
	rec := httptest.NewRecorder()

	srv.Routes(http.NotFoundHandler(), docs).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := rec.Body.String(); got != "docs" {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestRoutesPreferAPIV1HandlersOverDocs(t *testing.T) {
	t.Parallel()

	srv := New(Config{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, nil)
	docs := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("docs handler should not serve /api/v1 routes")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	rec := httptest.NewRecorder()

	srv.Routes(http.NotFoundHandler(), docs).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
