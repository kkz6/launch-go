#!/bin/bash
{{ shellDefaults }}

# Install Traefik as a plain docker container.
#
# Pre-v2 launchctl ran Traefik as a swarm service (`docker service
# create`) with `providers.swarm` for discovery. We dropped swarm in
# favour of `providers.docker` because:
#   * Compose containers + `docker run` containers can both register
#     via labels — swarm provider only sees swarm services, so
#     compose was a second-class citizen for routing.
#   * Single-host SaaS doesn't need multi-host primitives.
#   * One less mental model ("service" vs "container") to explain.
#
# The {{ .ContainerName }} container is `--restart=always`, so it
# survives reboots without us needing a systemd unit.

echo "Writing Traefik config and starting the {{ .ContainerName }} container"

sudo mkdir -p "{{ .RootDir }}/traefik/dynamic"

# Static config — entrypoints, docker + file providers, ACME resolver.
# `providers.docker.network` pins the network Traefik uses when a
# container is attached to multiple networks. Customer workloads join
# {{ .NetworkName }} so Traefik reaches them by container name.
sudo tee "{{ .RootDir }}/traefik/traefik.yml" >/dev/null <<EOF
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: {{ .NetworkName }}
  file:
    directory: /etc/traefik/dynamic
    watch: true
api:
  dashboard: true
  insecure: false
log:
  level: INFO
EOF

# Let's Encrypt resolver. Traefik's ACME config requires an `email`, but the
# value is only the account's expiry-notice address — it is NOT user-facing.
# So we ALWAYS configure the resolver (defaulting to a valid placeholder when
# none is provided), the same way Dokploy hard-codes one. Omitting the resolver
# is what leaves HTTPS domains stuck on Traefik's self-signed default cert.
ACME_EMAIL="{{ .ACMEEmail }}"
if [ -z "${ACME_EMAIL}" ]; then
    ACME_EMAIL="ssl@launchctl.io"
fi
sudo tee -a "{{ .RootDir }}/traefik/traefik.yml" >/dev/null <<EOF
certificatesResolvers:
  letsencrypt:
    acme:
      email: ${ACME_EMAIL}
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
EOF

sudo touch "{{ .RootDir }}/traefik/acme.json"
sudo chmod 600 "{{ .RootDir }}/traefik/acme.json"

# Dynamic middlewares — http→https redirect available to all routers.
sudo tee "{{ .RootDir }}/traefik/dynamic/middlewares.yml" >/dev/null <<EOF
http:
  middlewares:
    redirect-to-https:
      redirectScheme:
        scheme: https
        permanent: true
EOF

# If a legacy swarm Traefik is running, leave it alone — old servers
# stay on the pre-v2 stack (see ProvisionStepSetupSwarmNetwork). The
# label-discovery upgrade only applies to fresh provisions.
if sudo docker service ls --format '{{`{{`}}.Name{{`}}`}}' 2>/dev/null | grep -qx '{{ .ContainerName }}'; then
    echo "Legacy swarm Traefik service detected; leaving it untouched"
    exit 0
fi

# Container path. Idempotent: re-running the step is safe.
if sudo docker ps -a --format '{{`{{`}}.Names{{`}}`}}' | grep -qx '{{ .ContainerName }}'; then
    echo "Traefik container {{ .ContainerName }} already exists; restarting to apply static config"
    # `restart` (not `start`) so a re-provision picks up changes to
    # traefik.yml (e.g. a newly-added certificatesResolvers block) — the
    # static config is only read at Traefik startup.
    sudo docker restart {{ .ContainerName }} >/dev/null || true
else
    sudo docker run -d \
        --name {{ .ContainerName }} \
        --restart=always \
        --network {{ .NetworkName }} \
        -p 80:80 \
        -p 443:443 \
        -v /var/run/docker.sock:/var/run/docker.sock:ro \
        -v {{ .RootDir }}/traefik:/etc/traefik \
        traefik:{{ .TraefikVersion }} >/dev/null

    echo "Waiting for Traefik container to report healthy..."
    for i in $(seq 1 30); do
        state=$(sudo docker inspect -f '{{`{{`}}.State.Status{{`}}`}}' {{ .ContainerName }} 2>/dev/null || true)
        if [ "${state}" = "running" ]; then
            echo "Traefik container is running"
            break
        fi
        sleep 1
        if [ "${i}" -eq 30 ]; then
            echo "Traefik container did not reach running state within 30s" >&2
            sudo docker logs --tail 100 {{ .ContainerName }} >&2 || true
            exit 1
        fi
    done
fi
