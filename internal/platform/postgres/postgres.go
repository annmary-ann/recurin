// Package postgres connects to recurin's database and applies its schema
// migrations (db/migrations, embedded in the binary).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/annmary-ann/recurin/db"
)

// Connect opens a connection pool for a postgres:// URL and checks that the
// server answers.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// pgx's message can echo the URL, password included.
		return nil, errors.New("postgres: cannot parse the database URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: cannot reach the database: %w", err)
	}
	return pool, nil
}

// Migrator applies and rolls back the embedded migrations.
type Migrator struct {
	db       *sql.DB
	provider *goose.Provider
}

// MigrationResult describes one migration that ran.
type MigrationResult = goose.MigrationResult

// MigrationStatus describes one known migration and whether it is applied.
type MigrationStatus = goose.MigrationStatus

// NewMigrator returns a Migrator that runs on pool. Close it when done; that
// does not close the pool.
func NewMigrator(pool *pgxpool.Pool) (*Migrator, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, db.Migrations())
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("postgres: load migrations: %w", err)
	}
	return &Migrator{db: sqlDB, provider: provider}, nil
}

// Up applies every pending migration, each in its own transaction. It returns
// the migrations it applied, none if the schema is already current.
func (m *Migrator) Up(ctx context.Context) ([]*MigrationResult, error) {
	res, err := m.provider.Up(ctx)
	if err != nil {
		return res, fmt.Errorf("postgres: migrate up: %w", err)
	}
	return res, nil
}

// Down rolls back the most recent migration.
func (m *Migrator) Down(ctx context.Context) (*MigrationResult, error) {
	res, err := m.provider.Down(ctx)
	if err != nil {
		return res, fmt.Errorf("postgres: migrate down: %w", err)
	}
	return res, nil
}

// Status lists every migration in version order with its state.
func (m *Migrator) Status(ctx context.Context) ([]*MigrationStatus, error) {
	st, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: migrate status: %w", err)
	}
	return st, nil
}

// Version returns the version of the last applied migration (0 if none).
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("postgres: migration version: %w", err)
	}
	return v, nil
}

// Close releases the Migrator's database handle. The pool stays open.
func (m *Migrator) Close() error { return m.db.Close() }
