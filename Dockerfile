# syntax=docker/dockerfile:1.7
# Build stage
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=development

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Module cache lives at /go/pkg/mod; the build cache at /root/.cache/go-build.
# BuildKit `--mount=type=cache` keeps these directories outside the layer
# hash, so they survive across builds even when COPY . . invalidates the
# layer above. Result: the Go compiler reuses .a files for unchanged
# packages, so a small source edit recompiles in seconds instead of minutes.

# Copy go mod files first — when go.mod/go.sum don't change, the download
# step is a clean cache hit at the Docker layer level too.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build binaries for target platform.
# Each build shares the same cache mounts; the second and third
# builds reuse compilations from the first.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-w -s -X main.Version=${VERSION}" -o /api ./cmd/api
# The console binary hosts the worker (queue:work), migrations (migrate:run),
# and every other ops command in one Artisan-style CLI.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-w -s -X main.Version=${VERSION}" -o /console ./cmd/console

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /api .
COPY --from=builder /console .

# Copy scripts
COPY scripts ./scripts

# Copy and set up entrypoint
COPY scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

ENV MODE=api

EXPOSE 8080

# Kamal v2 inspects container.State.Health to gate rolling deploys. Without
# a HEALTHCHECK directive the inspect returns null and Kamal fails the
# deploy even when the app is actually healthy. wget ships in the alpine
# base; --spider performs a HEAD request that the /health endpoint serves.
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=3 \
  CMD sh -c '[ "$MODE" = "worker" ] || wget -q --spider http://127.0.0.1:8080/health'

# Usage:
#   docker run <image>                              # runs api (default)
#   docker run -e MODE=worker <image>               # runs worker
#   docker run -e RUN_MIGRATIONS=true <image>       # runs migrations then api
ENTRYPOINT ["/app/entrypoint.sh"]
