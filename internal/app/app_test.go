package app

import (
	"io"
	"log/slog"
	"testing"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/updatecheck"
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
