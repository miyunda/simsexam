GO ?= go
DB ?= ./simsexam_v1.db
ADDR ?= 127.0.0.1:6080
IMPORT_FILE ?= ./docs/examples/se-demo.md
SEED_FILES ?=
BIN_DIR ?= ./bin
SERVER_BIN ?= $(BIN_DIR)/simsexam
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'simsexam/internal/buildinfo.Version=$(VERSION)' -X 'simsexam/internal/buildinfo.Commit=$(COMMIT)' -X 'simsexam/internal/buildinfo.BuildTime=$(BUILD_TIME)'

.PHONY: help fmt fmt-check test build run migrate bootstrap import validate version clean

help:
	@printf "simsexam targets:\n"
	@printf "  make fmt           Format Go files\n"
	@printf "  make fmt-check     Verify Go files are formatted\n"
	@printf "  make test          Run all tests\n"
	@printf "  make build         Build all packages\n"
	@printf "  make run           Run the web server\n"
	@printf "  make migrate       Run v1 database migrations\n"
	@printf "  make bootstrap     Prepare v1 database and seed data\n"
	@printf "  make validate      Validate IMPORT_FILE without importing\n"
	@printf "  make import        Validate and import IMPORT_FILE into DB\n"
	@printf "  make version       Print build metadata values\n"
	@printf "  make clean         Remove local build artifacts\n"
	@printf "\n"
	@printf "Variables:\n"
	@printf "  DB=%s\n" "$(DB)"
	@printf "  ADDR=%s\n" "$(ADDR)"
	@printf "  IMPORT_FILE=%s\n" "$(IMPORT_FILE)"
	@printf "  VERSION=%s\n" "$(VERSION)"

fmt:
	find . -name '*.go' -not -path './.tmp/*' -print0 | xargs -0 gofmt -w

fmt-check:
	@test -z "$$(find . -name '*.go' -not -path './.tmp/*' -print0 | xargs -0 gofmt -l)"

test:
	$(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(SERVER_BIN) ./cmd/server
	$(GO) build -ldflags "$(LDFLAGS)" ./...

run:
	SIMSEXAM_DB_PATH=$(DB) SIMSEXAM_ADDR=$(ADDR) $(GO) run ./cmd/server

migrate:
	$(GO) run ./cmd/migrate -dsn $(DB)

bootstrap:
	$(GO) run ./cmd/bootstrapv1 -dsn $(DB) $(if $(SEED_FILES),-seed-files $(SEED_FILES),)

validate:
	$(GO) run ./cmd/importer -file $(IMPORT_FILE) -dsn $(DB)

import:
	$(GO) run ./cmd/importer -file $(IMPORT_FILE) -dsn $(DB) -apply

version:
	@printf "VERSION=%s\n" "$(VERSION)"
	@printf "COMMIT=%s\n" "$(COMMIT)"
	@printf "BUILD_TIME=%s\n" "$(BUILD_TIME)"

clean:
	rm -rf $(BIN_DIR)
