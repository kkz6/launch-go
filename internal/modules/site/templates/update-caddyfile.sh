{{/* Go Template: update-caddyfile.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/update-caddyfile.blade.php */}}
#!/bin/bash
set -euo pipefail

# Create a temporary file with the new Caddyfile
cat > {{ .CaddyfilePath }}{{ .TmpSuffix }} <<EOF
{{ .Caddyfile }}

EOF

# Validate the Caddyfile
set +e
caddy validate --config {{ .CaddyfilePath }}{{ .TmpSuffix }} --adapter caddyfile

# If the Caddyfile is invalid, remove the temporary file and exit
if [ $? -ne 0 ]; then
    rm {{ .CaddyfilePath }}{{ .TmpSuffix }}
    exit 1
fi

set -e

# Format the Caddyfile
caddy fmt {{ .CaddyfilePath }}{{ .TmpSuffix }} --overwrite

# Replace the old Caddyfile with the new one
mv {{ .CaddyfilePath }}{{ .TmpSuffix }} {{ .CaddyfilePath }}

# Reload Caddy
sudo /usr/sbin/service caddy reload
