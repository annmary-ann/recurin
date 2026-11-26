-- Baseline: the tenant table and the helpers every later table uses.
--
-- Conventions (docs/database.md), checked by TestSchemaConventions:
--   id          uuid primary key; a UUIDv7 made by the application (ADR 0005)
--   account_id  uuid not null references accounts (id): the tenant, on every
--               row so row-level security can be turned on later
--   created_at  timestamptz not null default now()
--   updated_at  timestamptz, on mutable tables only; kept by recurin_touch()
--   version     bigint, on mutable tables only; optimistic locking, bumped by
--               recurin_touch(); updates say "where id = $1 and version = $2"
--   metadata    jsonb object, on customer-facing rows
-- Money columns are <name>_paise bigint plus currency char(3) (ADR 0003).
-- Business times (billing periods, due dates) come from the application's
-- clock; created_at and updated_at are audit times from the database.

-- +goose Up

-- +goose StatementBegin
create function recurin_touch() returns trigger
language plpgsql as $$
begin
    new.updated_at := now();
    new.version := old.version + 1;
    return new;
end;
$$;
-- +goose StatementEnd

comment on function recurin_touch() is
    'BEFORE UPDATE trigger for mutable tables: sets updated_at and bumps version.';

create table accounts (
    id          uuid        primary key,
    name        text        not null check (length(name) between 1 and 200),
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    version     bigint      not null default 1,
    metadata    jsonb       not null default '{}' check (jsonb_typeof(metadata) = 'object')
);

comment on table accounts is
    'Tenants. Every other table carries account_id referencing this one.';

create trigger accounts_touch
    before update on accounts
    for each row execute function recurin_touch();

-- +goose Down

drop table accounts;
drop function recurin_touch();
