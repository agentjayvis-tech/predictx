package repository

import (
	"context"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewPool creates a pgx connection pool from the given DATABASE_URL.
// maxConns and minConns control pool sizing.
func NewPool(ctx context.Context, databaseURL string, maxConns, minConns int32, log *zap.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	log.Info("database connected", zap.Int32("max_conns", maxConns))
	return pool, nil
}

// RunMigrations applies all pending migrations from the given directory.
func RunMigrations(databaseURL, migrationsDir string, log *zap.Logger) error {
	m, err := migrate.New("file://"+migrationsDir, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate: up: %w", err)
	}

	log.Info("migrations applied")
	return nil
}
