{{/* Go Template: rollback-deployment.sh.tmpl */}}
{{/* Migrated from Laravel: modules/site/resources/views/tasks/rollback-deployment.blade.php */}}
#!/bin/bash
set -euo pipefail

echo "Rolling back to previous release..."

# Verify the target release directory exists
if [ ! -d "{{ .ReleaseDirectory }}" ]; then
    echo "Error: Target release directory does not exist: {{ .ReleaseDirectory }}"
    exit 1
fi

# Switch the current symlink to the target release
cd {{ .Site.Path }}
ln -nfs --relative {{ .ReleaseDirectory }} {{ .CurrentDirectory }}

echo "Rollback completed successfully!"
echo "Current release is now: {{ .ReleaseDirectory }}"
