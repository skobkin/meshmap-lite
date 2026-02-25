package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	httpapi "meshmap-lite/internal/api/http"
	"meshmap-lite/internal/api/ws"
	"meshmap-lite/internal/config"
	"meshmap-lite/internal/dedup"
	"meshmap-lite/internal/frontend"
	"meshmap-lite/internal/ingest"
	"meshmap-lite/internal/logging"
	"meshmap-lite/internal/mqttclient"
	"meshmap-lite/internal/persistence/sqlite"
)

// Run initializes dependencies and starts HTTP, WS, and MQTT services.
func Run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	logMgr := logging.NewManager()
	if err := logMgr.Configure(cfg.Logging); err != nil {
		return err
	}
	log := logMgr.Logger("internal/app")
	log.Info("app starting",
		"http_listen_addr", cfg.Web.ListenAddr,
		"mqtt_host", cfg.MQTT.Host,
		"mqtt_port", cfg.MQTT.Port,
		"mqtt_root_topic", cfg.MQTT.RootTopic,
		"log_level", cfg.Logging.Level,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	storeLog := logMgr.Logger("internal/persistence/sqlite")
	store, err := sqlite.Open(ctx, cfg.Storage.SQL.URL, cfg.Storage.SQL.AutoMigrate, storeLog)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	hub := ws.NewHub(logMgr.Logger("internal/api/ws"))
	dedupStore := dedup.New(cfg.Storage.KV.Size, cfg.Storage.KV.TTL)
	ing := ingest.New(cfg, store, dedupStore, hub, logMgr.Logger("internal/ingest"))

	var mqttReady atomic.Bool
	mqtt := mqttclient.New(cfg.MQTT, logMgr.Logger("internal/mqttclient"), func(topic string, payload []byte) {
		mqttReady.Store(true)
		ing.HandleMessage(ctx, topic, payload)
	})

	api := httpapi.New(cfg, store, logMgr.Logger("internal/api/http"), mqttReady.Load, hub.ClientCount)
	apiMux := api.Routes(hub)
	mux := http.NewServeMux()
	mux.Handle("/api/", apiMux)
	mux.Handle("/healthz", apiMux)
	mux.Handle("/readyz", apiMux)
	mux.Handle("/", frontend.Handler())

	httpSrv := &http.Server{Addr: cfg.Web.ListenAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Info("mqtt service starting")
		if err := mqtt.Start(ctx); err != nil {
			log.Error("mqtt stopped", "err", err)
		}
	}()
	log.Info("stats ticker starting", "interval", cfg.Web.WS.HeartbeatInterval.String())
	go api.StartStatsTicker(ctx, hub.Emit)
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
