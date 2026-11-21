# Contributing to recurin

Thanks for your interest. recurin handles other people's money, so the bar for correctness is high, but the process is simple.

## Workflow

1. Every change starts from an issue. Plan and progress live on the [project board](https://github.com/users/annmary-ann/projects/9). For anything larger than a small fix, comment on the issue before you start.
2. Branch from `main`: `feat/<issue>-<short-slug>` (for example `feat/22-billing-tick`). Use `fix/` for bugs and `docs/` for documentation only.
3. Open a pull request that says `Closes #<issue>` and fill in the template.
4. CI must pass. Pull requests are squash-merged and the branch is deleted after merge. Direct pushes to `main` are blocked.

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/). The squash-merge title becomes the commit on `main`, and the changelog is generated from it.

```
feat(billing): renew due subscriptions from a periodic tick
fix(tax): use IGST when the customer's state differs from the seller's
test(ledger): property test that postings always sum to zero
docs(adr): record why invoices are created before the charge
chore(ci): cache Go modules
```

Scopes match the modules: `catalog`, `customers`, `subscriptions`, `usage`, `billing`, `tax`, `invoices`, `ledger`, `payments`, `mandates`, `dunning`, `webhooks`, `platform`, `api`, plus `ci`, `infra` and `docs`.

## Rules for money code

These rules apply to any code that touches amounts, taxes or the ledger. Reviews check them.

- Amounts are `money.Amount` (integer paise plus currency). Never use `float32`, `float64` or `numeric` for money.
- Round in one place, half-even, using the helpers in `internal/platform/money`.
- Get the time from the injected `clock.Clock`, never from `time.Now()`.
- A state change, its ledger entries, its outbox events and its follow-up jobs commit in one transaction.
- New behaviour comes with tests. Money and tax logic also gets property tests. A bug fix starts with a test that fails.

## Compliance rules

If you find that recurin gets an RBI, NPCI, GST or DPDP rule wrong, open a **Compliance rule** issue with a link to the official circular or notification. We change the code only when there is a primary source.

## Development setup

You need **Docker** (Docker Desktop on Windows and macOS) and **git**. The Go toolchain runs in a container, so every machine and CI use the same Go version (currently 1.27) and nothing has to be installed on the host.

Run any Go command through the dev script:

```sh
./scripts/dev.sh go test ./...         # macOS / Linux / WSL
./scripts/dev.ps1 go test ./...        # Windows PowerShell
./scripts/dev.sh                       # a shell inside the container
```

Common tasks live in the `Makefile` (`make help` lists them):

```sh
./scripts/dev.ps1 make check     # lint + tests, the same as CI
./scripts/dev.ps1 make fmt       # format with gofumpt and goimports
./scripts/dev.ps1 make build     # build ./bin/recurin
```

The first run builds the image from `.devcontainer/Dockerfile`; later runs reuse it, and Go's module and build caches persist in Docker volumes. The container gets the host's Docker socket, so integration tests can start Postgres with testcontainers.

**Editors.** VS Code and JetBrains can open the repo in the devcontainer (`.devcontainer/devcontainer.json`) for completion and debugging inside the same image.

**Native Go is optional.** If Go 1.27 runs natively on your machine, you can call `go` directly. Some Windows machines with Smart App Control turned on block `go.exe` and the test binaries it builds; the container sidesteps that without weakening the policy.

The `docker compose` dev stack (Postgres, Gotenberg, Mailpit, Jaeger) arrives with issue #5.

## Code of conduct

By taking part you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
