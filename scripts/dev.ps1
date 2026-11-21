# Runs a command inside the recurin dev container (Go toolchain in Docker).
#
#   ./scripts/dev.ps1 go test ./...
#   ./scripts/dev.ps1 go run ./cmd/recurin --help
#   ./scripts/dev.ps1                 # opens a shell in the container
#
# The image is rebuilt only when .devcontainer/Dockerfile changes (Docker caches it).
# Go's module and build caches live in named volumes, so repeat runs are fast.

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$image = 'recurin-dev'

docker build --quiet --tag $image (Join-Path $repo '.devcontainer') | Out-Null
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$tty = @('-i')
if (-not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected) { $tty = @('-it') }

$cmd = $args
if ($cmd.Count -eq 0) { $cmd = @('bash') }

docker run --rm @tty `
    --add-host=host.docker.internal:host-gateway `
    -v "${repo}:/workspace" `
    -v /var/run/docker.sock:/var/run/docker.sock `
    -v recurin-gomod:/go/pkg/mod `
    -v recurin-gocache:/root/.cache/go-build `
    -w /workspace `
    $image @cmd
exit $LASTEXITCODE
