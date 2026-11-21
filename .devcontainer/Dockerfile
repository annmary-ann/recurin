# Development image for recurin: the Go toolchain plus the few system tools the
# project needs. Used by the devcontainer and by scripts/dev.ps1 / scripts/dev.sh.
FROM golang:1.27-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates git postgresql-client \
    && rm -rf /var/lib/apt/lists/*

# The repo is bind-mounted from the host, so its files belong to a different
# user. Without this, git (and Go's VCS stamping) refuse to read the repo.
RUN git config --system --add safe.directory '*'

# testcontainers-go starts sibling containers through the host's Docker socket;
# their published ports are reachable through the host, not localhost.
ENV TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal

WORKDIR /workspace
