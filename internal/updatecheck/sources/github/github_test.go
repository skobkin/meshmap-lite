package github

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRejectsBadRepository(t *testing.T) {
	if _, err := New(Options{Name: "x", Repository: ""}); err == nil {
		t.Fatalf("expected error for empty repository")
	}
	if _, err := New(Options{Name: "x", Repository: "noowner"}); err == nil {
		t.Fatalf("expected error for ownerless repository")
	}
	if _, err := New(Options{Name: "x", Repository: "a/b/c"}); err == nil {
		t.Fatalf("expected error for repository with too many segments")
	}
}

func TestNewBuildsDefaultGitHubURLs(t *testing.T) {
	s, err := New(Options{
		Name:       "meshmap-lite",
		Repository: "skobkin/meshmap-lite",
		Limit:      7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := s.APIURL(), "https://api.github.com/repos/skobkin/meshmap-lite/releases?per_page=7&page=1"; got != want {
		t.Fatalf("unexpected APIURL: %q", got)
	}
	if got, want := s.ReleasesPageURL(), "https://github.com/skobkin/meshmap-lite/releases"; got != want {
		t.Fatalf("unexpected ReleasesPageURL: %q", got)
	}
}

func TestNewBuildsCustomEnterpriseURLs(t *testing.T) {
	s, err := New(Options{
		Name:       "meshmap-lite",
		BaseURL:    "https://github.example.org/api/v3/",
		Repository: "skobkin/meshmap-lite",
		Limit:      101,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := s.APIURL(), "https://github.example.org/api/v3/repos/skobkin/meshmap-lite/releases?per_page=100&page=1"; got != want {
		t.Fatalf("unexpected APIURL: %q", got)
	}
	if got, want := s.ReleasesPageURL(), "https://github.example.org/skobkin/meshmap-lite/releases"; got != want {
		t.Fatalf("unexpected ReleasesPageURL: %q", got)
	}
}

func TestPostProcessorUsesGitHubRepositoryAndUserBase(t *testing.T) {
	s, err := New(Options{
		Name:       "firmware",
		Repository: "meshtastic/firmware",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := s.PostProcessor()("See https://github.com/meshtastic/firmware/pull/10166 by @jp-bennett")
	want := "See [meshtastic/firmware#10166](https://github.com/meshtastic/firmware/pull/10166) by [@jp-bennett](https://github.com/jp-bennett)"
	if got != want {
		t.Fatalf("unexpected processed body: %q", got)
	}
}

func TestPostProcessorUsesGitHubEnterpriseBase(t *testing.T) {
	s, err := New(Options{
		Name:       "meshmap-lite",
		BaseURL:    "https://github.example.org/api/v3",
		Repository: "skobkin/meshmap-lite",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := s.PostProcessor()("See #12 by @skobkin")
	want := "See [#12](https://github.example.org/skobkin/meshmap-lite/issues/12) by [@skobkin](https://github.example.org/skobkin)"
	if got != want {
		t.Fatalf("unexpected processed body: %q", got)
	}
}

func TestNewDefaultsAndClampsLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		want  string
	}{
		{name: "zero", limit: 0, want: "per_page=15&page=1"},
		{name: "negative", limit: -1, want: "per_page=15&page=1"},
		{name: "max", limit: 100, want: "per_page=100&page=1"},
		{name: "above max", limit: 101, want: "per_page=100&page=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(Options{Name: "x", Repository: "a/b", Limit: tc.limit})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(s.APIURL(), tc.want) {
				t.Fatalf("expected APIURL to end with %q, got %q", tc.want, s.APIURL())
			}
		})
	}
}

func TestFetchReleasesDecodesFiltersAndSortsPayload(t *testing.T) {
	var accept string
	var version string
	var gotURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		version = r.Header.Get("X-GitHub-Api-Version")
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"tag_name":"v1.0.0","body":"old","html_url":"https://example.com/r/v1.0.0","published_at":"2026-02-10T01:00:00Z","draft":false,"prerelease":false},
			{"tag_name":"v2.0.0-rc1","body":"skip prerelease","html_url":"https://example.com/r/rc","published_at":"2026-02-14T01:00:00Z","draft":false,"prerelease":true},
			{"tag_name":"v1.1.0","body":"new","html_url":"https://example.com/r/v1.1.0","published_at":"2026-02-12T01:00:00Z","draft":false,"prerelease":false},
			{"tag_name":"v9.0.0","body":"skip draft","html_url":"https://example.com/r/draft","published_at":"2026-02-15T01:00:00Z","draft":true,"prerelease":false},
			{"tag_name":"","body":"skip empty","html_url":"https://example.com/r/empty","published_at":"2026-02-16T01:00:00Z","draft":false,"prerelease":false}
		]`)
	}))
	defer server.Close()

	s, err := New(Options{
		Name:       "meshmap-lite",
		BaseURL:    server.URL,
		Repository: "skobkin/meshmap-lite",
		Limit:      20,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	releases, err := s.FetchReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected FetchReleases error: %v", err)
	}
	if accept != "application/vnd.github+json" {
		t.Fatalf("unexpected Accept header: %q", accept)
	}
	if version != "2022-11-28" {
		t.Fatalf("unexpected API version header: %q", version)
	}
	if gotURL != "/repos/skobkin/meshmap-lite/releases?per_page=20&page=1" {
		t.Fatalf("unexpected request URL: %q", gotURL)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(releases))
	}
	if releases[0].Version != "v1.1.0" {
		t.Fatalf("expected newest release first, got %q", releases[0].Version)
	}
	if releases[1].Version != "v1.0.0" {
		t.Fatalf("unexpected second release: %q", releases[1].Version)
	}
	if releases[0].Body != "new" || releases[0].HTMLURL != "https://example.com/r/v1.1.0" {
		t.Fatalf("unexpected decoded release: %#v", releases[0])
	}
}

func TestFetchReleasesSurfacesNonOKStatusBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"rate limit"}`)
	}))
	defer server.Close()

	s, err := New(Options{
		Name:       "x",
		BaseURL:    server.URL,
		Repository: "a/b",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	_, err = s.FetchReleases(context.Background())
	if err == nil {
		t.Fatalf("expected error on non-200 response")
	}
	if !strings.Contains(err.Error(), `github source "x": unexpected status 403: {"message":"rate limit"}`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchReleasesWrapsDecodeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `not-json`)
	}))
	defer server.Close()

	s, err := New(Options{
		Name:       "x",
		BaseURL:    server.URL,
		Repository: "a/b",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	_, err = s.FetchReleases(context.Background())
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), `github source "x": decode:`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchReleasesRespectsPreReleases(t *testing.T) {
	const payload = `[
		{"tag_name":"v2.0.0","body":"stable","html_url":"https://example.com/r/v2.0.0","published_at":"2026-02-15T01:00:00Z","draft":false,"prerelease":false},
		{"tag_name":"v2.0.0-rc1","body":"rc","html_url":"https://example.com/r/rc","published_at":"2026-02-14T01:00:00Z","draft":false,"prerelease":true},
		{"tag_name":"v1.9.0","body":"older","html_url":"https://example.com/r/v1.9.0","published_at":"2026-02-10T01:00:00Z","draft":false,"prerelease":false}
	]`

	for _, tc := range []struct {
		name        string
		preReleases bool
		want        []string
	}{
		{name: "off drops prereleases", preReleases: false, want: []string{"v2.0.0", "v1.9.0"}},
		{name: "on keeps prereleases", preReleases: true, want: []string{"v2.0.0", "v2.0.0-rc1", "v1.9.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, payload)
			}))
			defer server.Close()

			s, err := New(Options{
				Name:        "x",
				BaseURL:     server.URL,
				Repository:  "a/b",
				Limit:       20,
				PreReleases: tc.preReleases,
				HTTPClient:  server.Client(),
			})
			if err != nil {
				t.Fatalf("unexpected New error: %v", err)
			}

			releases, err := s.FetchReleases(context.Background())
			if err != nil {
				t.Fatalf("unexpected FetchReleases error: %v", err)
			}
			if len(releases) != len(tc.want) {
				t.Fatalf("expected %d releases, got %d: %#v", len(tc.want), len(releases), releases)
			}
			for i, version := range tc.want {
				if releases[i].Version != version {
					t.Fatalf("expected release %d to be %q, got %q", i, version, releases[i].Version)
				}
			}
			if tc.preReleases && !releases[1].Prerelease {
				t.Fatalf("expected kept prerelease to preserve prerelease flag")
			}
		})
	}
}

func TestFetchReleasesKeepsMeshtasticAlphaMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"tag_name":"v2.7.25.104df5f","body":"alpha","html_url":"https://example.com/r/alpha","published_at":"2026-06-10T00:18:32Z","draft":false,"prerelease":true},
			{"tag_name":"v2.7.15.567b8ea","body":"stable","html_url":"https://example.com/r/stable","published_at":"2025-11-19T16:11:54Z","draft":false,"prerelease":false}
		]`)
	}))
	defer server.Close()

	s, err := New(Options{
		Name:        "firmware",
		BaseURL:     server.URL,
		Repository:  "meshtastic/firmware",
		Limit:       20,
		PreReleases: true,
		HTTPClient:  server.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected New error: %v", err)
	}

	releases, err := s.FetchReleases(context.Background())
	if err != nil {
		t.Fatalf("unexpected FetchReleases error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("expected 2 releases, got %d: %#v", len(releases), releases)
	}
	if releases[0].Version != "v2.7.25.104df5f" || !releases[0].Prerelease {
		t.Fatalf("expected latest Meshtastic alpha to be preserved, got %#v", releases[0])
	}
}
