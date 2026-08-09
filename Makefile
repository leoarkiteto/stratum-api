# Stratum Backend — development workflow.
# See README.md for setup instructions and REASONIX.md for conventions.

GO      ?= go
COMPOSE ?= docker compose

.PHONY: help build run test vet lint fmt fmt-check tidy check clean db-up db-down

.DEFAULT_GOAL := help

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Compile-check all packages (no artifacts)
	go build ./...

run: ## Run the API (applies pending migrations at startup)
	go run ./cmd/server

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run staticcheck (install with: go install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...

fmt: ## Format all Go files in place
	gofmt -w .

fmt-check: ## Fail if any Go file is not gofmt-formatted
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed for:"; gofmt -l .; exit 1; }

tidy: ## Tidy go.mod / go.sum
	go mod tidy

check: fmt-check vet test ## Format check + vet + tests

clean: ## Remove build cache artifacts
	go clean ./...

db-up: ## Start Postgres 16 (docker compose, matches .env.example)
	$(COMPOSE) up -d

db-down: ## Stop the Postgres container
	$(COMPOSE) down
