package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/annmary-ann/recurin/internal/platform/postgres/pgtest"
)

// Tables exempt from a convention, with the reason. Keep these lists short.
var (
	// Tables the checks skip entirely.
	notOurs = map[string]string{
		"goose_db_version": "owned by the migration tool",
	}
	// Tables without account_id.
	noAccountID = map[string]string{
		"accounts": "it is the tenant table",
	}
)

// TestSchemaConventions checks the rules in docs/database.md against the
// migrated schema, so a new migration that forgets one fails CI.
func TestSchemaConventions(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t)
	ctx := t.Context()

	type column struct{ table, name, dataType, nullable string }
	rows, err := pool.Query(ctx, `
		select table_name, column_name, data_type, is_nullable
		from information_schema.columns
		where table_schema = 'public'`)
	if err != nil {
		t.Fatal(err)
	}
	cols, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (column, error) {
		var c column
		err := r.Scan(&c.table, &c.name, &c.dataType, &c.nullable)
		return c, err
	})
	if err != nil {
		t.Fatal(err)
	}
	byTable := map[string]map[string]column{}
	for _, c := range cols {
		if byTable[c.table] == nil {
			byTable[c.table] = map[string]column{}
		}
		byTable[c.table][c.name] = c
	}

	touched := map[string]bool{}
	trows, err := pool.Query(ctx, `
		select c.relname
		from pg_trigger tg
		join pg_class c on c.oid = tg.tgrelid
		join pg_proc p on p.oid = tg.tgfoid
		where p.proname = 'recurin_touch' and not tg.tgisinternal`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(trows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		touched[n] = true
	}

	want := func(table, col, dataType string) {
		t.Helper()
		c, ok := byTable[table][col]
		switch {
		case !ok:
			t.Errorf("%s: missing column %s", table, col)
		case c.dataType != dataType:
			t.Errorf("%s.%s: type %s, want %s", table, col, c.dataType, dataType)
		case c.nullable != "NO":
			t.Errorf("%s.%s: must be NOT NULL", table, col)
		}
	}

	if _, ok := byTable["accounts"]; !ok {
		t.Fatal("accounts table not found; did the migrations run?")
	}
	for table, columns := range byTable {
		if _, ok := notOurs[table]; ok {
			continue
		}
		if _, ok := noAccountID[table]; !ok {
			want(table, "account_id", "uuid")
		}
		want(table, "created_at", "timestamp with time zone")
		if _, ok := columns["id"]; ok {
			want(table, "id", "uuid")
		}
		// Mutable tables have updated_at and version together, kept by the trigger.
		_, hasUpdated := columns["updated_at"]
		_, hasVersion := columns["version"]
		if hasUpdated || hasVersion {
			want(table, "updated_at", "timestamp with time zone")
			want(table, "version", "bigint")
			if !touched[table] {
				t.Errorf("%s: has updated_at/version but no recurin_touch trigger", table)
			}
		}
		if _, ok := columns["metadata"]; ok {
			want(table, "metadata", "jsonb")
		}
		for name, c := range columns {
			if c.dataType == "real" || c.dataType == "double precision" {
				t.Errorf("%s.%s: floating-point column; money is bigint paise (ADR 0003)", table, name)
			}
			if c.dataType == "timestamp without time zone" {
				t.Errorf("%s.%s: use timestamptz", table, name)
			}
		}
	}
}
