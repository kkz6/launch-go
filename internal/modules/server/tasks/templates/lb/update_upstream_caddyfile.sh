{{ shellDefaults }}

echo "Updating upstream Caddyfile at {{ .CaddyfilePath }}"

# Ensure the upstreams directory exists
sudo mkdir -p /etc/caddy/upstreams

# Create a temporary file with the new Caddyfile
cat > {{ .CaddyfilePath }}.tmp <<'CADDYEOF'
{{ .CaddyfileContent }}
CADDYEOF

# Validate the Caddyfile
set +e
caddy validate --config {{ .CaddyfilePath }}.tmp --adapter caddyfile
if [ $? -ne 0 ]; then
    rm -f {{ .CaddyfilePath }}.tmp
    echo "Caddyfile validation failed"
    exit 1
fi
set -e

# Format the Caddyfile
caddy fmt {{ .CaddyfilePath }}.tmp --overwrite

# Replace the old Caddyfile with the new one
mv {{ .CaddyfilePath }}.tmp {{ .CaddyfilePath }}

# Reload Caddy
sudo /usr/sbin/service caddy reload

echo "Upstream Caddyfile updated successfully"
