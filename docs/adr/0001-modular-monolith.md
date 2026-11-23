# 0001. A modular monolith with enforced module boundaries

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

recurin is self-hosted. The people running it are platform teams whose main job is their own product, not operating a billing system. Every extra deployable means another thing to monitor, upgrade and debug at 2 a.m.

At the same time, billing has clear sub-domains (catalog, subscriptions, invoicing, tax, ledger, payments, mandates, dunning, webhooks) with different rates of change. Gateway adapters change whenever Razorpay changes an API; the ledger should almost never change. Without boundaries, a quick fix in one area quietly couples it to another, and a billing codebase becomes impossible to reason about.

Most money-moving operations also span several sub-domains. Finalising a renewal touches subscriptions, invoices, tax, the ledger and webhooks, and it must happen atomically.

## Decision

recurin is **one Go binary** with **one Postgres database**, split internally into modules under `internal/<module>`.

- A module owns its tables. No other module reads or writes them.
- Modules call each other only through the Go interfaces a module exports. CI enforces this with an import-boundary lint (#13).
- Modules react to each other's changes through **domain events** written to an outbox in the same transaction (see [0002](0002-postgres-as-store-queue-and-outbox.md)).
- The binary runs in one of three **roles**: `api`, `worker` or `migrate`. Teams scale `api` and `worker` independently.

## Alternatives considered

- **Microservices per sub-domain.** Real isolation and independent deploys, but a renewal would become a distributed transaction (sagas, compensations, idempotent consumers on every hop) for a system whose load fits comfortably on one database. The operational cost lands on self-hosters who did not ask for it.
- **A plain monolith without boundaries.** Fastest to start, but billing logic tangles quickly, and there is no path to splitting anything out later.
- **Separate API and worker codebases.** They share almost all domain code; splitting them duplicates it or forces a shared library with the same coupling problems.

## Consequences

- Cross-module operations are ordinary database transactions, which keeps money operations atomic and simple.
- One artifact to build, sign, deploy and upgrade.
- Boundaries hold only because CI enforces them; the lint in #13 is mandatory, not optional.
- If a module ever needs to become its own service, its table ownership and event contracts already form the seam to cut along.
- The whole system shares one database's capacity. Read replicas and partitioning are the scaling path, and the load test (#59) has to show where the limits are.
