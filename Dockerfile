# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /worker ./cmd/worker

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /api .
COPY --from=builder /worker .

# Copy migrations and scripts
COPY migrations ./migrations
COPY scripts ./scripts

# Copy and set up entrypoint
COPY scripts/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh

# Create non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

# Default mode is 'api', can be overridden with 'worker'
# Usage:
#   docker run <image>                    # runs api (default)
#   docker run <image> api                # runs api explicitly
#   docker run <image> worker             # runs worker
ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["api"]
