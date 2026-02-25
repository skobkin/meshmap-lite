package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"meshmap-lite/internal/config"
)

const humanTimeLayout = "2006-01-02 15:04:05.000"

// Manager owns process logger configuration and scoped child logger creation.
type Manager struct {
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewManager creates a manager with sane defaults.
func NewManager() *Manager {
	m := &Manager{}
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelInfo,
		ReplaceAttr: replaceAttrs,
	})
	m.logger = slog.New(h)
	slog.SetDefault(m.logger)

	return m
}

// Configure applies runtime logger settings from application config.
func (m *Manager) Configure(cfg config.LoggingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return err
	}

	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceAttrs,
	})
	m.logger = slog.New(h)
	slog.SetDefault(m.logger)

	return nil
}

// Logger returns a component-scoped logger with package attribute set.
func (m *Manager) Logger(pkg string) *slog.Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.logger.With("pkg", pkg)
}

func replaceAttrs(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key != slog.TimeKey {
		return attr
	}
	ts, ok := attr.Value.Any().(time.Time)
	if !ok {
		return attr
	}

	return slog.String(slog.TimeKey, ts.Format(humanTimeLayout))
}

func parseLevel(raw string) (slog.Leveler, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return nil, fmt.Errorf("unsupported log level: %q", raw)
	}
}
