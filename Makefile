GO ?= go
DB ?= ./simsexam_v1.db
ADDR ?= 127.0.0.1:6080
IMPORT_FILE ?= ./docs/examples/se-demo.md
SEED_FILES ?=
BIN_DIR ?= ./bin
SERVER_BIN ?= $(BIN_DIR)/simsexam

.PHONY: help fmt test build run migrate bootstrap import validate clean

help:
	@printf "simsexam targets:\n"
	@printf "  make fmt           Format Go files\n"
	@printf "  make test          Run all tests\n"
	@printf "  make build         Build all packages\n"
	@printf "  make run           Run the web server\n"
	@printf "  make migrate       Run v1 database migrations\n"
	@printf "  make bootstrap     Prepare v1 database and seed data\n"
	@printf "  make validate      Validate IMPORT_FILE without importing\n"
	@printf "  make import        Validate and import IMPORT_FILE into DB\n"
	@printf "  make clean         Remove local build artifacts\n"
	@printf "\n"
	@printf "Variables:\n"
	@printf "  DB=%s\n" "$(DB)"
	@printf "  ADDR=%s\n" "$(ADDR)"
	@printf "  IMPORT_FILE=%s\n" "$(IMPORT_FILE)"

fmt:
	find . -name '*.go' -not -path './.tmp/*' -print0 | xargs -0 gofmt -w

test:
	$(GO) test ./...

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(SERVER_BIN) ./cmd/server
	$(GO) build ./...

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

clean:
	rm -rf $(BIN_DIR)
