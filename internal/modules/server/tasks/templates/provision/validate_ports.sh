#!/bin/bash
{{ shellDefaults }}

echo "Checking that ports 80 and 443 are free for Traefik"

if command -v ss >/dev/null 2>&1; then
    if ss -tulnp 2>/dev/null | grep -qE ':(80)\s'; then
        echo "WARN: something is already listening on port 80" >&2
    fi
    if ss -tulnp 2>/dev/null | grep -qE ':(443)\s'; then
        echo "WARN: something is already listening on port 443" >&2
    fi
else
    echo "ss not available, skipping port-availability check"
fi
