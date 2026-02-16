{{ shellDefaults }}

# Ensure the site logs directory exists
LOGS_DIR="$(dirname {{ .CaddyfilePath }})/logs"
mkdir -p "$LOGS_DIR"

# Create a temporary file with the new Caddyfile
cat > {{ .CaddyfilePath }}.tmp <<EOF
{{ .CaddyfileContent }}

EOF

# Format the Caddyfile
caddy fmt {{ .CaddyfilePath }}.tmp --overwrite

# Replace the old Caddyfile with the new one
mv {{ .CaddyfilePath }}.tmp {{ .CaddyfilePath }}

# Reload Caddy
sudo /usr/sbin/service caddy reload
