package forgejo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRejectsMissingBaseURL(t *testing.T) {
	if _, err := New(Options{Name: "x", Repository: "owner/repo"}); err == nil {
		t.Fatalf("expected error for missing base_url")
	}
}

func TestNewRejectsBadRepository(t *testing.T) {
	if _, err := New(Options{Name: "x", BaseURL: "https://example.org", Repository: ""}); err == nil {
		t.Fatalf("expected error for empty repository")
	}
	if _, err := New(Options{Name: "x", BaseURL: "https://example.org", Repository: "noowner"}); err == nil {
		t.Fatalf("expected error for ownerless repository")
	}
}

func TestNewBuildsExpectedURLs(t *testing.T) {
	s, err := New(Options{
		Name:       "meshmap-lite",
		BaseURL:    "https://git.example.org/",
		Repository: "skobkin/meshmap-lite",
		Limit:      7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := s.APIURL(), "https://git.example.org/api/v1/repos/skobkin/meshmap-lite/releases?draft=false&pre-release=false&limit=7"; got != want {
		t.Fatalf("unexpected APIURL: %q", got)
	}
	if got, want := s.ReleasesPageURL(), "https://git.example.org/skobkin/meshmap-lite/releases"; got != want {
		t.Fatalf("unexpected ReleasesPageURL: %q", got)
	}
}

func TestFetchReleasesDecodesPayload(t *testing.T) {
	var accept string
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"tag_name":"0.7.0","body":"body-1","html_url":"https://example.com/r/0.7.0","published_at":"2026-02-12T01:00:00Z"},
			{"tag_name":"0.6.1","body":"body-2","html_url":"https://example.com/r/0.6.1","published_at":"2026-02-10T01:00:00Z"}
		]`)
	}))
	defer server.Close()

	s, err := New(Options{
		Name:       "meshmap-lite",
		BaseURL:    server.URL,
		Repository: "skobkin/meshmap-lite",
		Limit:      15,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	releases, err := s.FetchReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected FetchReleases error: %v", err)
	}
	if accept != "application/json" {
		t.Fatalf("expected Accept application/json, got %q", accept)
	}
	if gotURL == "" || gotURL[:9] != "/api/v1/r" {
		t.Fatalf("expected request path to be /api/v1/..., got %q", gotURL)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].Version != "0.7.0" {
		t.Fatalf("unexpected latest version: %q", releases[0].Version)
	}
	if releases[1].Body != "body-2" {
		t.Fatalf("unexpected second release body: %q", releases[1].Body)
	}
}

func TestFetchReleasesSkipsEntriesWithoutVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"tag_name":"0.7.0","body":"good"},
			{"tag_name":"","body":"skip-me"}
		]`)
	}))
	defer server.Close()

	s, _ := New(Options{
		Name:       "x",
		BaseURL:    server.URL,
		Repository: "a/b",
		HTTPClient: server.Client(),
	})

	releases, err := s.FetchReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(releases))
	}
}

func TestFetchReleasesSurfacesNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()

	s, _ := New(Options{
		Name:       "x",
		BaseURL:    server.URL,
		Repository: "a/b",
		HTTPClient: server.Client(),
	})

	if _, err := s.FetchReleases(context.Background()); err == nil {
		t.Fatalf("expected error on non-200 response")
	}
}
