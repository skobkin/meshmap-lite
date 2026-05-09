package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	// Register SQLite driver.
	_ "modernc.org/sqlite"

	"meshmap-lite/internal/config"
	"meshmap-lite/internal/persistence/sqlite/migrations"
)

// Store implements repository operations on top of SQLite.
type Store struct {
	db                *sql.DB
	log               *slog.Logger
	logMaxRows        int
	logPruneBatchRows int
	logChannelMu      sync.RWMutex
	logChannelIDs     map[string]int64
	nextLogPruneAtID  atomic.Int64
}

const (
	sqliteBusyTimeoutMillis = 5000
	sqliteJournalModeWAL    = "wal"
)

// Open creates a SQLite-backed store and optionally runs migrations.
func Open(ctx context.Context, cfg config.SQLConfig, log *slog.Logger) (*Store, error) {
	if log != nil {
		log.Info("opening sqlite database", "dsn", cfg.URL, "auto_migrate", cfg.AutoMigrate)
	}
	db, err := sql.Open("sqlite", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmas, err := configureSQLite(ctx, db)
	if err != nil {
		_ = db.Close()

		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{
		db:                db,
		log:               log,
		logMaxRows:        normalizedLimit(cfg.LogMaxRows),
		logPruneBatchRows: normalizedLimit(cfg.LogPruneBatchRows),
		logChannelIDs:     make(map[string]int64),
	}
	if cfg.AutoMigrate {
		if s.log != nil {
			s.log.Info("running sqlite migrations")
		}
		if err := s.Migrate(ctx); err != nil {
			return nil, err
		}
		if s.log != nil {
			s.log.Info("sqlite migrations complete")
		}
	}
	if s.log != nil {
		s.log.Info(
			"sqlite database ready",
			"journal_mode", pragmas.JournalMode,
			"busy_timeout_ms", pragmas.BusyTimeoutMillis,
			"foreign_keys", pragmas.ForeignKeys,
			"max_open_conns", db.Stats().MaxOpenConnections,
		)
	}

	return s, nil
}

type sqlitePragmas struct {
	JournalMode       string
	BusyTimeoutMillis int
	ForeignKeys       bool
}

func configureSQLite(ctx context.Context, db *sql.DB) (sqlitePragmas, error) {
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`PRAGMA busy_timeout = %d;`, sqliteBusyTimeoutMillis)); err != nil {
		return sqlitePragmas{}, fmt.Errorf("set sqlite busy_timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return sqlitePragmas{}, fmt.Errorf("set sqlite foreign_keys: %w", err)
	}

	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode = WAL;`).Scan(&journalMode); err != nil {
		return sqlitePragmas{}, fmt.Errorf("set sqlite journal_mode: %w", err)
	}

	var busyTimeout int
	if err := db.QueryRowContext(ctx, `PRAGMA busy_timeout;`).Scan(&busyTimeout); err != nil {
		return sqlitePragmas{}, fmt.Errorf("read sqlite busy_timeout: %w", err)
	}

	var foreignKeys int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys;`).Scan(&foreignKeys); err != nil {
		return sqlitePragmas{}, fmt.Errorf("read sqlite foreign_keys: %w", err)
	}

	return sqlitePragmas{
		JournalMode:       strings.ToLower(journalMode),
		BusyTimeoutMillis: busyTimeout,
		ForeignKeys:       foreignKeys == 1,
	}, nil
}

func normalizedLimit(limit int) int {
	if limit < 0 {
		return 0
	}

	return limit
}

// Close releases the underlying SQL database handle.
func (s *Store) Close() error {
	if s.log != nil {
		s.log.Info("closing sqlite database")
	}

	return s.db.Close()
}

// Migrate applies pending schema migrations.
func (s *Store) Migrate(ctx context.Context) error {
	if err := migrations.Apply(ctx, s.db, s.log); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}

	return nil
}
