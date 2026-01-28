#!/bin/sh
set -e

MODE="${1:-api}"

# Run migrations if RUN_MIGRATIONS is set to true
if [ "$RUN_MIGRATIONS" = "true" ]; then
    echo "Running database migrations..."
    ./migrate migrate
    echo "Migrations completed."
fi

case "$MODE" in
    api)
        echo "Starting API server..."
        exec ./api
        ;;
    worker)
        echo "Starting Worker..."
        exec ./worker
        ;;
    *)
        echo "Unknown mode: $MODE"
        echo "Usage: docker run <image> [api|worker]"
        echo "  api    - Start the HTTP API server (default)"
        echo "  worker - Start the background job worker"
        echo ""
        echo "Environment variables:"
        echo "  RUN_MIGRATIONS=true - Run database migrations before starting"
        exit 1
        ;;
esac
