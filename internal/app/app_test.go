package app

import (
	"io"
	"log/slog"
	"testing"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/updatecheck"
	"meshmap-lite/internal/updatecheck/sources/forgejo"
	"meshmap-lite/internal/updatecheck/sources/github"
)

func TestRegisterUpdateCheckSourceConstructsGitHubSource(t *testing.T) {
	mgr := updatecheck.NewManager(updatecheck.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	err := registerUpdateCheckSource(mgr, config.UpdateCheckSourceConfig{
		Name:                 "firmware",
		Label:                "Firmware",
		Type:                 "github",
		Repository:           "meshtastic/firmware",
		CurrentVersionSource: "none",
		Limit:                5,
	}, logger)
	if err != nil {
		t.Fatalf("unexpected registration error: %v", err)
	}
	if !mgr.HasSource("firmware") {
		t.Fatalf("expected firmware source to be registered")
	}
	src, ok := mgr.SnapshotSource("firmware").(*github.Source)
	if !ok {
		t.Fatalf("expected github source, got %T", mgr.SnapshotSource("firmware"))
	}
	if got, want := src.APIURL(), "https://api.github.com/repos/meshtastic/firmware/releases?per_page=5&page=1"; got != want {
		t.Fatalf("unexpected APIURL: %q", got)
	}
	if got, want := src.ReleasesPageURL(), "https://github.com/meshtastic/firmware/releases"; got != want {
		t.Fatalf("unexpected ReleasesPageURL: %q", got)
	}
}

func TestRegisterUpdateCheckSourcePropagatesPreReleases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tc := range []struct {
		name       string
		cfg        config.UpdateCheckSourceConfig
		wantAPIURL string
	}{
		{
			name: "github pre-releases on",
			cfg: config.UpdateCheckSourceConfig{
				Name:        "firmware-pr",
				Type:        "github",
				Repository:  "meshtastic/firmware",
				Limit:       7,
				PreReleases: true,
			},
			wantAPIURL: "https://api.github.com/repos/meshtastic/firmware/releases?per_page=7&page=1",
		},
		{
			name: "forgejo pre-releases on",
			cfg: config.UpdateCheckSourceConfig{
				Name:        "meshmap-pr",
				Type:        "forgejo",
				BaseURL:     "https://git.example.org",
				Repository:  "skobkin/meshmap-lite",
				Limit:       9,
				PreReleases: true,
			},
			wantAPIURL: "https://git.example.org/api/v1/repos/skobkin/meshmap-lite/releases?draft=false&limit=9",
		},
		{
			name: "forgejo pre-releases off",
			cfg: config.UpdateCheckSourceConfig{
				Name:       "meshmap-stable",
				Type:       "forgejo",
				BaseURL:    "https://git.example.org",
				Repository: "skobkin/meshmap-lite",
				Limit:      9,
			},
			wantAPIURL: "https://git.example.org/api/v1/repos/skobkin/meshmap-lite/releases?draft=false&limit=9&pre-release=false",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr := updatecheck.NewManager(updatecheck.Options{
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			})
			if err := registerUpdateCheckSource(mgr, tc.cfg, logger); err != nil {
				t.Fatalf("unexpected registration error: %v", err)
			}
			if !mgr.HasSource(tc.cfg.Name) {
				t.Fatalf("expected %q source to be registered", tc.cfg.Name)
			}
			var got string
			switch tc.cfg.Type {
			case "forgejo":
				src, ok := mgr.SnapshotSource(tc.cfg.Name).(*forgejo.Source)
				if !ok {
					t.Fatalf("expected forgejo source, got %T", mgr.SnapshotSource(tc.cfg.Name))
				}
				got = src.APIURL()
			case "github":
				src, ok := mgr.SnapshotSource(tc.cfg.Name).(*github.Source)
				if !ok {
					t.Fatalf("expected github source, got %T", mgr.SnapshotSource(tc.cfg.Name))
				}
				got = src.APIURL()
			}
			if got != tc.wantAPIURL {
				t.Fatalf("unexpected APIURL: got %q, want %q", got, tc.wantAPIURL)
			}
		})
	}
}
