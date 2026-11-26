-- Secret API keys. Only a SHA-256 hash of each secret is stored; the secret
-- is shown once, when the key is created (internal/platform/apikey).

-- +goose Up

create table api_keys (
    id           uuid        primary key,
    account_id   uuid        not null references accounts (id),
    mode         text        not null check (mode in ('test', 'live')),
    name         text        not null default '' check (length(name) <= 200),
    secret_hash  bytea       not null unique check (length(secret_hash) = 32),
    -- "sk_test_...a1b2": enough to recognise a key in a list, useless to an attacker.
    secret_hint  text        not null,
    revoked_at   timestamptz,
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    version      bigint      not null default 1
);

comment on table api_keys is
    'Secret API keys (sk_test_/sk_live_), stored as SHA-256 hashes.';

create index api_keys_account_id on api_keys (account_id);

create trigger api_keys_touch
    before update on api_keys
    for each row execute function recurin_touch();

-- +goose Down

drop table api_keys;
