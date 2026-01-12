#!/bin/sh
set -e

MODE="${1:-api}"

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
        exit 1
        ;;
esac
