-- name: CreateAPIKey :one
insert into api_keys (id, account_id, mode, name, secret_hash, secret_hint)
values (@id, @account_id, @mode, @name, @secret_hash, @secret_hint)
returning *;

-- AuthenticateAPIKey finds the live (unrevoked) key with this secret hash.
-- name: AuthenticateAPIKey :one
select id, account_id, mode
from api_keys
where secret_hash = @secret_hash and revoked_at is null;

-- name: RevokeAPIKey :one
update api_keys
set revoked_at = now()
where id = @id and revoked_at is null
returning *;
