package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

func TestMigrateArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"unknown action", []string{"migrate", "sideways"}, exitUsage, "", `unknown action "sideways"`},
		{"too many arguments", []string{"migrate", "up", "now"}, exitUsage, "", "Usage: recurin migrate"},
		{"help", []string{"migrate", "help"}, exitOK, "Roll back the most recent migration", ""},
		{"needs a database URL", []string{"migrate"}, exitError, "", "RECURIN_DATABASE_URL is not set"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := run(t.Context(), tt.args, noEnv, &stdout, &stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// Drives the real command against Postgres: status, down, status, up, up.
func TestMigrateAgainstPostgres(t *testing.T) {
	t.Parallel()
	dsn := pgtest.NewURL(t) // already fully migrated
	env := func(k string) string {
		if k == "RECURIN_DATABASE_URL" {
			return dsn
		}
		return ""
	}
	migrate := func(action string, wantStdout string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(t.Context(), []string{"migrate", action}, env, &stdout, &stderr); code != exitOK {
			t.Fatalf("migrate %s: exit %d, stderr %q", action, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), wantStdout) {
			t.Fatalf("migrate %s: stdout %q, want %q", action, stdout.String(), wantStdout)
		}
	}

	migrate("status", "applied")
	migrate("down", "rolled back 00002_api_keys.sql")
	migrate("status", "pending")
	migrate("up", "applied 00002_api_keys.sql")
	migrate("up", "schema is up to date")
}

func TestMigrateReportsUnreachableDatabase(t *testing.T) {
	t.Parallel()
	env := func(k string) string {
		if k == "RECURIN_DATABASE_URL" {
			return "postgres://recurin:hunter2@127.0.0.1:1/recurin?sslmode=disable&connect_timeout=2"
		}
		return ""
	}
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"migrate", "status"}, env, &stdout, &stderr); code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr.String(), "cannot reach the database") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "hunter2") {
		t.Fatalf("stderr leaks the password: %q", stderr.String())
	}
}
