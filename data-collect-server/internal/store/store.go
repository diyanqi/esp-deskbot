// Package store persists IoTDA events and telemetry in PostgreSQL.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Options controls the PostgreSQL pool and migration behavior.
type Options struct {
	MaxConns       int32
	MinConns       int32
	MaxConnLife    time.Duration
	ConnectTimeout time.Duration
	RunMigrations  bool
	Logger         *slog.Logger
}

// Store wraps a PostgreSQL connection pool.
type Store struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New opens the pool, verifies connectivity and optionally migrates.
func New(ctx context.Context, databaseURL string, opts Options) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if opts.MaxConns > 0 {
		poolConfig.MaxConns = opts.MaxConns
	}
	if opts.MinConns >= 0 {
		poolConfig.MinConns = opts.MinConns
	}
	if opts.MaxConnLife > 0 {
		poolConfig.MaxConnLifetime = opts.MaxConnLife
	}
	if opts.ConnectTimeout > 0 {
		poolConfig.ConnConfig.ConnectTimeout = opts.ConnectTimeout
	}
	// Always read timestamptz values back as UTC regardless of the server's
	// default session timezone. This keeps the API contract deterministic.
	poolConfig.ConnConfig.RuntimeParams["timezone"] = "UTC"
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	store := &Store{pool: pool, logger: opts.Logger}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if opts.RunMigrations {
		if err := store.migrate(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return store, nil
}

// Close releases all pool connections.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Ping checks the database connection.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLockKey int64 = 723601

func (s *Store) migrate(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)

	for _, entry := range entries {
		version := filepath.Base(entry)
		var applied bool
		if err := conn.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).
			Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}

		contents, err := migrationFiles.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
		if s.logger != nil {
			s.logger.Info("applied migration", "version", version)
		}
	}
	return nil
}
