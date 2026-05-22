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
if sudo docker service ls --format '{{`{{`}}.Name{{`}}`}}' | grep -qx '{{ .ServiceName }}'; then
    echo "Traefik service {{ .ServiceName }} already exists"
else
    sudo docker service create \
        --name {{ .ServiceName }} \
        --constraint=node.role==manager \
        --publish published=80,target=80,mode=host \
        --publish published=443,target=443,mode=host \
        --network {{ .NetworkName }} \
        --mount type=bind,src={{ .RootDir }}/traefik,dst=/etc/traefik \
        --restart-condition any \
        traefik:{{ .TraefikVersion }}
fi
