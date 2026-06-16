package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"meshmap-lite/internal/buildinfo"
	"meshmap-lite/internal/config"
	"meshmap-lite/internal/updatecheck"
	"meshmap-lite/internal/updatecheck/sources/forgejo"
	"meshmap-lite/internal/updatecheck/sources/github"
)

// buildUpdateCheckManager constructs the multi-source release checker and
// registers every configured source (auto-registering the canonical
// meshmap-lite source when none is provided). It returns nil when the
// feature is disabled, so the HTTP layer can short-circuit cleanly.
//
// The Manager is returned unstarted; the caller is expected to call
// Start on the same ctx used for the rest of the app, so the fetcher
// goroutines exit cleanly on shutdown.
func buildUpdateCheckManager(ctx context.Context, cfg config.UpdateCheckConfig, logger *slog.Logger) *updatecheck.Manager {
	if !cfg.Enabled {
		logger.Info("update check disabled")

		return nil
	}

	sources := cfg.Sources
	if len(sources) == 0 {
		sources = []config.UpdateCheckSourceConfig{config.DefaultUpdateCheckSource}
	}

	mgr := updatecheck.NewManager(updatecheck.Options{
		Interval: cfg.Interval,
		Timeout:  cfg.Timeout,
		Logger:   logger,
	})

	for _, src := range sources {
		if err := registerUpdateCheckSource(mgr, src, logger); err != nil {
			logger.Warn("update check source registration failed",
				"source", src.Name,
				"error", err,
			)
		}
	}

	mgr.Start(ctx)
	logger.Info("update check manager started", "sources", mgr.Names())

	return mgr
}

// registerUpdateCheckSource constructs the per-platform Source adapter
// and registers it with the Manager. Unknown current_version_source
// values fall back to "" (no version comparison).
func registerUpdateCheckSource(mgr *updatecheck.Manager, cfg config.UpdateCheckSourceConfig, logger *slog.Logger) error {
	if cfg.Name == "" {
		return errors.New("source name is required")
	}

	var (
		src           updatecheck.Source
		postProcessor updatecheck.ReleasePostProcessor
		err           error
	)
	switch strings.TrimSpace(cfg.Type) {
	case updatecheck.SourceTypeForgejo:
		forgejoSource, forgejoErr := forgejo.New(forgejo.Options{
			Name:        cfg.Name,
			BaseURL:     cfg.BaseURL,
			Repository:  cfg.Repository,
			Limit:       cfg.Limit,
			PreReleases: cfg.PreReleasesEnabled(),
		})
		src, err = forgejoSource, forgejoErr
		if forgejoSource != nil {
			postProcessor = forgejoSource.PostProcessor()
		}
	case updatecheck.SourceTypeGitHub:
		githubSource, githubErr := github.New(github.Options{
			Name:        cfg.Name,
			BaseURL:     cfg.BaseURL,
			Repository:  cfg.Repository,
			Limit:       cfg.Limit,
			PreReleases: cfg.PreReleasesEnabled(),
		})
		src, err = githubSource, githubErr
		if githubSource != nil {
			postProcessor = githubSource.PostProcessor()
		}
	default:
		return errors.New("unsupported source type: " + cfg.Type)
	}
	if err != nil {
		return err
	}

	return mgr.Register(updatecheck.SourceSpec{
		Name:                cfg.Name,
		Label:               cfg.Label,
		Source:              src,
		CurrentVersion:      resolveCurrentVersion(cfg.CurrentVersionSource, logger),
		PostProcessMarkdown: cfg.PostProcessEnabled(),
		PostProcessor:       postProcessor,
	})
}

// resolveCurrentVersion translates the config-side current_version_source
// key into a concrete version string. Unknown keys are treated as "none".
func resolveCurrentVersion(key string, logger *slog.Logger) string {
	switch strings.TrimSpace(key) {
	case "buildinfo":
		return buildinfo.Version
	case "", "none":
		return ""
	default:
		logger.Debug("unknown current_version_source; treating as none", "key", key)

		return ""
	}
}
