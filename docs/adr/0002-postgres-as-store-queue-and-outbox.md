# 0002. Postgres as the store, the job queue and the outbox

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

Billing is full of "change state, then do something later" work: send a pre-debit notice 48 hours before renewal, charge after the notice window, retry a failed payment in three days, deliver a webhook with backoff. Each of these must happen **if and only if** the state change that caused it committed.

The classic failure is the dual write: commit to the database, then publish to a queue. If the process dies between the two, a renewal is recorded but no charge is ever scheduled, or a webhook announces an invoice that rolled back.

recurin's expected scale is thousands to low millions of subscriptions per installation, a few hundred API requests per second, and renewals clustered around billing boundaries. That is well within what one Postgres instance handles.

## Decision

Postgres does three jobs:

1. **System of record** for all module data and the ledger.
2. **Job queue**, using [River](https://riverqueue.com). Jobs are inserted with `InsertTx` in the same transaction as the state change. Workers claim jobs with `SELECT ... FOR UPDATE SKIP LOCKED`, so many workers never pick the same job.
3. **Transactional outbox** for domain events. An event row is written in the same transaction as the change. A relay job publishes it to in-process handlers and to outbound webhook delivery after commit.

There is no Kafka, RabbitMQ or Redis in v1.

## Alternatives considered

- **Kafka or RabbitMQ with an outbox relay.** Durable and fast, but it adds a second stateful system for self-hosters to run and back up, and it still needs the outbox to avoid dual writes. The throughput it adds is not needed at our scale.
- **Redis-backed queues (Asynq, Sidekiq-style).** Simple to use, but jobs cannot be enqueued atomically with Postgres state, which is exactly the guarantee we need.
- **Temporal for durable workflows.** Excellent for long, multi-step processes, but it is a large dependency for teams to operate. Worth revisiting if dunning and mandate flows become workflow-heavy.
- **Polling tables with ad-hoc cron.** What River does, minus retries, backoff, uniqueness, observability and a tested implementation.

## Consequences

- A rolled-back transaction leaves no job and no event behind. This is tested directly (#11).
- Operators run and back up exactly one stateful service.
- Queue throughput is bounded by Postgres. Job and outbox tables need retention and vacuum tuning, which the Helm chart and docs must cover.
- At-least-once delivery remains: a job can run twice if a worker dies after doing the work but before marking it complete. Every job is therefore idempotent, keyed on business identity (for example `(subscription_id, period_start)`), not on the job ID.
- If a future installation outgrows this, the outbox is the seam: its relay can publish to Kafka without changing any module.
