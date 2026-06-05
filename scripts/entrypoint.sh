#!/bin/sh
set -e

MODE="${MODE:-api}"

# Run migrations if RUN_MIGRATIONS is set to true
if [ "$RUN_MIGRATIONS" = "true" ]; then
    echo "Running database migrations..."
    ./console migrate:run
    echo "Migrations completed."
fi

case "$MODE" in
    api)
        echo "Starting API server..."
        exec ./api
        ;;
    worker)
        echo "Starting Worker..."
        exec ./console queue:work
        ;;
    *)
        echo "Unknown mode: $MODE"
        echo "Usage: docker run -e MODE=<mode> <image>"
        echo "  api    - Start the HTTP API server (default)"
        echo "  worker - Start the background job worker"
        echo ""
        echo "Environment variables:"
        echo "  MODE=api|worker     - Select which service to run (default: api)"
        echo "  RUN_MIGRATIONS=true - Run database migrations before starting"
        exit 1
        ;;
esac
