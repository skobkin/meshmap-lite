package httpapi

import (
	"context"
	"log/slog"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// Server serves HTTP API routes and shared operational endpoints.
type Server struct {
	cfg      Config
	store    repo.ReadStore
	log      *slog.Logger
	ready    func() bool
	wsClient func() int
}

// Config contains the subset of app config required by the HTTP API.
type Config struct {
	AppName  string
	Version  string
	Web      config.WebConfig
	Channels map[string]config.ChannelConfig
}

// New creates an HTTP API server with configured dependencies.
func New(cfg Config, store repo.ReadStore, log *slog.Logger, ready func() bool, wsClient func() int) *Server {
	return &Server{cfg: cfg, store: store, log: log, ready: ready, wsClient: wsClient}
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
		emit(domain.RealtimeEvent{Type: "ws.heartbeat", TS: now.UTC(), Payload: heartbeatPayload{Status: "ok"}})
	})
}
