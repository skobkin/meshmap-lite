package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"meshmap-lite/internal/config"
)

func TestOpen_ConfiguresSQLitePragmasForFileDatabase(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "meshmap-lite.sqlite")

	s, err := Open(ctx, config.SQLConfig{URL: dbPath, AutoMigrate: true}, nil)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var journalMode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode;`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != sqliteJournalModeWAL {
		t.Fatalf("expected journal_mode %q, got %q", sqliteJournalModeWAL, journalMode)
	}

	var busyTimeout int
	if err := s.db.QueryRowContext(ctx, `PRAGMA busy_timeout;`).Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != sqliteBusyTimeoutMillis {
		t.Fatalf("expected busy_timeout %d, got %d", sqliteBusyTimeoutMillis, busyTimeout)
	}

	var foreignKeys int
	if err := s.db.QueryRowContext(ctx, `PRAGMA foreign_keys;`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("expected foreign_keys enabled, got %d", foreignKeys)
	}

	if got := s.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("expected max open connections 1, got %d", got)
	}
}
