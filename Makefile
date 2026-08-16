# Stratum — GOTTH (Go + Templ + TailwindCSS + HTMX) development workflow.
# See README.md for setup instructions and REASONIX.md for conventions.

GO               ?= go
TEMPL            ?= templ
TAILWIND_VERSION ?= v4.3.3

.PHONY: help deps templ css build run test vet fmt fmt-check tidy check clean db-up db-down

.DEFAULT_GOAL := help

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

deps: bin/tailwindcss ## Install local build tools (standalone tailwindcss)
	@command -v templ >/dev/null 2>&1 || { \
		echo "templ not found — install with: go install github.com/a-h/templ/cmd/templ@latest"; \
		exit 1; \
	}

bin/tailwindcss: ## Download the standalone TailwindCSS CLI (pinned version)
	@mkdir -p bin
	@case "$$(uname -s)/$$(uname -m)" in \
	  Darwin/arm64)  asset=tailwindcss-macos-arm64 ;; \
	  Darwin/x86_64) asset=tailwindcss-macos-x64 ;; \
	  Linux/arm64)   asset=tailwindcss-linux-arm64 ;; \
	  Linux/x86_64)  asset=tailwindcss-linux-x64 ;; \
	  *) echo "unsupported platform for standalone tailwindcss"; exit 1 ;; \
	esac; \
	curl -fsSL -o $@ "https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/$$asset" && chmod +x $@

templ: ## Generate Go code from .templ templates
	@command -v templ >/dev/null 2>&1 || { \
		echo "templ not found — install with: go install github.com/a-h/templ/cmd/templ@latest"; \
		exit 1; \
	}
	$(TEMPL) generate

css: bin/tailwindcss ## Compile TailwindCSS into static/css/app.css
	./bin/tailwindcss -i assets/input.css -o static/css/app.css --minify

build: templ css ## Generate templates, build CSS, compile all packages
	go build ./...

run: templ css ## Run the app (applies pending migrations at startup)
	go run ./cmd/server

test: templ ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files in place
	gofmt -w .

fmt-check: ## Fail if any Go file is not gofmt-formatted
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed for:"; gofmt -l .; exit 1; }

tidy: ## Tidy go.mod / go.sum
	go mod tidy

check: fmt-check vet test ## Format check + vet + tests

clean: ## Remove build cache artifacts
	go clean ./...
	rm -rf bin

db-up: ## Start Postgres 16 (docker compose, matches .env.example)
	docker compose up -d

db-down: ## Stop the Postgres container
	docker compose down
