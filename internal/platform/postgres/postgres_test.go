package postgres_test

import (
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/annmary-ann/recurin/internal/platform/postgres"
	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

func TestConnectRejectsBadURLWithoutEchoingIt(t *testing.T) {
	t.Parallel()
	_, err := postgres.Connect(t.Context(), "postgres://user:s3cret@host:notaport/db")
	if err == nil {
		t.Fatal("want an error for a malformed URL")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("error leaks the password: %v", err)
	}
}

// Every migration must roll back cleanly and re-apply: down to zero, then up.
func TestMigrationsRoundTrip(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	m, err := postgres.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	ctx := t.Context()

	status, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) == 0 {
		t.Fatal("no migrations found")
	}
	for _, s := range status {
		if s.State != goose.StateApplied {
			t.Fatalf("template database has %s in state %s", s.Source.Path, s.State)
		}
	}
	latest := status[len(status)-1].Source.Version

	for range status {
		if _, err := m.Down(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if v, err := m.Version(ctx); err != nil || v != 0 {
		t.Fatalf("after rolling everything back: version %d, %v; want 0", v, err)
	}
	var tables int
	if err := pool.QueryRow(ctx, `select count(*) from pg_tables where schemaname = 'public' and tablename <> 'goose_db_version'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("%d tables left after rolling back every migration", tables)
	}

	applied, err := m.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != len(status) {
		t.Fatalf("re-applied %d migrations, want %d", len(applied), len(status))
	}
	if v, err := m.Version(ctx); err != nil || v != latest {
		t.Fatalf("version %d, %v; want %d", v, err, latest)
	}
	if again, err := m.Up(ctx); err != nil || len(again) != 0 {
		t.Fatalf("second Up applied %d, %v; want nothing", len(again), err)
	}
}
