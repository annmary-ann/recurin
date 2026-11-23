# Architecture decision records

Each record captures one significant decision: the forces behind it, what was chosen, what was rejected and why, and what we accept as a result. Records are immutable once accepted; a later decision that changes course adds a new record that supersedes the old one.

| ADR | Decision | Status |
| --- | --- | --- |
| [0001](0001-modular-monolith.md) | A modular monolith with enforced module boundaries | Accepted |
| [0002](0002-postgres-as-store-queue-and-outbox.md) | Postgres as the store, the job queue and the outbox | Accepted |
| [0003](0003-money-as-int64-paise.md) | Money as int64 paise with one rounding point | Accepted |
| [0004](0004-spec-first-openapi.md) | Spec-first OpenAPI 3.1 | Accepted |
| [0005](0005-type-prefixed-uuidv7-ids.md) | Type-prefixed UUIDv7 IDs | Accepted |
| [0006](0006-bill-ahead-of-the-charge.md) | Bill ahead of the charge to meet the pre-debit rule | Accepted |
| [0007](0007-apache-2-license.md) | Apache License 2.0 | Accepted |

## Writing a new ADR

Copy [`template.md`](template.md) to the next number, open a PR with the `type: research` label, and link the issue that raised the question. Keep it short: a reviewer should be able to read it in five minutes.
