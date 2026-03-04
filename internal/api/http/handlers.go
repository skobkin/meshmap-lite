package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
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
	writeJSON(w, http.StatusOK, metaPayload{
		Version:               "dev",
		WebsocketPath:         "/api/v1/ws",
		ChatEnabled:           s.cfg.Web.Chat.Enabled,
		DefaultChatChannel:    s.cfg.Web.Chat.DefaultChannel,
		ShowRecentMessages:    s.cfg.Web.Chat.ShowRecentMessages,
		LogLiveUpdates:        s.cfg.Web.Log.LiveUpdates,
		LogPageSizeDefault:    s.cfg.Web.Log.PageSizeDefault,
		DisconnectedThreshold: s.cfg.Web.Map.DisconnectedThreshold.String(),
		Map: metaMapPayload{
			Clustering: s.cfg.Web.Map.Clustering,
			DefaultView: metaDefaultViewPayload{
				Latitude:  s.cfg.Web.Map.DefaultView.Latitude,
				Longitude: s.cfg.Web.Map.DefaultView.Longitude,
				Zoom:      s.cfg.Web.Map.DefaultView.Zoom,
			},
		},
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
	items, err := s.store.GetMapNodes(r.Context())
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

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListNodes(r.Context())
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

func (s *Server) nodeByID(w http.ResponseWriter, r *http.Request) {
	nodeID, ok := nodeIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_node_id")

		return
	}

	item, err := s.store.GetNodeDetails(r.Context(), nodeID)
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
