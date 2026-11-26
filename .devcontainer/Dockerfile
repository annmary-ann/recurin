# Development image for recurin: the Go toolchain plus the tools the project
# needs. Used by the devcontainer and by scripts/dev.ps1 / scripts/dev.sh.
# CI installs the same tool versions, so bump them here and in CI together.
FROM golang:1.27-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git make postgresql-client \
    && rm -rf /var/lib/apt/lists/*

# Built with this image's Go, so the linter always understands the Go version
# the project targets.
ARG GOLANGCI_LINT_VERSION=v2.15.0
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION} \
    && rm -rf /root/.cache/go-build /go/pkg/mod

# sqlc generates Go from db/queries (make gen). The release binary avoids a
# slow cgo build of its Postgres parser.
ARG SQLC_VERSION=1.31.1
RUN arch="$(dpkg --print-architecture)" \
    && curl -fsSL "https://github.com/sqlc-dev/sqlc/releases/download/v${SQLC_VERSION}/sqlc_${SQLC_VERSION}_linux_${arch}.tar.gz" \
       | tar -xz -C /usr/local/bin sqlc \
    && sqlc version

# The repo is bind-mounted from the host, so its files belong to a different
# user. Without this, git (and Go's VCS stamping) refuse to read the repo.
RUN git config --system --add safe.directory '*'

# testcontainers-go starts sibling containers through the host's Docker socket;
# their published ports are reachable through the host, not localhost.
ENV TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal

WORKDIR /workspace
