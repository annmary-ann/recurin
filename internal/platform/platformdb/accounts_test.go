package platformdb_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/annmary-ann/recurin/internal/platform/id"
	"github.com/annmary-ann/recurin/internal/platform/platformdb"
	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

func TestAccountLifecycle(t *testing.T) {
	t.Parallel()
	q := platformdb.New(pgtest.New(t))
	ctx := t.Context()

	created, err := q.CreateAccount(ctx, platformdb.CreateAccountParams{
		ID:       id.New[id.Account](),
		Name:     "Acme Learning",
		Metadata: json.RawMessage(`{"plan":"pilot"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.CreatedAt.IsZero() || !created.UpdatedAt.Equal(created.CreatedAt) {
		t.Fatalf("new account: version %d, created %v, updated %v", created.Version, created.CreatedAt, created.UpdatedAt)
	}

	got, err := q.GetAccount(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Name != "Acme Learning" {
		t.Fatalf("GetAccount = %+v", got)
	}
	var meta map[string]string
	if err := json.Unmarshal(got.Metadata, &meta); err != nil || meta["plan"] != "pilot" {
		t.Fatalf("metadata = %s, %v", got.Metadata, err)
	}

	renamed, err := q.RenameAccount(ctx, platformdb.RenameAccountParams{ID: created.ID, Name: "Acme", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Version != 2 || renamed.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("after rename: version %d, updated %v; the trigger should bump both", renamed.Version, renamed.UpdatedAt)
	}

	// A writer holding the old version loses: no row comes back.
	_, err = q.RenameAccount(ctx, platformdb.RenameAccountParams{ID: created.ID, Name: "Stale", Version: 1})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("stale rename error = %v, want pgx.ErrNoRows", err)
	}

	if _, err := q.GetAccount(ctx, id.New[id.Account]()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetAccount(unknown) error = %v, want pgx.ErrNoRows", err)
	}
}

func TestAccountConstraints(t *testing.T) {
	t.Parallel()
	q := platformdb.New(pgtest.New(t))
	ctx := t.Context()

	cases := map[string]platformdb.CreateAccountParams{
		"empty name":           {ID: id.New[id.Account](), Name: "", Metadata: json.RawMessage(`{}`)},
		"metadata not object":  {ID: id.New[id.Account](), Name: "x", Metadata: json.RawMessage(`[1,2]`)},
		"metadata JSON scalar": {ID: id.New[id.Account](), Name: "x", Metadata: json.RawMessage(`"x"`)},
	}
	for name, params := range cases {
		_, err := q.CreateAccount(ctx, params)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" { // check_violation
			t.Errorf("%s: error = %v, want a check violation", name, err)
		}
	}
}

// Each pgtest database is separate: rows written in one test are invisible to
// another running at the same time.
func TestDatabasesAreIsolated(t *testing.T) {
	t.Parallel()
	a := platformdb.New(pgtest.New(t))
	b := platformdb.New(pgtest.New(t))
	acct, err := a.CreateAccount(t.Context(), platformdb.CreateAccountParams{
		ID: id.New[id.Account](), Name: "only in a", Metadata: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetAccount(t.Context(), acct.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("account leaked into another test database: %v", err)
	}
}
