#!/bin/bash
{{ shellDefaults }}

# Create the bridge network Traefik and every customer workload joins.
#
# Pre-v2 docker servers used an *overlay* network that required Docker
# Swarm to back it. We've since dropped swarm (single-node SaaS — the
# multi-host primitives weren't paying for themselves and the swarm
# provider blocks Traefik from discovering compose / `docker run`
# containers by label). Bridge is the standalone-docker equivalent and
# gives us the same in-network DNS that lets Traefik reach containers
# by name.

echo "Creating the {{ .NetworkName }} bridge network"

# Use plain {{`{{`}}.Name{{`}}`}} so docker's Go-template format string is not consumed by our renderer.
if sudo docker network ls --format '{{`{{`}}.Name{{`}}`}}' | grep -qx '{{ .NetworkName }}'; then
    # Could be a legacy overlay (pre-v2 swarm install). Leave it alone —
    # the existing Traefik + workloads are already attached to it.
    # The install_traefik step below detects which driver is in use
    # before deciding how to attach.
    echo "Network {{ .NetworkName }} already exists; leaving driver as-is"
else
    sudo docker network create --driver bridge {{ .NetworkName }}
fi
