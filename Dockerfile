# Build stage
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=development

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries for target platform
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s -X main.Version=${VERSION}" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s" -o /worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-w -s" -o /migrate ./cmd/migrate

# Final stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /api .
COPY --from=builder /worker .
COPY --from=builder /migrate .

# Copy scripts
COPY scripts ./scripts

# Copy and set up entrypoint
COPY scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

# Usage:
#   docker run <image>                              # runs api (default)
#   docker run <image> api                          # runs api explicitly
#   docker run <image> worker                       # runs worker
#   docker run -e RUN_MIGRATIONS=true <image>       # runs migrations then api
#   docker run <image> ./migrate status             # run migrate CLI directly
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["api"]
