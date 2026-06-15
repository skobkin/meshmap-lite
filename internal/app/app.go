package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	httpapi "meshmap-lite/internal/api/http"
	"meshmap-lite/internal/api/ws"
	"meshmap-lite/internal/apidocs"
	"meshmap-lite/internal/buildinfo"
	"meshmap-lite/internal/config"
	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/frontend"
	"meshmap-lite/internal/ingest"
	"meshmap-lite/internal/logging"
	"meshmap-lite/internal/mqttclient"
	"meshmap-lite/internal/persistence/sqlite"
	"meshmap-lite/internal/siteinfo"
	"meshmap-lite/internal/updatecheck"
	"meshmap-lite/internal/updatecheck/sources/forgejo"
	githubsource "meshmap-lite/internal/updatecheck/sources/github"
)

const missingFrontendBuildHint = "frontend assets are not built; run `cd web && npm run build`"

// Run initializes dependencies and starts HTTP, WS, and MQTT services.
func Run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	logMgr, err := logging.NewManager(logging.Options{
		Level:      cfg.Logging.Level,
		SetDefault: true,
	})
	if err != nil {
		return err
	}
	log := logMgr.Logger("internal/app")
	log.Info("app starting",
		"app_name", buildinfo.AppName,
		"version", buildinfo.Version,
		"http_listen_addr", cfg.Web.ListenAddr,
		"mqtt_host", cfg.MQTT.Host,
		"mqtt_port", cfg.MQTT.Port,
		"mqtt_root_topic", cfg.MQTT.RootTopic,
		"log_level", cfg.Logging.Level,
	)

	info, err := siteinfo.Load(cfg.Web.Info.File)
	if err != nil {
		return err
	}
	if info != nil {
		log.Info("site info loaded", "file", cfg.Web.Info.File, "source_hash", info.SourceHash)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	storeLog := logMgr.Logger("internal/persistence/sqlite")
	store, err := sqlite.Open(ctx, cfg.Storage.SQL, storeLog)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var api *httpapi.Server
	hub := ws.NewHub(logMgr.Logger("internal/api/ws"), ws.Options{
		OnConnect: func(_ *http.Request, send func(domain.RealtimeEvent) error) error {
			if api == nil {
				return nil
			}

			return send(api.HeartbeatEvent(time.Now()))
		},
	})
	dedupStore := dedup.New(dedup.Options{
		Size: cfg.Storage.KV.Size,
		TTL:  cfg.Storage.KV.TTL,
	})
	ing := ingest.New(ingest.Config{
		RootTopic: cfg.MQTT.RootTopic,
		Traceroute: ingest.TracerouteConfig{
			Timeout:        cfg.Ingest.Traceroute.Timeout,
			MaxEntries:     cfg.Ingest.Traceroute.MaxEntries,
			FinalRetention: cfg.Ingest.Traceroute.FinalRetention,
		},
		MapReports: ingest.MapReportsConfig{
			Enabled:     cfg.Ingest.MapReports.Enabled,
			TopicSuffix: cfg.Ingest.MapReports.TopicSuffix,
		},
		Channels: ingestChannels(cfg.Channels),
		Log: ingest.LogConfig{
			LiveUpdates: cfg.Web.Log.LiveUpdates,
		},
	}, store, dedupStore, hub, logMgr.Logger("internal/ingest"))

	var mqttReady atomic.Bool
	mqtt := mqttclient.New(mqttclient.Options{
		Host:             cfg.MQTT.Host,
		Port:             cfg.MQTT.Port,
		TLS:              cfg.MQTT.TLS,
		ClientID:         cfg.MQTT.ClientID,
		Username:         cfg.MQTT.Username,
		Password:         cfg.MQTT.Password,
		RootTopic:        cfg.MQTT.RootTopic,
		SubscribeQoS:     cfg.MQTT.SubscribeQoS,
		CleanSession:     cfg.MQTT.CleanSession,
		ReconnectTimeout: cfg.MQTT.ReconnectTimeout,
		ConnectTimeout:   cfg.MQTT.ConnectTimeout,
		Keepalive:        cfg.MQTT.Keepalive,
	}, logMgr.Logger("internal/mqttclient"), func(topic string, payload []byte) {
		mqttReady.Store(true)
		ing.HandleMessage(ctx, topic, payload)
	})

	api = httpapi.New(httpapi.Config{
		AppName:  buildinfo.AppName,
		Version:  buildinfo.Version,
		Web:      cfg.Web,
		Channels: cfg.Channels,
		Info:     info,
		Updates:  buildUpdateCheckManager(ctx, cfg.UpdateCheck, logMgr.Logger("internal/updatecheck")),
	}, store, logMgr.Logger("internal/api/http"), mqttReady.Load, hub.ClientCount, mqtt.ConnectionStatus)
	apiMux := api.Routes(hub, apidocs.Handler(apidocs.Options{
		SpecURL: "/api/openapi.yaml",
		Title:   buildinfo.AppName + " API",
	}))
	mux := http.NewServeMux()
	mux.Handle("/api/", apiMux)
	mux.Handle("/healthz", apiMux)
	mux.Handle("/readyz", apiMux)
	mux.Handle("/", frontend.Handler(frontend.Options{
		MissingBuildHint: missingFrontendBuildHint,
	}))

	httpSrv := &http.Server{Addr: cfg.Web.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Info("mqtt service starting")
		if err := mqtt.Start(ctx); err != nil {
			log.Error("mqtt stopped", "err", err)
		}
	}()
	log.Info("stats ticker starting", "interval", cfg.Web.WS.StatsInterval.String())
	go api.StartStatsTicker(ctx, hub.Emit)
	log.Info("heartbeat ticker starting", "interval", cfg.Web.WS.HeartbeatInterval.String())
	go api.StartHeartbeatTicker(ctx, hub.Emit)
	go func() {
		log.Info("http server listening", "addr", cfg.Web.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("shutdown initiated")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		return err
	}
	log.Info("shutdown complete")

	return nil
}

func ingestChannels(channels map[string]config.ChannelConfig) map[string]ingest.ChannelConfig {
	out := make(map[string]ingest.ChannelConfig, len(channels))
	for name, ch := range channels {
		out[name] = ingest.ChannelConfig{
			PSK:     ch.PSK,
			Primary: ch.Primary,
		}
	}

	return out
}

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
		src updatecheck.Source
		err error
	)
	switch strings.TrimSpace(cfg.Type) {
	case "forgejo":
		src, err = forgejo.New(forgejo.Options{
			Name:       cfg.Name,
			BaseURL:    cfg.BaseURL,
			Repository: cfg.Repository,
			Limit:      cfg.Limit,
		})
	case "github":
		src, err = githubsource.New(githubsource.Options{
			Name:       cfg.Name,
			Repository: cfg.Repository,
			BaseURL:    cfg.BaseURL,
		})
	default:
		return errors.New("unsupported source type: " + cfg.Type)
	}
	if err != nil {
		return err
	}

	return mgr.Register(updatecheck.SourceSpec{
		Name:           cfg.Name,
		Label:          cfg.Label,
		Source:         src,
		CurrentVersion: resolveCurrentVersion(cfg.CurrentVersionSource, logger),
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
