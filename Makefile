.PHONY: gen-model clean-gen db/migrate-up db/migrate-down db/seed db/reset db/new-migration run/api run/consumer build build/api build/mq docker/build docker/build-api docker/build-mq run/test

IDL_DIR := idl
THRIFT_FILES := $(shell find $(IDL_DIR) -type f -name "*.thrift")

# Load .env if present (local secrets, never commit)
# Export so they're visible to go run / sub-makes.
ifneq (,$(wildcard .env))
include .env
export
endif

# Database targets read host/port/name from config/config.<APP_ENV>.yaml
# and credentials (username/password) from Infisical via the confloader.
# Override the env selection:  make <target> APP_ENV=production
APP_ENV ?= development

# Service identity for logs, metrics, and traces.
APP_NAME ?= efficient-report-exporter

# Version is stamped into the binary at link time. The version is never derived
# from VCS metadata; set an explicit value at build/deploy time when a stable
# release version is needed:  make build VERSION=1.4.2
VERSION ?= dev
LDFLAGS := -X github.com/fikrimohammad/go-dev-sdk/appinfo.version=$(VERSION)

# Logging format & level for run targets.
# Override:  make run/api LOG_FORMAT=json LOG_LEVEL=info
LOG_FORMAT ?= text
LOG_LEVEL ?= debug

# Generate API models from Thrift IDL
gen-model:
	@if [ -z "$(THRIFT_FILES)" ]; then \
		echo "No .thrift files found."; \
	else \
		for file in $(THRIFT_FILES); do \
			echo "Generating model for $$file"; \
			hz model -idl $$file -model_dir model; \
		done; \
	fi

# Clean generated API models
clean-gen:
	rm -rf model/api

# Database migrations (reads config + Infisical secrets like the app does)
db/migrate-up:
	APP_ENV=$(APP_ENV) go run db/migrate/main.go up

db/migrate-down:
	APP_ENV=$(APP_ENV) go run db/migrate/main.go down-all

# Database seed (reads config + Infisical secrets like the app does)
# Seeded tables:  report, export_report_job
#   make db/seed                          → seed all tables
#   make db/seed TABLES=report            → seed only report
#   make db/seed TABLES="report export_report_job"  → seed both
db/seed:
	APP_ENV=$(APP_ENV) go run db/seed/seed.go $(TABLES)

# Scaffold a new timestamped migration file pair
#   make db/new-migration NAME=add_user_table
db/new-migration:
	go run db/migrate/main.go new $(NAME)

# Full DB reset: drop tables, re-create, seed
db/reset: db/migrate-down db/migrate-up db/seed

# ── Run services ──────────────────────────────────────────────────────────────

run/api:
	APP_ENV=$(APP_ENV) APP_NAME=$(APP_NAME) LOG_FORMAT=$(LOG_FORMAT) LOG_LEVEL=$(LOG_LEVEL) go run -ldflags "$(LDFLAGS)" cmd/api/main.go

run/consumer:
	APP_ENV=$(APP_ENV) APP_NAME=$(APP_NAME) LOG_FORMAT=$(LOG_FORMAT) LOG_LEVEL=$(LOG_LEVEL) go run -ldflags "$(LDFLAGS)" cmd/mq/main.go

# build produces binaries in ./bin with the version stamped in.
build: build/api build/mq

build/api:
	APP_ENV=$(APP_ENV) APP_NAME=$(APP_NAME) go build -ldflags "$(LDFLAGS)" -o bin/api ./cmd/api

build/mq:
	APP_ENV=$(APP_ENV) APP_NAME=$(APP_NAME) go build -ldflags "$(LDFLAGS)" -o bin/mq ./cmd/mq

# Docker images (multi-stage; --target selects the service).
#   make docker/build VERSION=1.2.0
docker/build: docker/build-api docker/build-mq

docker/build-api:
	docker build --target api -t efficient-report-exporter-api:$(VERSION) .

docker/build-mq:
	docker build --target mq -t efficient-report-exporter-mq:$(VERSION) .

# run/test: mockey (used by common/observability) rewrites function
# instructions at runtime, so inlining and optimization must be disabled
# during compilation for its mocks to take effect.
run/test:
	go test -count=1 -gcflags="all=-N -l" ./...
