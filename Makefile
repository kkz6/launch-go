.PHONY: build run worker test lint migrate shellcheck help

# Build variables
BINARY_API=bin/api
BINARY_WORKER=bin/worker
BINARY_SHELLCHECK=bin/shellcheck
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")

# Default target
all: build

## build: Build the API and worker binaries
build:
	@echo "Building API..."
	@go build -o $(BINARY_API) ./cmd/api
	@echo "Building Worker..."
	@go build -o $(BINARY_WORKER) ./cmd/worker
	@echo "Build complete!"

## run: Run the API server
run:
	@go run ./cmd/api

## worker: Run the queue worker
worker:
	@go run ./cmd/worker

## dev: Run API with hot reload (requires air)
dev:
	@air -c .air.toml

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

## migrate-up: Run database migrations
migrate-up:
	@migrate -path migrations -database "$(DATABASE_URL)" up

## migrate-down: Rollback last migration
migrate-down:
	@migrate -path migrations -database "$(DATABASE_URL)" down 1

## migrate-create: Create a new migration (usage: make migrate-create name=create_users_table)
migrate-create:
	@migrate create -ext sql -dir migrations -seq $(name)

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
