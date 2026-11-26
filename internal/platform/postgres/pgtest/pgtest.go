// Package pgtest gives integration tests a real, migrated and isolated
// Postgres database.
//
// Each call to New or NewURL creates a fresh database, cloned from a template
// that has every migration applied, and drops it when the test ends. Tests
// never see each other's rows, so they can run with t.Parallel.
//
// Where the server comes from:
//
//   - RECURIN_TEST_DATABASE_URL, if set: an existing server, for example the
//     Compose stack (postgres://recurin:recurin@localhost:5436/postgres). The
//     user needs the CREATEDB privilege.
//   - otherwise a postgres:16 container started with testcontainers, one per
//     test binary, removed by the testcontainers reaper when tests finish.
//
// With go test -short, tests that call New are skipped.
package pgtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/annmary-ann/recurin/db"
	"github.com/annmary-ann/recurin/internal/platform/postgres"
)

// EnvURL names the variable that points tests at an existing server.
const EnvURL = "RECURIN_TEST_DATABASE_URL"

const image = "postgres:16-alpine"

var (
	setupOnce sync.Once
	setupErr  error
	serverURL *url.URL // the server, with no database selected
	template  string   // name of the migrated template database
)

// New returns a pool connected to a fresh, fully migrated database that is
// dropped when the test ends.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := NewURL(t)
	pool, err := postgres.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	// Registered after NewURL's cleanup, so it runs first: close, then drop.
	t.Cleanup(pool.Close)
	return pool
}

// NewURL is like New but returns the database's URL, for code under test that
// opens its own connections. The database is dropped when the test ends.
func NewURL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("pgtest: skipping Postgres integration test in -short mode")
	}
	setupOnce.Do(func() { setupErr = setup() })
	if setupErr != nil {
		t.Fatalf("pgtest: %v", setupErr)
	}

	name := "recurin_test_" + randomHex(8)
	if err := adminExec(t.Context(),
		fmt.Sprintf("create database %s template %s", pgx.Identifier{name}.Sanitize(), pgx.Identifier{template}.Sanitize()),
	); err != nil {
		t.Fatalf("pgtest: create test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := adminExec(ctx, fmt.Sprintf("drop database if exists %s with (force)", pgx.Identifier{name}.Sanitize())); err != nil {
			t.Errorf("pgtest: drop test database: %v", err)
		}
	})
	return databaseURL(name)
}

// setup finds or starts the server and makes sure the template exists.
func setup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	raw := os.Getenv(EnvURL)
	if raw == "" {
		var err error
		if raw, err = startContainer(ctx); err != nil {
			return err
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse %s: %w", EnvURL, err)
	}
	u.Path = "/postgres"
	serverURL = u

	sum, err := migrationsHash()
	if err != nil {
		return err
	}
	// The name changes whenever a migration does, so an edited migration
	// never reuses a stale template on a long-lived server.
	template = "recurin_tpl_" + sum[:16]
	return ensureTemplate(ctx)
}

func startContainer(ctx context.Context) (string, error) {
	c, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("recurin"),
		tcpostgres.WithPassword("recurin"),
		tcpostgres.BasicWaitStrategies(),
		// Durability is pointless for throwaway test data; this makes
		// CREATE DATABASE and commits much faster.
		testcontainers.WithCmd("postgres", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"),
	)
	if err != nil {
		return "", fmt.Errorf("start Postgres container (is Docker running? or set %s): %w", EnvURL, err)
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("container connection string: %w", err)
	}
	return dsn, nil
}

// ensureTemplate builds the migrated template once per server. Several test
// binaries may race here, so an advisory lock serialises them, and the
// template is built under a temporary name and renamed only when complete.
func ensureTemplate(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, serverURL.String())
	if err != nil {
		return fmt.Errorf("connect to the test server: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	const lockKey = "recurin-pgtest-template"
	if _, err := conn.Exec(ctx, "select pg_advisory_lock(hashtext($1))", lockKey); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), "select pg_advisory_unlock(hashtext($1))", lockKey) }()

	var exists bool
	if err := conn.QueryRow(ctx, "select exists (select 1 from pg_database where datname = $1)", template).Scan(&exists); err != nil {
		return fmt.Errorf("look up template: %w", err)
	}
	if exists {
		return nil
	}

	building := template + "_build"
	quoted := pgx.Identifier{building}.Sanitize()
	if _, err := conn.Exec(ctx, "drop database if exists "+quoted+" with (force)"); err != nil {
		return fmt.Errorf("drop stale template build: %w", err)
	}
	if _, err := conn.Exec(ctx, "create database "+quoted); err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	if err := migrate(ctx, databaseURL(building)); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "alter database "+quoted+" rename to "+pgx.Identifier{template}.Sanitize()); err != nil {
		return fmt.Errorf("publish template: %w", err)
	}
	return nil
}

func migrate(ctx context.Context, dsn string) error {
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	m, err := postgres.NewMigrator(pool)
	if err != nil {
		return err
	}
	defer func() { _ = m.Close() }()
	if _, err := m.Up(ctx); err != nil {
		return fmt.Errorf("migrate template: %w", err)
	}
	return nil
}

func adminExec(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, serverURL.String())
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	_, err = conn.Exec(ctx, sql)
	return err
}

func databaseURL(name string) string {
	u := *serverURL
	u.Path = "/" + name
	return u.String()
}

func migrationsHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(db.Migrations(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(db.Migrations(), path)
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(b))
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("hash migrations: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return hex.EncodeToString(b)
}
