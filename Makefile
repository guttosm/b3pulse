# ───────────────────────────────────────────────────────────────────────────────
# Project settings
# ───────────────────────────────────────────────────────────────────────────────
SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

APP_NAME            ?= b3pulse
GO                  ?= go
GOLANGCI_LINT       ?= golangci-lint
SWAG                ?= github.com/swaggo/swag/cmd/swag
SWAG_BIN            := $(shell $(GO) env GOPATH)/bin/swag

# Exclude packages (regex passed to grep -Ev). Adjust as needed.
# Exclude generated or data-only packages from coverage calculations
EXCLUDE_PKGS_REGEX ?= internal/domain/models|internal/domain/dto|github.com/guttosm/b3pulse/docs$

# All packages except excluded ones
PKGS := $(shell $(GO) list ./... | grep -Ev '$(EXCLUDE_PKGS_REGEX)')

# Integration packages (respect exclusions too)
PKGS_INT := $(PKGS)

# Common test flags
TEST_FLAGS       ?= -race -shuffle=on -count=1
COVER_PROFILE    ?= coverage.out
COVER_MODE       ?= atomic
MIN_COVERAGE     ?= 60.0

# DB defaults (env-overridable, used by migrate target)
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_USER ?= postgres
DB_PASS ?= postgres
DB_NAME ?= b3pulse
DB_SSL  ?= disable

# ───────────────────────────────────────────────────────────────────────────────
# Helpers
# ───────────────────────────────────────────────────────────────────────────────
define print-target
	@printf "  \033[36m%-20s\033[0m %s\n" "$(1)" "$(2)"
endef

help: ## Show this help
	@echo "Make targets for $(APP_NAME):"
	@echo
	$(call print-target,setup,             Create local folders for bind mounts)
	$(call print-target,install,           Download deps & install tools)
	$(call print-target,run-api,           Run API locally (auto-gen Swagger))
	$(call print-target,ingest,            Run ingestion locally (no Docker))
	$(call print-target,build,             Build binary (auto-gen Swagger))
	$(call print-target,fmt,               go fmt)
	$(call print-target,tidy,              go mod tidy)
	$(call print-target,lint,              Run golangci-lint)
	$(call print-target,swagger,           Generate Swagger docs)
	$(call print-target,test,              Run ALL tests (unit + integration))
	$(call print-target,test-unit,         Run unit tests only, exclude $(EXCLUDE_PKGS_REGEX))
	$(call print-target,test-integration,  Run integration tests (tag: integration))
	$(call print-target,coverage,          Print coverage summary)
	$(call print-target,coverage-html,     Open HTML coverage report)
	$(call print-target,migrate,           Run DB migrations locally via Goose container)
	$(call print-target,docker-build,      Docker Compose build images (auto-gen Swagger))
	$(call print-target,docker-up,         Docker Compose up (Swagger + build, API only))
	$(call print-target,docker-api-up,     Start Postgres, run migrations, and start API)
	$(call print-target,docker-ingest,     Run ingestion once via Compose (uses profiles))
	$(call print-target,docker-down,       Docker Compose down)
	$(call print-target,docker-restart,    Restart Compose stack (DB, migrations, API))
	$(call print-target,clean,             Clean artifacts)
	$(call print-target,vet,               Run go vet static analysis)
	$(call print-target,staticcheck,       Run staticcheck static analysis)
	$(call print-target,analyze,           Run all static analysis tools)
	@echo

# ───────────────────────────────────────────────────────────────────────────────
# Setup / Dev
# ───────────────────────────────────────────────────────────────────────────────
setup: ## Create local folders for bind mounts
	mkdir -p postgres-data data/input migrations

install: ## Download deps & install tools
	$(GO) mod download
	$(GO) install $(SWAG)@latest

run-api: swagger ## Run API locally (Swagger auto-generated)
	$(GO) run ./cmd/main.go --mode=api --port=8080

ingest: ## Run ingestion locally (requires files in ./data/input)
	$(GO) run ./cmd/main.go --mode=ingest --dir=./data/input

build: swagger ## Build Go binary (Swagger auto-generated)
	@echo "Building $(APP_NAME)..."
	$(GO) build -o $(APP_NAME) ./cmd/main.go

fmt: ## Format code
	$(GO) fmt ./...

tidy: ## Tidy go.mod/go.sum
	$(GO) mod tidy

lint: ## Run static analysis (golangci-lint)
	$(GOLANGCI_LINT) run

swagger: ## Generate Swagger docs
	$(GO) install $(SWAG)@latest
	$(SWAG_BIN) init -g ./cmd/main.go --parseDependency --parseInternal

# ───────────────────────────────────────────────────────────────────────────────
# Testing
# ───────────────────────────────────────────────────────────────────────────────
test: ## Run ALL tests (unit + integration)
	@echo "→ Unit tests"
	$(GO) test $(PKGS) $(TEST_FLAGS) -coverprofile=$(COVER_PROFILE) -covermode=$(COVER_MODE)
	@echo "→ Integration tests"
	$(GO) test -tags=integration $(PKGS_INT) -count=1

test-unit: ## Run ONLY unit tests, exclude $(EXCLUDE_PKGS_REGEX)
	@echo "→ Unit tests (excluding: $(EXCLUDE_PKGS_REGEX))"
	$(GO) test $(PKGS) $(TEST_FLAGS) -coverprofile=$(COVER_PROFILE) -covermode=$(COVER_MODE)

test-integration: ## Run ONLY integration tests (tag: integration)
	@echo "→ Integration tests"
	$(GO) test -tags=integration $(PKGS_INT) -count=1

coverage: ## Show coverage summary
	$(GO) tool cover -func=$(COVER_PROFILE)

coverage-html: ## Open HTML coverage report
	$(GO) tool cover -html=$(COVER_PROFILE)

coverage-matrix: ## Show per-package coverage and highlight those below MIN_COVERAGE
	@tmpdir=$$(mktemp -d); \
	failed=0; \
	for p in $(PKGS); do \
	  prof=$$tmpdir/$$(echo $$p | tr '/' '_').out; \
	  $(GO) test $$p -coverprofile=$$prof -covermode=$(COVER_MODE) >/dev/null; \
	  pct=$$( $(GO) tool cover -func=$$prof | awk '/total:/ {print $$3}' | sed 's/%//' ); \
	  printf "%-60s %6.2f%%\n" "$$p" "$$pct"; \
	  awk -v x=$$pct -v min=$(MIN_COVERAGE) 'BEGIN{ if (x+0 < min+0) exit 1; }' || failed=1; \
	done | sort -k2 -n; \
	[ $$failed -eq 0 ] || (echo "\nSome packages below MIN_COVERAGE=$(MIN_COVERAGE)%" && exit 1)

coverage-it: ## Generate coverage for integration-tagged tests per package
	@tmpdir=$$(mktemp -d); \
	for p in $(PKGS_INT); do \
	  prof=$$tmpdir/$$(echo $$p | tr '/' '_').it.out; \
	  $(GO) test -tags=integration $$p -coverprofile=$$prof -covermode=$(COVER_MODE) -count=1 >/dev/null || true; \
	  if [ -f $$prof ]; then \
	    pct=$$( $(GO) tool cover -func=$$prof | awk '/total:/ {print $$3}' | sed 's/%//' ); \
	    printf "[IT] %-56s %6.2f%%\n" "$$p" "$$pct"; \
	  fi; \
	done | sort -k2 -n

# ───────────────────────────────────────────────────────────────────────────────
# Migrations & Docker
# ───────────────────────────────────────────────────────────────────────────────
migrate: ## Run DB migrations locally (requires local Postgres up)
	docker run --rm \
		--network host \
		-v $$PWD/migrations:/migrations \
		ghcr.io/pressly/goose \
		-dir /migrations \
		postgres "postgres://$(DB_USER):$(DB_PASS)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL)" up

docker-build: swagger ## Compose build images (Swagger auto-generated)
	docker compose build

docker-up: swagger ## Compose up (Swagger auto-generated + build, API only)
	docker compose up --build -d api

docker-api-up: swagger ## Start Postgres, apply migrations (goose), then API
	docker compose up -d postgres
	docker compose up -d goose
	docker compose up -d api

docker-ingest: swagger ## Run ingestion one-off with 7 files in parallel
	docker compose run --rm --profile ingest ingest

docker-down: ## Compose down
	docker compose down

docker-restart: docker-down docker-api-up ## Restart Compose stack (DB, migrations, API)

# ───────────────────────────────────────────────────────────────────────────────
# Housekeeping
# ───────────────────────────────────────────────────────────────────────────────
clean: ## Clean compiled files and coverage artifacts
	rm -f $(APP_NAME) $(COVER_PROFILE)

vet: ## Run go vet static analysis
	$(GO) vet ./...

staticcheck: ## Run staticcheck static analysis
	@which staticcheck > /dev/null || (echo "Installing staticcheck..." && $(GO) install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...

analyze: vet staticcheck lint ## Run all static analysis tools

.PHONY: help setup install run-api ingest build fmt tidy lint swagger \
        test test-unit test-integration coverage coverage-html coverage-matrix coverage-it \
        migrate docker-build docker-up docker-api-up docker-ingest docker-down docker-restart \
        clean vet staticcheck analyze