package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"meshmap-lite/internal/repo"
	"meshmap-lite/internal/siteinfo"
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
		UpdateCheckAvailable:  s.updateMgr != nil,
		UpdateCheckSources:    s.updateSourceSummaries(),
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

// updateSourceSummaries builds the per-source metadata list embedded
// in /api/v1/meta. Sources without a cached snapshot are still listed
// (with empty releases and SourceHash) so the SPA can render their tab
// labels up-front; the "ready" check is the SourceHash being non-empty.
func (s *Server) updateSourceSummaries() []*updateSourceSummary {
	if s.updateMgr == nil {
		return nil
	}

	labels := s.updateMgr.Labels()
	names := s.updateMgr.Names()
	summaries := make([]*updateSourceSummary, 0, len(names))
	for _, name := range names {
		summary := &updateSourceSummary{
			Name:     name,
			Label:    labels[name],
			Releases: []updateReleaseMetadataEntry{},
		}

		// ReleasesPageURL stays empty when the platform adapter didn't
		// supply one; the SPA falls back to a generic label.
		if src := s.updateMgr.SnapshotSource(name); src != nil {
			summary.ReleasesPageURL = src.ReleasesPageURL()
		}

		snap, ok := s.updateMgr.Snapshot(name)
		if ok {
			summary.SourceHash = snap.SourceHash
			summary.CurrentVersion = snap.CurrentVersion
			if snap.Latest.Version != "" {
				summary.LatestVersion = snap.Latest.Version
			}
			summary.UpdateAvailable = snap.UpdateAvailable
			summary.Releases = make([]updateReleaseMetadataEntry, 0, len(snap.Releases))
			for _, r := range snap.Releases {
				summary.Releases = append(summary.Releases, updateReleaseMetadataEntry{
					Version:     r.Version,
					PublishedAt: r.PublishedAt,
					Prerelease:  r.Prerelease,
				})
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

func (s *Server) updates(w http.ResponseWriter, r *http.Request) {
	if s.updateMgr == nil {
		writeError(w, http.StatusNotFound, "update_check_not_configured")

		return
	}

	sourceName := r.URL.Query().Get("source")
	if sourceName == "" {
		names := s.updateMgr.Names()
		if len(names) == 0 {
			writeError(w, http.StatusNotFound, "update_check_not_configured")

			return
		}
		sourceName = names[0]
	}
	if !s.updateMgr.HasSource(sourceName) {
		writeError(w, http.StatusNotFound, "update_check_source_not_found")

		return
	}

	snap, ok := s.updateMgr.Snapshot(sourceName)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "update_check_not_ready")

		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "html"
	}
	if format != "html" && format != "markdown" {
		writeError(w, http.StatusBadRequest, "invalid_format")

		return
	}

	releases := make([]updateReleaseEntry, 0, len(snap.Releases))
	for _, rel := range snap.Releases {
		body := rel.Body
		if format == "html" {
			rendered, err := siteinfo.RenderMarkdown([]byte(rel.Body))
			if err != nil {
				s.log.Warn("render release body failed", "source", sourceName, "version", rel.Version, "err", err)
				rendered = rel.Body
			}
			body = rendered
		}
		releases = append(releases, updateReleaseEntry{
			Version:     rel.Version,
			PublishedAt: rel.PublishedAt,
			HTMLURL:     rel.HTMLURL,
			Body:        body,
			Prerelease:  rel.Prerelease,
		})
	}

	writeJSON(w, http.StatusOK, updatesPayload{
		Format:     format,
		Source:     sourceName,
		SourceHash: snap.SourceHash,
		Releases:   releases,
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
	payload, err := s.loadTopologyEdges(r.Context(), query, s.now().UTC())
	if err != nil {
		if isRequestCanceled(err) {
			s.log.Debug("list topology edges canceled", "err", err)

			return
		}
		s.log.Error("list topology edges", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error")

		return
	}
	writeJSON(w, http.StatusOK, payload)
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
