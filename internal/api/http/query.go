package httpapi

import (
	"net/url"
	"slices"
	"strconv"
	"strings"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/domain"
	"meshmap-lite/internal/repo"
)

func parseChatQuery(values url.Values, chat config.ChatConfig) repo.ChatEventQuery {
	channel := values.Get("channel")
	if channel == "" {
		channel = chat.DefaultChannel
	}
	limit := chat.ShowRecentMessages
	if raw := values.Get("limit"); raw != "" {
		limit = parseInt(raw, limit)
	}

	return repo.ChatEventQuery{
		Channel:  channel,
		Limit:    limit,
		BeforeID: int64(parseInt(values.Get("before"), 0)),
	}
}

func parseLogQuery(values url.Values, logConfig config.LogConfig) domain.LogEventQuery {
	limit := logConfig.PageSizeDefault
	if raw := values.Get("limit"); raw != "" {
		limit = parseInt(raw, limit)
	}

	return domain.LogEventQuery{
		Limit:      limit,
		BeforeID:   int64(parseInt(values.Get("before"), 0)),
		EventKinds: parseEventKinds(values),
		Channel:    values.Get("channel"),
	}
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

func parseEventKinds(values url.Values) []domain.LogEventKind {
	raw := make([]string, 0)
	if kinds, ok := values["event_kind"]; ok {
		raw = append(raw, kinds...)
	}
	if kinds, ok := values["event_kinds"]; ok {
		raw = append(raw, kinds...)
	}
	if len(raw) == 0 {
		return nil
	}

	out := make([]domain.LogEventKind, 0, len(raw))
	for _, row := range raw {
		for _, part := range strings.Split(row, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				continue
			}
			kind, ok := domain.LogEventKindFromInt(n)
			if !ok || slices.Contains(out, kind) {
				continue
			}
			out = append(out, kind)
		}
	}
	if len(out) == 0 {
		return nil
	}

	return out
}

func nodeIDFromPath(path string) (string, bool) {
	const nodePathPrefix = "/api/v1/nodes/"

	if !strings.HasPrefix(path, nodePathPrefix) {
		return "", false
	}
	nodeID := strings.TrimPrefix(path, nodePathPrefix)
	if nodeID == "" || strings.Contains(nodeID, "/") {
		return "", false
	}

	return nodeID, true
}
