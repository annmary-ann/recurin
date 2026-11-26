# HTTP API

The API is spec-first ([ADR 0004](adr/0004-spec-first-openapi.md)). `api/openapi.yaml` (OpenAPI 3.1) is the contract; Go code is generated from it, and every request is checked against it.

## How a request flows

```
request
  -> Request-Id        new req_... ID, set on the response and in every error body
  -> Cache-Control     no-store on every response
  -> panic recovery    a panic becomes a 500 api_error, logged with the request ID
  -> OpenAPI validator finds the operation in the spec (404/405 if none), runs
                       authentication if the operation needs a key, then checks
                       parameters and body against the spec
  -> strict handler    internal/api/handlers.go, implementing the generated interface
```

Which operations need a key is decided by the spec, not by Go code: the top-level `security` requires `secretKey`, and public operations such as `/healthz` say `security: []`.

## Adding or changing an endpoint

1. Edit `api/openapi.yaml`. Reuse the shared components: `Error` responses, `Money`, `Metadata`, `ListMeta` and the `Limit` / `StartingAfter` / `EndingBefore` parameters.
2. Run `make gen` (in the dev container on Windows). This regenerates `internal/api/gen/api.gen.go`.
3. Implement the new method on `handlers`. It does not compile until the method exists with the generated request and response types.
4. Test through `api.New` like `internal/api/api_test.go` does. Its `call` helper validates every response against the spec, so a handler that returns an undocumented shape fails.
5. Run `make api-check`. CI runs the same check against the base branch and fails on breaking changes: a removed field, a newly required parameter, a narrowed enum, and so on. Additive changes pass.

## Errors

Every error has the same body:

```json
{
  "error": {
    "type": "authentication_error",
    "code": "api_key_missing",
    "message": "No API key provided. Send it as \"Authorization: Bearer sk_test_...\".",
    "request_id": "req_01jd8x3k5fe2s9v7q4w6ytbn0m"
  }
}
```

| `type` | Status | Example `code`s |
| --- | --- | --- |
| `invalid_request_error` | 400, 405 | `parameter_invalid`, `body_invalid`, `method_not_allowed` |
| `authentication_error` | 401 | `api_key_missing`, `api_key_invalid` |
| `permission_error` | 403 | |
| `not_found_error` | 404 | `route_not_found` |
| `conflict_error` | 409 | |
| `rate_limit_error` | 429 | |
| `api_error` | 5xx | `internal_error` |

Clients match on `type` and `code`; `message` is for people and may change. `param` names the offending field when there is one. Internal details are logged with the request ID and never sent to the client.

## API keys

Secret keys look like `sk_test_<52 characters>` or `sk_live_<52 characters>`. They carry 256 random bits and belong to one account. recurin stores only their SHA-256 hash plus a hint (`sk_test_...a1b2`), so a key cannot be shown again after it is created. A test key acts on test-mode data; responses carry `livemode` to say which mode served them.

Create and revoke keys with the binary (on the Compose stack, run it through the `migrate` service, which has the database URL):

```sh
recurin account create "Acme Learning"            # account plus its first test key
recurin apikey create acct_... live "prod server" # another key
recurin apikey revoke key_...

docker compose -f deploy/compose/docker-compose.yml run --rm --no-deps migrate account create "Acme Learning"
```

Then call the API:

```sh
curl -H "Authorization: Bearer sk_test_..." http://localhost:8086/v1/account
```
