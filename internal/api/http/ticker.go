package httpapi

import (
	"context"
	"log/slog"
	"time"

	"meshmap-lite/internal/config"
)

func (s *Server) statsInterval() time.Duration {
	if s.cfg.Web.WS.StatsInterval <= 0 {
		return config.DefaultWSStatsInterval
	}

	return s.cfg.Web.WS.StatsInterval
}

func (s *Server) heartbeatInterval() time.Duration {
	if s.cfg.Web.WS.HeartbeatInterval <= 0 {
		return config.DefaultWSHeartbeatInterval
	}

	return s.cfg.Web.WS.HeartbeatInterval
}

func startTickerLoop(ctx context.Context, log *slog.Logger, name string, interval time.Duration, tick func(time.Time)) {
	log.Info(name+" started", "interval", interval.String())
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer log.Info(name + " stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			tick(now)
		}
	}
}
