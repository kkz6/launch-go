.PHONY: build run worker test lint lint-revive lint-static lint-fix lint-install migrate migrate-rollback migrate-fresh migrate-status shellcheck help client-install client-dev dev-all setup-hooks

# Build variables
BINARY_API=bin/api
BINARY_WORKER=bin/worker
BINARY_MIGRATE=bin/migrate
BINARY_SHELLCHECK=bin/shellcheck
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
GOBIN=$(shell go env GOPATH)/bin

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

## client-install: Install client dependencies
client-install:
	@cd client && npm install

## client-dev: Run Nuxt dev server
client-dev:
	@cd client && npm run dev

## dev-all: Run API, worker, and client dev servers concurrently
dev-all:
	@trap 'kill 0' EXIT; \
	$(shell go env GOPATH)/bin/air -c .air.toml & \
	go run ./cmd/worker & \
	cd client && npm run dev & \
	wait

## test: Run tests
test:
	@go test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

## lint: Run all linters (revive, staticcheck, go vet)
lint: lint-revive lint-static lint-vet
	@echo "All linters passed!"

## lint-revive: Run revive linter
lint-revive:
	@echo "Running revive..."
	@$(GOBIN)/revive -config revive.toml -formatter friendly ./...

## lint-static: Run staticcheck
lint-static:
	@echo "Running staticcheck..."
	@$(GOBIN)/staticcheck -f stylish ./...

## lint-vet: Run go vet
lint-vet:
	@echo "Running go vet..."
	@go vet ./...

## lint-fix: Auto-fix imports with goimports
lint-fix:
	@echo "Running goimports..."
	@$(GOBIN)/goimports -w $(GO_FILES)
	@echo "Imports fixed!"

## lint-install: Install all linting tools
lint-install:
	@echo "Installing linting tools..."
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/mgechev/revive@latest
	@go install honnef.co/go/tools/cmd/staticcheck@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "All linting tools installed at $(GOBIN)"

## lint-check: Check if code needs formatting (CI use)
lint-check:
	@echo "Checking code formatting..."
	@GOFMT_OUTPUT=$$(gofmt -l -s .); \
	if [ -n "$$GOFMT_OUTPUT" ]; then \
		echo "The following files need formatting:"; \
		echo "$$GOFMT_OUTPUT"; \
		echo ""; \
		echo "Run 'make fmt' to fix formatting issues"; \
		exit 1; \
	fi
	@echo "All files are properly formatted!"

## vuln: Run vulnerability check
vuln:
	@echo "Running govulncheck..."
	@$(GOBIN)/govulncheck ./...

## fmt: Format code
fmt:
	@gofmt -s -w $(GO_FILES)
	@echo "Code formatted!"

## setup-hooks: Setup git hooks and install linting tools
setup-hooks:
	@chmod +x scripts/setup-hooks.sh
	@./scripts/setup-hooks.sh

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

## migrate-encryption: Migrate Laravel-encrypted data to new format (one-time)
migrate-encryption:
	@echo "⚠️  This will decrypt Laravel-encrypted data and store as plaintext."
	@echo "    Make sure your APP_KEY is set correctly in .env"
	@echo ""
	@go run ./cmd/migrate-encryption

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
