package main

import (
	"context"
	"fmt"
	"io"
	"path"
	"text/tabwriter"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/annmary-ann/recurin/internal/platform/postgres"
)

const migrateUsage = `Usage: recurin migrate [up|down|status]

  up      Apply every pending migration (the default)
  down    Roll back the most recent migration
  status  List migrations and whether each is applied

Needs RECURIN_DATABASE_URL.
`

func runMigrate(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	action := "up"
	switch len(args) {
	case 0:
	case 1:
		action = args[0]
	default:
		fmt.Fprint(stderr, migrateUsage)
		return exitUsage
	}
	switch action {
	case "up", "down", "status":
	case "help", "-h", "--help":
		fmt.Fprint(stdout, migrateUsage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "recurin migrate: unknown action %q\n\n%s", action, migrateUsage)
		return exitUsage
	}

	dsn := getenv("RECURIN_DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(stderr, "recurin migrate: RECURIN_DATABASE_URL is not set")
		return exitError
	}
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		fmt.Fprintf(stderr, "recurin migrate: %v\n", err)
		return exitError
	}
	defer pool.Close()
	m, err := postgres.NewMigrator(pool)
	if err != nil {
		fmt.Fprintf(stderr, "recurin migrate: %v\n", err)
		return exitError
	}
	defer func() { _ = m.Close() }()

	switch action {
	case "up":
		err = migrateUp(ctx, m, stdout)
	case "down":
		err = migrateDown(ctx, m, stdout)
	default:
		err = migrateStatus(ctx, m, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "recurin migrate %s: %v\n", action, err)
		return exitError
	}
	return exitOK
}

func migrateUp(ctx context.Context, m *postgres.Migrator, stdout io.Writer) error {
	results, err := m.Up(ctx)
	for _, r := range results {
		fmt.Fprintf(stdout, "applied %s (%s)\n", path.Base(r.Source.Path), r.Duration.Round(time.Millisecond))
	}
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(stdout, "schema is up to date")
	}
	return nil
}

func migrateDown(ctx context.Context, m *postgres.Migrator, stdout io.Writer) error {
	r, err := m.Down(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rolled back %s (%s)\n", path.Base(r.Source.Path), r.Duration.Round(time.Millisecond))
	return nil
}

func migrateStatus(ctx context.Context, m *postgres.Migrator, stdout io.Writer) error {
	status, err := m.Status(ctx)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATE\tAPPLIED AT\tMIGRATION")
	for _, s := range status {
		applied := "-"
		if s.State == goose.StateApplied {
			applied = s.AppliedAt.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.State, applied, path.Base(s.Source.Path))
	}
	return w.Flush()
}
