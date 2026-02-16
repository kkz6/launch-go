{{ shellDefaults }}

echo "Updating upstream Caddyfile at {{ .CaddyfilePath }}"

# Ensure the upstreams directory exists
sudo mkdir -p /etc/caddy/upstreams

# Create a temporary file with the new Caddyfile
cat > "{{ .CaddyfilePath }}.tmp" <<'LAUNCH_CADDYFILE_EOF'
{{ .CaddyfileContent }}
LAUNCH_CADDYFILE_EOF

# Format the Caddyfile
caddy fmt "{{ .CaddyfilePath }}.tmp" --overwrite

# Replace the old Caddyfile with the new one
mv "{{ .CaddyfilePath }}.tmp" "{{ .CaddyfilePath }}"

# Reload Caddy (validates config before applying)
sudo /usr/sbin/service caddy reload

echo "Upstream Caddyfile updated successfully"
