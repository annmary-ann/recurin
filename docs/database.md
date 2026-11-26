# Database

recurin stores everything in one Postgres 16 database ([ADR 0002](adr/0002-postgres-as-store-queue-and-outbox.md)). This page covers how the schema changes, how code talks to it, and how tests get a real database.

| Piece | Where | Tool |
| --- | --- | --- |
| Migrations | `db/migrations/NNNNN_description.sql` | [goose](https://github.com/pressly/goose), embedded in the binary |
| Queries | `db/queries/<module>/*.sql` | [sqlc](https://sqlc.dev) generates Go into `internal/<module>/<module>db` |
| Driver | `internal/platform/postgres` | [pgx v5](https://github.com/jackc/pgx) pool |
| Test databases | `internal/platform/postgres/pgtest` | testcontainers-go, or an existing server |

## Running migrations

```sh
RECURIN_DATABASE_URL=postgres://... recurin migrate          # apply pending (same as "migrate up")
RECURIN_DATABASE_URL=postgres://... recurin migrate status   # list migrations and their state
RECURIN_DATABASE_URL=postgres://... recurin migrate down     # roll back the latest one
```

The Compose `app` profile runs `recurin migrate` once before the API starts.

## Adding a migration

1. Create `db/migrations/NNNNN_what_it_does.sql` with the next number (`00002_customers.sql`). Numbers are sequential, not timestamps, so the order is obvious in review.
2. Write both sections. `-- +goose Up` and `-- +goose Down` are required; Down must undo Up exactly, because `TestMigrationsRoundTrip` rolls every migration back and re-applies it.
3. Wrap functions and other statements that contain semicolons in `-- +goose StatementBegin` / `-- +goose StatementEnd`.
4. Never edit a migration that has been merged to `main`. Fix forward with a new one.

Each migration runs in its own transaction. A migration that cannot run in a transaction (for example `create index concurrently`) starts with `-- +goose NO TRANSACTION`.

## Table conventions

`TestSchemaConventions` checks these against the migrated schema, so a migration that forgets one fails CI.

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `uuid primary key` | A UUIDv7 made by the application, exposed as a prefixed ID such as `cus_...` ([ADR 0005](adr/0005-type-prefixed-uuidv7-ids.md)) |
| `account_id` | `uuid not null references accounts (id)` | The tenant. On every table except `accounts`, so row-level security can be enabled later |
| `created_at` | `timestamptz not null default now()` | On every table |
| `updated_at`, `version` | `timestamptz`, `bigint not null default 1` | On mutable tables only, always together, kept by the `recurin_touch()` trigger |
| `metadata` | `jsonb not null default '{}'`, checked to be an object | On customer-facing objects |
| money | `<name>_paise bigint` + `currency char(3)` | Never `real`, `double precision` or `numeric` ([ADR 0003](adr/0003-money-as-int64-paise.md)) |

A mutable table gets the trigger like this:

```sql
create trigger customers_touch
    before update on customers
    for each row execute function recurin_touch();
```

Append-only tables (postings, `*_events`, the outbox) have no `updated_at` or `version`.

**Optimistic locking.** Updates name the version the caller read: `update ... where id = @id and version = @version returning *`. No row back means someone else changed it first; the caller re-reads and decides. The trigger bumps `version`, so application code never sets it.

**Times.** `created_at` and `updated_at` are audit times from the database clock. Business times (billing periods, due dates, notice deadlines) come from the application's `clock.Clock` and are stored in their own columns, so test clocks control them.

## Writing queries

1. Add SQL to `db/queries/<module>/<table>.sql` with a `-- name: CreateThing :one` header. Use named parameters (`@id`) for readable generated code.
2. A new module gets an entry in `sqlc.yaml` (copy the `platform` one) and its own `<module>db` package.
3. Map ID columns to typed IDs with a column override in `sqlc.yaml` (`customers.id` → `id.CustomerID`).
4. Run `make gen` and commit the generated files. CI fails if they are stale.

A module queries only its own tables (see [architecture](architecture.md#module-rules)).

## Tests against Postgres

```go
func TestSomething(t *testing.T) {
	t.Parallel()
	pool := pgtest.New(t) // fresh, migrated database; dropped when the test ends
	q := customersdb.New(pool)
	...
}
```

`pgtest` builds a template database with every migration applied, once per server, and clones it for each test (`create database ... template ...`), which takes milliseconds. Tests never share rows, so they can all run in parallel. The template name includes a hash of the migrations, so editing a migration rebuilds it.

Where the server comes from:

- **Default:** a `postgres:16-alpine` container per test binary, started by testcontainers. Needs Docker. CI uses this.
- **Faster locally:** `make up`, then `make test-db`. This points `RECURIN_TEST_DATABASE_URL` at the Compose Postgres, and the database tests take about 2 seconds instead of about 15.
- **No database:** `make test-unit` runs `go test -short`, which skips every test that calls `pgtest`.
