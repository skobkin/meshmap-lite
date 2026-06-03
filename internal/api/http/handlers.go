package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"meshmap-lite/internal/repo"
)

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthStatusPayload{Status: "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	if s.ready != nil && !s.ready() {
		writeJSON(w, http.StatusServiceUnavailable, healthStatusPayload{Status: "not_ready"})

		return
	}
	writeJSON(w, http.StatusOK, healthStatusPayload{Status: "ready"})
}

func (s *Server) meta(w http.ResponseWriter, _ *http.Request) {
	infoAvailable := s.cfg.Info != nil
	infoSourceHash := ""
	if infoAvailable {
		infoSourceHash = s.cfg.Info.SourceHash
	}

	writeJSON(w, http.StatusOK, metaPayload{
		AppName:               s.cfg.AppName,
		Version:               s.cfg.Version,
		WebsocketPath:         "/api/v1/ws",
		ChatEnabled:           s.cfg.Web.Chat.Enabled,
		DefaultChatChannel:    s.cfg.Web.Chat.DefaultChannel,
		ShowRecentMessages:    s.cfg.Web.Chat.ShowRecentMessages,
		LogLiveUpdates:        s.cfg.Web.Log.LiveUpdates,
		LogPageSizeDefault:    s.cfg.Web.Log.PageSizeDefault,
		DisconnectedThreshold: s.cfg.Web.Map.DisconnectedThreshold.String(),
		InfoAvailable:         infoAvailable,
		InfoSourceHash:        infoSourceHash,
		Map: metaMapPayload{
			Clustering:           s.cfg.Web.Map.Clustering,
			TopologyCacheTTL:     s.cfg.Web.Map.TopologyCacheTTL.String(),
			PrecisionCirclesMode: string(s.cfg.Web.Map.PrecisionCirclesMode),
			DefaultView: metaDefaultViewPayload{
				Latitude:  s.cfg.Web.Map.DefaultView.Latitude,
				Longitude: s.cfg.Web.Map.DefaultView.Longitude,
				Zoom:      s.cfg.Web.Map.DefaultView.Zoom,
			},
		},
		Relevance: metaRelevancePayload{
			TelemetryMaxAge:        s.cfg.Web.Relevance.TelemetryMaxAge.String(),
			TopologyEvidenceMaxAge: s.cfg.Web.Relevance.TopologyEvidenceMaxAge.String(),
			MapPositionMaxAge:      s.cfg.Web.Relevance.MapPositionMaxAge.String(),
		},
	})
}

func (s *Server) info(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Info == nil {
		writeError(w, http.StatusNotFound, "info_not_configured")

		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}

	content := ""
	switch format {
	case "html":
		content = s.cfg.Info.HTML
	case "markdown":
		content = s.cfg.Info.Markdown
	default:
		writeError(w, http.StatusBadRequest, "invalid_format")

		return
	}

	writeJSON(w, http.StatusOK, infoPayload{
		Format:     format,
		SourceHash: s.cfg.Info.SourceHash,
		Content:    content,
	})
}

func (s *Server) channels(w http.ResponseWriter, _ *http.Request) {
	items := make([]channelPayload, 0, len(s.cfg.Channels))
	for name, channel := range s.cfg.Channels {
		items = append(items, channelPayload{Name: name, ChatEnabled: true, IsPrimary: channel.Primary})
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) mapNodes(w http.ResponseWriter, r *http.Request) {
	now := s.now().UTC()
	items, err := s.store.GetMapNodes(r.Context(), repo.MapNodeQuery{
		PositionObservedSince:  now.Add(-s.cfg.Web.Relevance.MapPositionMaxAge),
		TelemetryObservedSince: now.Add(-s.cfg.Web.Relevance.TelemetryMaxAge),
	})
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("map nodes canceled", "err", err)

			return
		}
		s.log.Error("map nodes", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) chatMessages(w http.ResponseWriter, r *http.Request) {
	query := parseChatQuery(r.URL.Query(), s.cfg.Web.Chat)
	if s.cfg.Web.Chat.HistoryWindow > 0 {
		query.ObservedSinceAt = s.now().UTC().Add(-s.cfg.Web.Chat.HistoryWindow)
	}
	items, err := s.store.ListChatEvents(r.Context(), query)
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("chat messages canceled", "channel", query.Channel, "err", err)

			return
		}
		s.log.Error("chat messages", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) logEvents(w http.ResponseWriter, r *http.Request) {
	query := parseLogQuery(r.URL.Query(), s.cfg.Web.Log)
	items, err := s.store.ListLogEvents(r.Context(), query)
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("log events canceled", "err", err)

			return
		}
		s.log.Error("log events", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) statsActivity(w http.ResponseWriter, r *http.Request) {
	now := s.now().UTC()
	periods := make([]activityPeriodPayload, 0, 2)
	for _, def := range s.activityPeriods() {
		period, err := s.loadActivityPeriod(r.Context(), def, now)
		if err != nil {
			if isRequestCanceled(err) {
				s.log.Debug("stats activity canceled", "period", def.key, "err", err)

				return
			}
			s.log.Error("stats activity", "period", def.key, "err", err)
			writeError(w, http.StatusInternalServerError, "internal_error")

			return
		}
		periods = append(periods, period)
	}
	writeJSON(w, http.StatusOK, activityPayload{GeneratedAt: now, Periods: periods})
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNodes(r.Context(), repo.NodeListQuery{
		PositionObservedSince: s.now().UTC().Add(-s.cfg.Web.Relevance.MapPositionMaxAge),
	})
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("list nodes canceled", "err", err)

			return
		}
		s.log.Error("list nodes", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) topologyEdges(w http.ResponseWriter, r *http.Request) {
	query := parseTopologyEdgeQuery(r.URL.Query())
	query.UpdatedSince = s.now().UTC().Add(-s.cfg.Web.Relevance.TopologyEvidenceMaxAge)
	items, err := s.store.ListTopologyEdges(r.Context(), query)
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("list topology edges canceled", "err", err)

			return
		}
		s.log.Error("list topology edges", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) nodeByID(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := nodeIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_node_id")

		return
	}

	now := s.now().UTC()
	item, err := s.store.GetNodeDetails(r.Context(), repo.NodeDetailsQuery{
		NodeID:                 nodeID,
		PositionObservedSince:  now.Add(-s.cfg.Web.Relevance.MapPositionMaxAge),
		TelemetryObservedSince: now.Add(-s.cfg.Web.Relevance.TelemetryMaxAge),
		TopologyUpdatedSince:   now.Add(-s.cfg.Web.Relevance.TopologyEvidenceMaxAge),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found")

			return
		}
		if isRequestCanceled(err) {
			s.log.Debug("get node canceled", "node_id", nodeID, "err", err)

			return
		}
		s.log.Error("get node", "node_id", nodeID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, item)
}

func isRequestCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
