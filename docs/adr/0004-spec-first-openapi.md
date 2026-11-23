# 0004. Spec-first OpenAPI 3.1

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

recurin's API is its product surface. Platform teams integrate once and then depend on it for years; a field that silently changes type breaks someone's billing. The same API definition has to drive several things: server request validation, generated TypeScript and Python SDKs, reference docs, and the admin console's client.

If the code is the source of truth and the spec is generated from it, the spec ends up describing whatever the code happens to do, and breaking changes slip through review because nobody reads generated YAML.

## Decision

`api/openapi.yaml` (OpenAPI 3.1) is written **first** and is the contract.

- Server interfaces and request/response types are generated with `oapi-codegen` and served with `chi`. Handlers implement the generated interface, so a handler that does not match the spec does not compile.
- Incoming requests are validated against the spec in middleware.
- SDKs, docs and the admin console client are generated from the same file.
- CI runs a breaking-change check (`oasdiff`) against `main`. A breaking change needs an explicit API version bump.
- Shared conventions live in the spec: cursor pagination, the error object (`type`, `code`, `message`, `param`), the money object, `metadata`, and the `Idempotency-Key` header.

## Alternatives considered

- **Code-first with annotations that generate a spec.** Faster for the first endpoint, but the contract becomes a side effect of implementation details, and reviewers see Go diffs, not API diffs.
- **gRPC / Protobuf.** Strong contracts and codegen, but platform teams integrating billing expect REST and JSON webhooks, and gRPC-web adds friction for browser-based admin tools.
- **GraphQL.** Flexible reads, but billing writes need strict idempotency and simple, cacheable semantics; it would also complicate webhook schemas.

## Consequences

- API changes are reviewed as spec diffs first, which is where API design discussion belongs.
- The server cannot drift from the documentation, and SDKs are always in sync.
- Spec-first is slower for exploratory endpoints; exploration happens on branches, and the spec is settled before merge.
- Generated code is committed and checked by the "generated code is up to date" CI job.
