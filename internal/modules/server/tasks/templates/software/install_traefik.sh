#!/bin/bash
{{ shellDefaults }}

echo "Install Traefik reverse proxy"

# Create directories
sudo mkdir -p /etc/launch/traefik/dynamic
sudo mkdir -p /etc/launch/certs

# Write static Traefik config
sudo tee /etc/launch/traefik/traefik.yml > /dev/null <<'EOF'
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"

certificatesResolvers:
  letsencrypt:
    acme:
      email: "{{ .AdminEmail }}"
      storage: "/letsencrypt/acme.json"
      httpChallenge:
        entryPoint: web

providers:
  file:
    directory: "/etc/traefik/dynamic"
    watch: true

api:
  dashboard: false

log:
  level: "WARN"
EOF

# Ensure launch-network exists
docker network create launch-network 2>/dev/null || true

# Stop and remove old Traefik container if exists
docker stop launch-traefik 2>/dev/null || true
docker rm launch-traefik 2>/dev/null || true

# Create Traefik Let's Encrypt volume
docker volume create launch-traefik-letsencrypt 2>/dev/null || true

# Run Traefik container
docker run -d \
  --name launch-traefik \
  --network launch-network \
  --restart always \
  -p 80:80 \
  -p 443:443 \
  -v /etc/launch/traefik/traefik.yml:/etc/traefik/traefik.yml:ro \
  -v /etc/launch/traefik/dynamic:/etc/traefik/dynamic:ro \
  -v launch-traefik-letsencrypt:/letsencrypt \
  -v /etc/launch/certs:/etc/launch/certs:ro \
  traefik:v3

echo "Traefik reverse proxy installed successfully"
