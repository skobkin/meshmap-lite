package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

// Server serves HTTP API routes and shared operational endpoints.
type Server struct {
	cfg      config.Config
	store    repo.Store
	log      *slog.Logger
	ready    func() bool
	wsClient func() int
}

// New creates an HTTP API server with configured dependencies.
func New(cfg config.Config, store repo.Store, log *slog.Logger, ready func() bool, wsClient func() int) *Server {
	return &Server{cfg: cfg, store: store, log: log, ready: ready, wsClient: wsClient}
}

// Routes returns the API route multiplexer wrapped with request logging.
func (s *Server) Routes(wsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", http.HandlerFunc(s.healthz))
	mux.Handle("/readyz", http.HandlerFunc(s.readyz))
	mux.Handle("/api/v1/meta", http.HandlerFunc(s.meta))
	mux.Handle("/api/v1/channels", http.HandlerFunc(s.channels))
	mux.Handle("/api/v1/map/nodes", http.HandlerFunc(s.mapNodes))
	mux.Handle("/api/v1/chat/messages", http.HandlerFunc(s.chatMessages))
	mux.Handle("/api/v1/nodes", http.HandlerFunc(s.nodes))
	mux.Handle("/api/v1/nodes/", http.HandlerFunc(s.nodeByID))
	mux.Handle("/api/v1/ws", wsHandler)

	return s.withRequestLog(mux)
}

func (s *Server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			return
		}
		s.log.Debug("api request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", rw.status,
			"bytes", rw.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	if s.ready != nil && !s.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})

		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) meta(w http.ResponseWriter, _ *http.Request) {
	payload := map[string]interface{}{
		"version":                "dev",
		"websocket_path":         "/api/v1/ws",
		"chat_enabled":           s.cfg.Web.Chat.Enabled,
		"default_chat_channel":   s.cfg.Web.Chat.DefaultChannel,
		"show_recent_messages":   s.cfg.Web.Chat.ShowRecentMessages,
		"disconnected_threshold": s.cfg.Web.Map.DisconnectedThreshold.String(),
		"map": map[string]interface{}{
			"clustering": s.cfg.Web.Map.Clustering,
			"default_view": map[string]interface{}{
				"latitude":  s.cfg.Web.Map.DefaultView.Latitude,
				"longitude": s.cfg.Web.Map.DefaultView.Longitude,
				"zoom":      s.cfg.Web.Map.DefaultView.Zoom,
			},
		},
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) channels(w http.ResponseWriter, _ *http.Request) {
	type item struct {
		Name       string `json:"name"`
		ChatEnable bool   `json:"chat_enabled"`
		IsPrimary  bool   `json:"is_primary"`
	}
	items := make([]item, 0, len(s.cfg.Channels))
	for name, c := range s.cfg.Channels {
		items = append(items, item{Name: name, ChatEnable: true, IsPrimary: c.Primary})
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) mapNodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.GetMapNodes(r.Context())
	if err != nil {
		s.log.Error("map nodes", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) chatMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	channel := q.Get("channel")
	if channel == "" {
		channel = s.cfg.Web.Chat.DefaultChannel
	}
	limit := s.cfg.Web.Chat.ShowRecentMessages
	if raw := q.Get("limit"); raw != "" {
		limit = parseInt(raw, limit)
	}
	before := int64(parseInt(q.Get("before"), 0))
	items, err := s.store.ListChatEvents(r.Context(), channel, limit, before)
	if err != nil {
		s.log.Error("chat messages", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNodes(r.Context())
	if err != nil {
		s.log.Error("list nodes", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) nodeByID(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "missing_node_id")

		return
	}
	item, err := s.store.GetNodeDetails(r.Context(), nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found")

			return
		}
		s.log.Error("get node", "node_id", nodeID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, item)
}

// StartStatsTicker periodically emits realtime stats and heartbeat events.
func (s *Server) StartStatsTicker(ctx context.Context, emit func(domain.RealtimeEvent)) {
	interval := s.cfg.Web.WS.HeartbeatInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	s.log.Info("stats ticker started", "interval", interval.String())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer s.log.Info("stats ticker stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st, err := s.store.Stats(ctx, s.cfg.Web.Map.DisconnectedThreshold)
			if err != nil {
				s.log.Warn("collect stats failed", "err", err)

				continue
			}
			st.WSClientsCount = s.wsClient()
			s.log.Debug("emit runtime stats",
				"known_nodes_count", st.KnownNodesCount,
				"online_nodes_count", st.OnlineNodesCount,
				"ws_clients", st.WSClientsCount,
				"last_ingest_at", st.LastIngestAt,
			)
			emit(domain.RealtimeEvent{Type: "stats", TS: time.Now().UTC(), Payload: st})
			emit(domain.RealtimeEvent{Type: "ws.heartbeat", TS: time.Now().UTC(), Payload: map[string]string{"status": "ok"}})
		}
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func parseInt(v string, d int) int {
	if v == "" {

		return d
	}
	n, err := strconv.Atoi(v)
	if err != nil {

		return d
	}

	return n
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n

	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}

	return h.Hijack()
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(r)
		w.bytes += int(n)

		return n, err
	}
	n, err := io.Copy(w.ResponseWriter, r)
	w.bytes += int(n)

	return n, err
}

func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}

	return http.ErrNotSupported
}
