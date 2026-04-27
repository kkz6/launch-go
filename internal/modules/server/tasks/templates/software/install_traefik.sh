#!/bin/bash
{{ shellDefaults }}

echo "Configure Traefik"

# Static configuration. The Docker provider auto-discovers containers
# attached to launch-network that carry traefik.* labels. The file
# provider lets the rest of the system drop dynamic config snippets in
# /etc/launch/traefik/dynamic without restarting Traefik.
sudo tee /etc/launch/traefik/traefik.yml > /dev/null <<'YAML'
entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"

providers:
  docker:
    exposedByDefault: false
    network: launch-network
  file:
    directory: /etc/launch/traefik/dynamic
    watch: true

api:
  dashboard: false

log:
  level: INFO
YAML
{{ if .TraefikAdminEmail }}
# ACME / Let's Encrypt resolver. Appended in a second tee call so the
# heredoc above can stay quoted (no template-tag interference).
sudo tee -a /etc/launch/traefik/traefik.yml > /dev/null <<YAML

certificatesResolvers:
  letsencrypt:
    acme:
      email: {{ .TraefikAdminEmail }}
      storage: /etc/launch/traefik/acme.json
      httpChallenge:
        entryPoint: web
YAML
{{ end }}
sudo touch /etc/launch/traefik/acme.json
sudo chmod 600 /etc/launch/traefik/acme.json

# Run Traefik. Idempotent: remove any previous container before
# launching so re-running this script always lands on the desired image
# / config.
sudo docker rm -f launch-traefik >/dev/null 2>&1 || true
sudo docker run -d \
    --name launch-traefik \
    --restart unless-stopped \
    --network launch-network \
    -p 80:80 \
    -p 443:443 \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /etc/launch/traefik/traefik.yml:/etc/traefik/traefik.yml:ro \
    -v /etc/launch/traefik/dynamic:/etc/launch/traefik/dynamic:ro \
    -v /etc/launch/traefik/acme.json:/etc/launch/traefik/acme.json \
    traefik:v{{ .Version }}

echo "Traefik installation complete."
