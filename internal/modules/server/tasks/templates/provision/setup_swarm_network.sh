#!/bin/bash
{{ shellDefaults }}

echo "Initializing Docker Swarm and creating the {{ .NetworkName }} overlay"

if sudo docker info 2>/dev/null | grep -q 'Swarm: active'; then
    echo "Docker Swarm is already active"
else
    ADVERTISE_ADDR="{{ .PublicIPv4 }}"
    if [ -z "$ADVERTISE_ADDR" ]; then
        ADVERTISE_ADDR="$(curl -4s --connect-timeout 5 https://ifconfig.io 2>/dev/null || true)"
    fi
    if [ -z "$ADVERTISE_ADDR" ]; then
        ADVERTISE_ADDR="$(curl -4s --connect-timeout 5 https://icanhazip.com 2>/dev/null || true)"
    fi
    if [ -z "$ADVERTISE_ADDR" ]; then
        echo "ERROR: could not determine an IPv4 address for swarm --advertise-addr" >&2
        exit 1
    fi

    echo "Advertising swarm on ${ADVERTISE_ADDR}"
    sudo docker swarm init --advertise-addr "${ADVERTISE_ADDR}"
fi

# Use plain {{`{{`}}.Name{{`}}`}} so docker's Go-template format string is not consumed by our renderer.
if sudo docker network ls --format '{{`{{`}}.Name{{`}}`}}' | grep -qx '{{ .NetworkName }}'; then
    echo "Overlay network {{ .NetworkName }} already exists"
else
    sudo docker network create --driver overlay --attachable {{ .NetworkName }}
fi
