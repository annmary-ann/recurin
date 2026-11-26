package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/annmary-ann/recurin/internal/platform/apikey"
	"github.com/annmary-ann/recurin/internal/platform/id"
	"github.com/annmary-ann/recurin/internal/platform/platformdb"
	"github.com/annmary-ann/recurin/internal/platform/postgres"
	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

func TestAdminArguments(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"account without name":   {"account", "create"},
		"account unknown action": {"account", "delete", "x"},
		"apikey no action":       {"apikey"},
		"apikey bad account id":  {"apikey", "create", "cus_x", "test"},
		"apikey bad mode":        {"apikey", "create", id.New[id.Account]().String(), "prod"},
		"apikey revoke bad id":   {"apikey", "revoke", "acct_x"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if code := run(t.Context(), args, noEnv, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit %d, want %d; stderr %q", code, exitUsage, stderr.String())
			}
		})
	}
}

var (
	acctRe = regexp.MustCompile(`acct_[0-9a-z]{26}`)
	keyRe  = regexp.MustCompile(`key_[0-9a-z]{26}`)
	skRe   = regexp.MustCompile(`sk_(test|live)_[0-9a-z]{52}`)
)

func TestAdminCreatesAccountAndKeys(t *testing.T) {
	t.Parallel()
	dsn := pgtest.NewURL(t)
	env := func(k string) string {
		if k == "RECURIN_DATABASE_URL" {
			return dsn
		}
		return ""
	}
	cli := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(t.Context(), args, env, &stdout, &stderr); code != exitOK {
			t.Fatalf("%v: exit %d, stderr %q", args, code, stderr.String())
		}
		return stdout.String()
	}

	out := cli("account", "create", "Acme Learning")
	acct, testKey := acctRe.FindString(out), skRe.FindString(out)
	if acct == "" || !strings.HasPrefix(testKey, "sk_test_") || !strings.Contains(out, "cannot show it again") {
		t.Fatalf("account create output:\n%s", out)
	}

	out = cli("apikey", "create", acct, "live", "production server")
	keyID, liveKey := keyRe.FindString(out), skRe.FindString(out)
	if keyID == "" || !strings.HasPrefix(liveKey, "sk_live_") {
		t.Fatalf("apikey create output:\n%s", out)
	}

	// Both keys authenticate, to the right account and mode.
	pool, err := postgres.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := platformdb.New(pool)
	for _, key := range []string{testKey, liveKey} {
		mode, hash, err := apikey.Parse(key)
		if err != nil {
			t.Fatal(err)
		}
		row, err := q.AuthenticateAPIKey(t.Context(), hash[:])
		if err != nil || row.AccountID.String() != acct || row.Mode != string(mode) {
			t.Fatalf("authenticate %s...: %+v, %v", key[:8], row, err)
		}
	}

	out = cli("apikey", "revoke", keyID)
	if !strings.Contains(out, "revoked") || !strings.Contains(out, "sk_live_...") || strings.Contains(out, liveKey) {
		t.Fatalf("revoke output:\n%s", out)
	}
	_, hash, _ := apikey.Parse(liveKey)
	if _, err := q.AuthenticateAPIKey(t.Context(), hash[:]); err == nil {
		t.Fatal("revoked key still authenticates")
	}

	// Revoking twice is an error, not a silent success.
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"apikey", "revoke", keyID}, env, &stdout, &stderr); code != exitError {
		t.Fatalf("second revoke exit %d, stderr %q", code, stderr.String())
	}
}
