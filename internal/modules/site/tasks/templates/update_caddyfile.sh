{{ shellDefaults }}
{{ caddyReloadFunc }}

# Ensure the site logs directory exists with correct ownership (Caddy runs as site user)
LOGS_DIR="$(dirname {{ .CaddyfilePath }})/logs"
mkdir -p "$LOGS_DIR"
chown {{ .SiteUser }}:{{ .SiteUser }} "$LOGS_DIR"

# Create a temporary file with the new Caddyfile
cat > {{ .CaddyfilePath }}.tmp <<EOF
{{ .CaddyfileContent }}

EOF

# Format the Caddyfile
caddy fmt {{ .CaddyfilePath }}.tmp --overwrite

# Replace the old Caddyfile with the new one
mv {{ .CaddyfilePath }}.tmp {{ .CaddyfilePath }}

# Reload Caddy (validates config before applying)
reloadCaddy
