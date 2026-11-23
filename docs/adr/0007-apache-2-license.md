# 0007. Apache License 2.0

- **Status:** Accepted
- **Date:** 2026-11-23
- **Issue:** #6

## Context

recurin is meant to be adopted by companies, many of them small Indian SaaS and D2C teams, and embedded in their own deployments. Billing code also gets customised: new gateways, local tax quirks, internal integrations. The license decides whether legal teams approve it and whether contributors' patents are covered.

Comparable open-source billing projects split on this: some use AGPL-3.0 to stop cloud vendors from reselling them as a hosted service; others use permissive licenses to maximise adoption.

## Decision

recurin is licensed under the **Apache License 2.0**.

## Alternatives considered

- **AGPL-3.0.** Protects against a third party offering recurin as a hosted service without sharing changes. But many company policies ban AGPL dependencies outright, which would block the exact users recurin is for. There is no hosted business to protect.
- **MIT.** Equally permissive, but has no explicit patent grant. For software that handles payments, the patent grant and patent-retaliation clause in Apache-2.0 are worth having.
- **Business Source License or another source-available license.** Not open source; it would discourage contributions and adoption.

## Consequences

- Companies can adopt, modify and embed recurin without legal friction, including in proprietary products.
- Contributors grant a patent license for their contributions.
- Anyone, including a large vendor, may offer recurin as a hosted service. That is accepted.
- Redistributions must keep the `LICENSE` and `NOTICE` files.
- Contributions are accepted under the same license. A DCO or CLA can be added later if outside contributions grow.
