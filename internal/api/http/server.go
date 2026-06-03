package httpapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/mqttclient"
	"meshmap-lite/internal/repo"
	"meshmap-lite/internal/siteinfo"
)

// Server serves HTTP API routes and shared operational endpoints.
type Server struct {
	cfg           Config
	store         repo.ReadStore
	log           *slog.Logger
	ready         func() bool
	wsClient      func() int
	mqttStatus    func() mqttclient.ConnectionStatus
	activityMu    sync.Mutex
	activityCache map[string]activityPeriodCache
	now           func() time.Time
}

// Config contains the subset of app config required by the HTTP API.
type Config struct {
	AppName  string
	Version  string
	Web      config.WebConfig
	Channels map[string]config.ChannelConfig
	Info     *siteinfo.Info
}

// New creates an HTTP API server with configured dependencies.
func New(cfg Config, store repo.ReadStore, log *slog.Logger, ready func() bool, wsClient func() int, mqttStatus func() mqttclient.ConnectionStatus) *Server {
	return &Server{
		cfg:           cfg,
		store:         store,
		log:           log,
		ready:         ready,
		wsClient:      wsClient,
		mqttStatus:    mqttStatus,
		activityCache: make(map[string]activityPeriodCache),
		now:           time.Now,
	}
}

// StartStatsTicker periodically emits runtime stats events.
func (s *Server) StartStatsTicker(ctx context.Context, emit func(domain.RealtimeEvent)) {
	startTickerLoop(ctx, s.log, "stats ticker", s.statsInterval(), func(now time.Time) {
		st, err := s.store.Stats(ctx, s.cfg.Web.Map.DisconnectedThreshold)
		if err != nil {
			s.log.Warn("collect stats failed", "err", err)

			return
		}
		st.WSClientsCount = s.wsClient()
		s.log.Debug("emit runtime stats",
			"known_nodes_count", st.KnownNodesCount,
			"online_nodes_count", st.OnlineNodesCount,
			"ws_clients", st.WSClientsCount,
			"last_ingest_at", st.LastIngestAt,
		)
		emit(domain.RealtimeEvent{Type: "stats", TS: now.UTC(), Payload: st})
	})
}

// StartHeartbeatTicker periodically emits websocket heartbeat events.
func (s *Server) StartHeartbeatTicker(ctx context.Context, emit func(domain.RealtimeEvent)) {
	startTickerLoop(ctx, s.log, "heartbeat ticker", s.heartbeatInterval(), func(now time.Time) {
		emit(s.HeartbeatEvent(now))
	})
}

// HeartbeatEvent builds the websocket heartbeat event payload for UI clients.
func (s *Server) HeartbeatEvent(now time.Time) domain.RealtimeEvent {
	return domain.RealtimeEvent{
		Type:    "ws.heartbeat",
		TS:      now.UTC(),
		Payload: s.heartbeatPayload(),
	}
}

func (s *Server) heartbeatPayload() heartbeatPayload {
	status := mqttclient.ConnectionStatusDisconnected
	if s.mqttStatus != nil && s.mqttStatus() == mqttclient.ConnectionStatusConnected {
		status = mqttclient.ConnectionStatusConnected
	}

	return heartbeatPayload{
		Status:               "ok",
		MQTTConnectionStatus: status,
	}
}
