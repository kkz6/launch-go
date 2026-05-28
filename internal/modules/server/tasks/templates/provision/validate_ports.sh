#!/bin/bash
{{ shellDefaults }}

# Free ports 80 and 443 for Traefik. Earlier this script only WARNed
# when something was listening, then install_traefik would fail two
# steps later with the cryptic docker error:
#   failed to bind host port 0.0.0.0:80/tcp: address already in use
# Customers had to ssh in and figure out the culprit themselves.
# Now we identify the offender, stop+disable known web servers, and
# fail loudly with diagnostics if the port is held by something we
# don't recognise — so the customer sees the real problem in the
# provision log instead of a cryptic docker container-start failure.

# whoBindsPort prints the process name that owns the given port, or
# empty if nothing's bound. Looks at ss's `users:(("<name>",pid=...))`
# field, which is the most reliable identifier across distros.
whoBindsPort() {
    local port="$1"
    ss -tulnp 2>/dev/null \
        | grep -E ":${port}\s" \
        | head -1 \
        | grep -oE 'users:\(\("[^"]+"' \
        | head -1 \
        | sed 's/users:(("//' \
        | sed 's/"//'
}

# stopDockerContainerBindingPort looks for a docker container that's
# mapping the host port, and stops it. Covers the "previous failed
# provision left a half-running container" case — e.g. a stale
# launch-traefik that crashed mid-start but kept the port reservation.
stopDockerContainerBindingPort() {
    local port="$1"
    if ! command -v docker >/dev/null 2>&1; then
        return 0
    fi
    # docker ps --format '{{.ID}} {{.Ports}}' returns lines like
    # "abc123 0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp". Match any
    # mapping of <anything>:<port>-> .
    local container
    container=$(sudo docker ps --format '{{.ID}} {{.Ports}}' \
                | grep -E ":${port}->" \
                | awk '{print $1}' \
                | head -1)
    if [ -n "${container}" ]; then
        echo "Stopping docker container ${container} that held port ${port}..."
        sudo docker stop "${container}" >/dev/null 2>&1 || true
    fi
}

# freePort runs the full identify → remediate → reverify cycle for a
# single port. Returns non-zero (which under set -e bubbles up and
# fails the provision step) if the port is still bound by something
# we couldn't handle automatically.
freePort() {
    local port="$1"
    if ! command -v ss >/dev/null 2>&1; then
        echo "ss not available; cannot validate port ${port}, continuing optimistically"
        return 0
    fi

    if ! ss -tulnp 2>/dev/null | grep -qE ":${port}\s"; then
        echo "Port ${port} is free"
        return 0
    fi

    local proc
    proc=$(whoBindsPort "${port}")
    echo "Port ${port} is in use by: ${proc:-<unidentified>}"

    case "${proc}" in
        apache2|httpd)
            echo "  Stopping and disabling apache2..."
            sudo systemctl stop apache2 2>/dev/null || true
            sudo systemctl disable apache2 2>/dev/null || true
            ;;
        nginx)
            echo "  Stopping and disabling nginx..."
            sudo systemctl stop nginx 2>/dev/null || true
            sudo systemctl disable nginx 2>/dev/null || true
            ;;
        lighttpd)
            echo "  Stopping and disabling lighttpd..."
            sudo systemctl stop lighttpd 2>/dev/null || true
            sudo systemctl disable lighttpd 2>/dev/null || true
            ;;
        caddy)
            echo "  Stopping and disabling caddy..."
            sudo systemctl stop caddy 2>/dev/null || true
            sudo systemctl disable caddy 2>/dev/null || true
            ;;
        haproxy)
            echo "  Stopping and disabling haproxy..."
            sudo systemctl stop haproxy 2>/dev/null || true
            sudo systemctl disable haproxy 2>/dev/null || true
            ;;
        docker-proxy)
            # Some other docker container is holding the port — likely
            # a stale launch-traefik from a previous failed attempt, or
            # a customer-deployed container that ended up on 80/443.
            stopDockerContainerBindingPort "${port}"
            ;;
        *)
            echo "ERROR: Port ${port} is held by an unrecognized process (${proc:-unknown})." >&2
            echo "Manually stop it on the server, then retry provisioning. The" >&2
            echo "Launch Docker stack needs exclusive ownership of ports 80 and 443" >&2
            echo "so Traefik can terminate TLS for every deployed application." >&2
            echo
            echo "Full ss output for diagnosis:" >&2
            sudo ss -tulnp 2>/dev/null | grep -E ":${port}\s" >&2 || true
            return 1
            ;;
    esac

    # systemctl returns before the socket is released; give the kernel
    # a beat to clear it before re-checking.
    sleep 2

    if ss -tulnp 2>/dev/null | grep -qE ":${port}\s"; then
        local still
        still=$(whoBindsPort "${port}")
        echo "ERROR: Port ${port} is still bound by ${still:-something} after the stop attempt." >&2
        echo "Investigate manually and retry provisioning." >&2
        return 1
    fi

    echo "  Port ${port} now free"
    return 0
}

echo "Checking that ports 80 and 443 are free for Traefik"
freePort 80
freePort 443
echo "Port validation complete"
