# Tasks for recurin. On Windows (or anywhere without Go), run them in the dev
# container:  ./scripts/dev.ps1 make test
.DEFAULT_GOAL := help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help
help: ## Show the available tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the recurin binary into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/recurin ./cmd/recurin

.PHONY: test
test: ## Run all tests with the race detector
	go test -race -count=1 ./...

.PHONY: cover
cover: ## Run tests with a coverage report in coverage.out
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

# Postgres from `make up`. In the dev container the host is host.docker.internal.
COMPOSE_TEST_DB ?= postgres://recurin:recurin@$(or $(TESTCONTAINERS_HOST_OVERRIDE),localhost):$(or $(RECURIN_DB_PORT),5436)/postgres?sslmode=disable

.PHONY: test-db
test-db: ## Run tests against the Compose Postgres (make up first); much faster than testcontainers
	RECURIN_TEST_DATABASE_URL='$(COMPOSE_TEST_DB)' go test -race -count=1 ./...

.PHONY: test-unit
test-unit: ## Run only the tests that need no database (go test -short)
	go test -race -count=1 -short ./...

.PHONY: lint
lint: ## Run golangci-lint (linters and format checks)
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format the code with gofumpt and goimports
	golangci-lint fmt ./...

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	go mod tidy

.PHONY: gen
gen: ## Run code generators (sqlc, oapi-codegen) once they exist
	go generate ./...

.PHONY: check
check: lint test ## Lint and test: what CI runs

COMPOSE := docker compose -f deploy/compose/docker-compose.yml

.PHONY: up
up: ## Start the dev dependencies (Postgres, Gotenberg, Mailpit, Jaeger)
	$(COMPOSE) up -d --wait

.PHONY: up-app
up-app: ## Start the dependencies plus recurin (migrate, then api on :8086)
	$(COMPOSE) --profile app up -d --wait --build

.PHONY: down
down: ## Stop the dev stack (data stays in the pgdata volume)
	$(COMPOSE) --profile app down

.PHONY: image
image: ## Build the production image
	docker build --build-arg VERSION=$(VERSION) -t recurin:$(VERSION) .

.PHONY: clean
clean: ## Remove build output
	rm -rf bin coverage.out
