#!/usr/bin/env sh
# Runs a command inside the recurin dev container (Go toolchain in Docker).
#
#   ./scripts/dev.sh go test ./...
#   ./scripts/dev.sh go run ./cmd/recurin --help
#   ./scripts/dev.sh                 # opens a shell in the container
#
# If Go is installed natively you can run commands directly instead; the
# container exists so every machine gets the same toolchain.
set -eu

repo="$(cd "$(dirname "$0")/.." && pwd)"
image="recurin-dev"

docker build --quiet --tag "$image" "$repo/.devcontainer" >/dev/null

tty="-i"
if [ -t 0 ] && [ -t 1 ]; then tty="-it"; fi

[ "$#" -eq 0 ] && set -- bash

exec docker run --rm $tty \
  --add-host=host.docker.internal:host-gateway \
  -v "$repo:/workspace" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v recurin-gomod:/go/pkg/mod \
  -v recurin-gocache:/root/.cache/go-build \
  -w /workspace \
  "$image" "$@"
