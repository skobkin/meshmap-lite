package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meshmap-lite/internal/repo/testkit"
	"meshmap-lite/internal/updatecheck"
)

// stubUpdateSource is a test double that implements updatecheck.Source and
// returns canned release data without making any network call.
type stubUpdateSource struct {
	name     string
	pageURL  string
	releases []updatecheck.ReleaseInfo
}

func (s *stubUpdateSource) Name() string            { return s.name }
func (s *stubUpdateSource) ReleasesPageURL() string { return s.pageURL }
func (s *stubUpdateSource) FetchReleases(_ context.Context) ([]updatecheck.ReleaseInfo, error) {
	return s.releases, nil
}

func newUpdatesTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newUpdatesTestManager(releases []updatecheck.ReleaseInfo) *updatecheck.Manager {
	return newUpdatesTestManagerWithPostProcess(releases, true)
}

func newUpdatesTestManagerWithPostProcess(releases []updatecheck.ReleaseInfo, postProcess bool) *updatecheck.Manager {
	mgr := updatecheck.NewManager(updatecheck.Options{
		Interval: time.Hour,
		Timeout:  time.Second,
		Logger:   newUpdatesTestLogger(),
	})
	src := &stubUpdateSource{
		name:     "meshmap-lite",
		pageURL:  "https://git.example/skobkin/meshmap-lite/releases",
		releases: releases,
	}
	if err := mgr.Register(updatecheck.SourceSpec{
		Name:                "meshmap-lite",
		Label:               "Map",
		Source:              src,
		CurrentVersion:      "0.6.0",
		PostProcessMarkdown: postProcess,
	}); err != nil {
		panic(err)
	}

	return mgr
}

func TestMetaHandlerExposesUpdateCheckAvailable(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		srv := New(Config{}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
		rec := httptest.NewRecorder()

		srv.meta(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload metaPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if payload.UpdateCheckAvailable {
			t.Fatalf("expected update_check_available=false when manager is nil")
		}
		if payload.UpdateCheckSources != nil {
			t.Fatalf("expected no update_check_sources when manager is nil, got %#v", payload.UpdateCheckSources)
		}
	})

	t.Run("enabled with snapshot", func(t *testing.T) {
		now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
			{Version: "0.7.0", Body: "## Notes", HTMLURL: "https://example/v0.7.0", PublishedAt: now},
			{Version: "0.6.1", Body: "older", HTMLURL: "https://example/v0.6.1", PublishedAt: now.Add(-24 * time.Hour)},
		})
		srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

		// Warm the cache with a synthetic snapshot so the meta extension
		// can list release metadata without going through the fetcher.
		mgr.SeedSnapshot("meshmap-lite", updatecheck.UpdateSnapshot{
			Source:         "meshmap-lite",
			CurrentVersion: "0.6.0",
			Latest: updatecheck.ReleaseInfo{
				Version:     "0.7.0",
				PublishedAt: now,
			},
			Releases: []updatecheck.ReleaseInfo{
				{Version: "0.7.0", PublishedAt: now, Prerelease: true},
				{Version: "0.6.1", PublishedAt: now.Add(-24 * time.Hour)},
			},
			UpdateAvailable: true,
			CheckedAt:       now,
			SourceHash:      "deadbeef",
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
		rec := httptest.NewRecorder()

		srv.meta(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
		var payload metaPayload
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !payload.UpdateCheckAvailable {
			t.Fatalf("expected update_check_available=true")
		}
		if len(payload.UpdateCheckSources) != 1 {
			t.Fatalf("expected one source summary, got %d", len(payload.UpdateCheckSources))
		}
		sum := payload.UpdateCheckSources[0]
		if sum.Name != "meshmap-lite" || sum.Label != "Map" {
			t.Fatalf("unexpected summary: name=%q label=%q", sum.Name, sum.Label)
		}
		if sum.LatestVersion != "0.7.0" || sum.CurrentVersion != "0.6.0" {
			t.Fatalf("unexpected version metadata: latest=%q current=%q", sum.LatestVersion, sum.CurrentVersion)
		}
		if !sum.UpdateAvailable {
			t.Fatalf("expected update_available=true")
		}
		if sum.ReleasesPageURL != "https://git.example/skobkin/meshmap-lite/releases" {
			t.Fatalf("unexpected releases page url: %q", sum.ReleasesPageURL)
		}
		if len(sum.Releases) != 2 || sum.Releases[0].Version != "0.7.0" {
			t.Fatalf("unexpected release metadata list: %#v", sum.Releases)
		}
		if !sum.Releases[0].Prerelease {
			t.Fatalf("expected prerelease metadata to be exposed")
		}
	})
}

func TestUpdatesHandlerReturns404WhenNotConfigured(t *testing.T) {
	srv := New(Config{}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "update_check_not_configured" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestUpdatesHandlerReturns404ForUnknownSource(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", PublishedAt: now},
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates?source=does-not-exist", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "update_check_source_not_found" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestUpdatesHandlerReturns503WhenNotReady(t *testing.T) {
	// Manager is registered but no snapshot has been cached yet.
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", PublishedAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)},
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates?source=meshmap-lite", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "update_check_not_ready" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestUpdatesHandlerReturnsHTMLByDefault(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", Body: "# Heading", HTMLURL: "https://example/v0.7.0", PublishedAt: now, Prerelease: true},
	})
	mgr.SeedSnapshot("meshmap-lite", updatecheck.UpdateSnapshot{
		Source: "meshmap-lite",
		Latest: updatecheck.ReleaseInfo{Version: "0.7.0", PublishedAt: now, Prerelease: true},
		Releases: []updatecheck.ReleaseInfo{
			{Version: "0.7.0", Body: "# Heading", HTMLURL: "https://example/v0.7.0", PublishedAt: now, Prerelease: true},
		},
		SourceHash: "feedface",
		CheckedAt:  now,
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates?source=meshmap-lite", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload updatesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Format != "html" {
		t.Fatalf("expected format=html, got %q", payload.Format)
	}
	if payload.Source != "meshmap-lite" || payload.SourceHash != "feedface" {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if len(payload.Releases) != 1 {
		t.Fatalf("expected one release, got %d", len(payload.Releases))
	}
	rel := payload.Releases[0]
	if rel.Version != "0.7.0" {
		t.Fatalf("unexpected version: %q", rel.Version)
	}
	if rel.HTMLURL != "https://example/v0.7.0" {
		t.Fatalf("unexpected html_url: %q", rel.HTMLURL)
	}
	if !rel.Prerelease {
		t.Fatalf("expected prerelease flag to be exposed")
	}
	// goldmark renders # Heading as <h1 id="...">Heading</h1>\n.
	if rel.Body == "" || rel.Body == "# Heading" {
		t.Fatalf("expected HTML body, got %q", rel.Body)
	}
}

func TestUpdatesHandlerReturnsMarkdown(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", Body: "# Heading", PublishedAt: now},
	})
	mgr.SeedSnapshot("meshmap-lite", updatecheck.UpdateSnapshot{
		Source: "meshmap-lite",
		Latest: updatecheck.ReleaseInfo{Version: "0.7.0", PublishedAt: now},
		Releases: []updatecheck.ReleaseInfo{
			{Version: "0.7.0", Body: "# Heading", HTMLURL: "https://example/v0.7.0", PublishedAt: now},
		},
		SourceHash: "feedface",
		CheckedAt:  now,
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates?source=meshmap-lite&format=markdown", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload updatesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Format != "markdown" {
		t.Fatalf("expected format=markdown, got %q", payload.Format)
	}
	if len(payload.Releases) != 1 || payload.Releases[0].Body != "# Heading" {
		t.Fatalf("expected raw markdown body, got %#v", payload.Releases)
	}
}

func TestUpdatesHandlerRejectsInvalidFormat(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", PublishedAt: now},
	})
	mgr.SeedSnapshot("meshmap-lite", updatecheck.UpdateSnapshot{
		Source:     "meshmap-lite",
		Latest:     updatecheck.ReleaseInfo{Version: "0.7.0", PublishedAt: now},
		Releases:   []updatecheck.ReleaseInfo{{Version: "0.7.0", PublishedAt: now}},
		SourceHash: "feedface",
		CheckedAt:  now,
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates?source=meshmap-lite&format=json", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload errorPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "invalid_format" {
		t.Fatalf("unexpected error payload: %+v", payload)
	}
}

func TestUpdatesHandlerDefaultsSourceToFirstRegistered(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mgr := newUpdatesTestManager([]updatecheck.ReleaseInfo{
		{Version: "0.7.0", PublishedAt: now},
	})
	mgr.SeedSnapshot("meshmap-lite", updatecheck.UpdateSnapshot{
		Source:     "meshmap-lite",
		Latest:     updatecheck.ReleaseInfo{Version: "0.7.0", PublishedAt: now},
		Releases:   []updatecheck.ReleaseInfo{{Version: "0.7.0", PublishedAt: now}},
		SourceHash: "feedface",
		CheckedAt:  now,
	})
	srv := New(Config{Updates: mgr}, &testkit.FakeStore{}, newUpdatesTestLogger(), nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/updates", nil)
	rec := httptest.NewRecorder()

	srv.updates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload updatesPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Source != "meshmap-lite" {
		t.Fatalf("expected source to default to first registered name, got %q", payload.Source)
	}
}
