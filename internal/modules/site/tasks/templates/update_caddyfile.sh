{{ shellDefaults }}

# Ensure the site logs directory exists with correct ownership (Caddy runs as site user)
LOGS_DIR="$(dirname {{ .CaddyfilePath }})/logs"
mkdir -p "$LOGS_DIR"
chown {{ .SiteUser }}:{{ .SiteUser }} "$LOGS_DIR"

# Create a temporary file with the new Caddyfile
cat > {{ .CaddyfilePath }}.tmp <<EOF
{{ .CaddyfileContent }}

EOF

# Validate the Caddyfile
set +e
caddy validate --config {{ .CaddyfilePath }}.tmp --adapter caddyfile

# If the Caddyfile is invalid, remove the temporary file and exit
if [ $? -ne 0 ]; then
    rm {{ .CaddyfilePath }}.tmp
    exit 1
fi

set -e

# Format the Caddyfile
caddy fmt {{ .CaddyfilePath }}.tmp --overwrite

# Replace the old Caddyfile with the new one
mv {{ .CaddyfilePath }}.tmp {{ .CaddyfilePath }}

# Reload Caddy
sudo /usr/sbin/service caddy reload
