.PHONY: build run worker test lint migrate migrate-rollback migrate-fresh migrate-status shellcheck help

# Load environment variables from .env file
ifneq (,$(wildcard ./.env))
include .env
export
endif

# Build variables
BINARY_API=bin/api
BINARY_WORKER=bin/worker
BINARY_MIGRATE=bin/migrate
BINARY_SHELLCHECK=bin/shellcheck
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")

# Default target
all: build

## build: Build the API, worker, and migrate binaries
build:
	@echo "Building API..."
	@go build -o $(BINARY_API) ./cmd/api
	@echo "Building Worker..."
	@go build -o $(BINARY_WORKER) ./cmd/worker
	@echo "Building Migrate..."
	@go build -o $(BINARY_MIGRATE) ./cmd/migrate
	@echo "Build complete!"

## run: Run the API server
run:
	@go run ./cmd/api

## worker: Run the queue worker
worker:
	@go run ./cmd/worker

## dev: Run API with hot reload (requires air)
dev:
	@$(shell go env GOPATH)/bin/air -c .air.toml

## test: Run tests
test:
	@go test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

## lint: Run linter
lint:
	@golangci-lint run

## fmt: Format code
fmt:
	@gofmt -s -w $(GO_FILES)

## tidy: Tidy go modules
tidy:
	@go mod tidy

## deps: Download dependencies
deps:
	@go mod download

## migrate: Run all pending migrations
migrate:
	@go run ./cmd/migrate migrate

## migrate-rollback: Rollback the last batch of migrations
migrate-rollback:
	@go run ./cmd/migrate rollback

## migrate-fresh: Drop all tables and re-run all migrations
migrate-fresh:
	@go run ./cmd/migrate fresh

## migrate-status: Show the status of all migrations
migrate-status:
	@go run ./cmd/migrate status

## docker-build: Build docker image
docker-build:
	@docker build -t launch-go .

## docker-up: Start docker containers
docker-up:
	@docker-compose up -d

## docker-down: Stop docker containers
docker-down:
	@docker-compose down

## docker-logs: View docker logs
docker-logs:
	@docker-compose logs -f

## shellcheck-render: Render shell scripts for ShellCheck validation
shellcheck-render:
	@go run ./cmd/shellcheck -output storage/shellcheck
	@echo "Scripts rendered to storage/shellcheck/"

## shellcheck: Run ShellCheck on rendered scripts
shellcheck: shellcheck-render
	@echo "Running ShellCheck..."
	@shellcheck storage/shellcheck/*.sh || true

## clean: Clean build artifacts
clean:
	@rm -rf bin/
	@rm -rf storage/shellcheck/
	@rm -f coverage.out coverage.html

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
