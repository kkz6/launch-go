#!/bin/bash
{{ shellDefaults }}

echo "Writing Traefik config and deploying the {{ .ServiceName }} Swarm service"

sudo mkdir -p "{{ .RootDir }}/traefik/dynamic"

# Static config — entrypoints, swarm + file providers, ACME resolver.
sudo tee "{{ .RootDir }}/traefik/traefik.yml" >/dev/null <<EOF
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
providers:
  swarm:
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

if [ -n "{{ .ACMEEmail }}" ]; then
    sudo tee -a "{{ .RootDir }}/traefik/traefik.yml" >/dev/null <<EOF
certificatesResolvers:
  letsencrypt:
    acme:
      email: {{ .ACMEEmail }}
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
EOF
fi

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

# Deploy or no-op if already present.
#
# `--detach` makes `docker service create` return immediately instead of
# streaming the "overall progress: 0 out of 1 tasks" / "verify: Waiting N
# seconds…" loop until the swarm converges (40+ identical lines in our
# logs). We poll for convergence ourselves with cleaner output below.
if sudo docker service ls --format '{{`{{`}}.Name{{`}}`}}' | grep -qx '{{ .ServiceName }}'; then
    echo "Traefik service {{ .ServiceName }} already exists"
else
    sudo docker service create --detach \
        --name {{ .ServiceName }} \
        --constraint=node.role==manager \
        --publish published=80,target=80,mode=host \
        --publish published=443,target=443,mode=host \
        --network {{ .NetworkName }} \
        --mount type=bind,src={{ .RootDir }}/traefik,dst=/etc/traefik \
        --restart-condition any \
        traefik:{{ .TraefikVersion }} >/dev/null

    echo "Waiting for Traefik service to start..."
    for i in $(seq 1 60); do
        running=$(sudo docker service ps {{ .ServiceName }} \
            --filter desired-state=running \
            --format '{{`{{`}}.CurrentState{{`}}`}}' 2>/dev/null \
            | grep -c '^Running' || true)
        if [ "${running}" -ge 1 ]; then
            echo "Traefik service is running"
            break
        fi
        sleep 2
        if [ "${i}" -eq 60 ]; then
            echo "Traefik service did not reach running state within 120s" >&2
            sudo docker service ps {{ .ServiceName }} --no-trunc >&2 || true
            exit 1
        fi
    done
fi
