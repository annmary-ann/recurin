-- name: CreateAccount :one
insert into accounts (id, name, metadata)
values (@id, @name, @metadata)
returning *;

-- name: GetAccount :one
select * from accounts
where id = @id;

-- RenameAccount updates only if the caller saw the current version; no row
-- back means someone else changed the account first.
-- name: RenameAccount :one
update accounts
set name = @name
where id = @id and version = @version
returning *;
